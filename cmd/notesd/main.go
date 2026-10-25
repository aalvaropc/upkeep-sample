package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aalvaropc/upkeep-sample/api"
	"github.com/aalvaropc/upkeep-sample/auth"
	"github.com/aalvaropc/upkeep-sample/clock"
	"github.com/aalvaropc/upkeep-sample/middleware"
	"github.com/aalvaropc/upkeep-sample/notes"
	"github.com/aalvaropc/upkeep-sample/store"
)

const usage = `usage:
  notesd              serve the notes API
  notesd token -sub NAME [-scope "notes:write"]
                      print a signed token for NAME
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Getenv)
	stop()
	os.Exit(code)
}

type config struct {
	addr    string
	key     []byte
	issuer  string
	ttl     time.Duration
	origins []string
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{addr: getenv("NOTES_ADDR"), issuer: getenv("NOTES_ISSUER"), key: []byte(getenv("NOTES_JWT_SECRET")), ttl: time.Hour}
	if cfg.addr == "" {
		cfg.addr = "127.0.0.1:8081"
	}
	if cfg.issuer == "" {
		cfg.issuer = "notesd"
	}
	if len(cfg.key) < auth.MinKeyLength {
		return config{}, fmt.Errorf("NOTES_JWT_SECRET must be set to at least %d bytes", auth.MinKeyLength)
	}
	for _, origin := range strings.Split(getenv("NOTES_ALLOWED_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			cfg.origins = append(cfg.origins, origin)
		}
	}
	if raw := getenv("NOTES_TOKEN_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return config{}, fmt.Errorf("NOTES_TOKEN_TTL %q must be a positive duration", raw)
		}
		cfg.ttl = ttl
	}
	return cfg, nil
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	cfg, err := loadConfig(getenv)
	if err != nil {
		fmt.Fprintf(stderr, "notesd: %v\n", err)
		return 2
	}
	issuer, err := auth.NewIssuer(cfg.key, cfg.issuer, cfg.ttl, clock.System{})
	if err != nil {
		fmt.Fprintf(stderr, "notesd: %v\n", err)
		return 2
	}
	switch {
	case len(args) == 0:
		svc := notes.NewService(store.NewMemory(), clock.System{})
		handler := api.WithCORS(api.NewServer(svc, middleware.NewAuthenticator(issuer)), cfg.origins)
		if err := serve(ctx, cfg.addr, handler, stdout); err != nil {
			fmt.Fprintf(stderr, "notesd: %v\n", err)
			return 1
		}
		return 0
	case args[0] == "token":
		return token(issuer, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "notesd: unexpected argument %q\n\n%s", args[0], usage)
		return 2
	}
}

func token(issuer *auth.Issuer, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("token", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sub := fs.String("sub", "", "subject the token is issued to")
	scope := fs.String("scope", "", "space-separated scopes, for example notes:write")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *sub == "" {
		fmt.Fprint(stderr, "notesd token: -sub is required\n")
		return 2
	}
	signed, err := issuer.Issue(*sub, strings.Fields(*scope)...)
	if err != nil {
		fmt.Fprintf(stderr, "notesd token: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, signed)
	return 0
}

func serve(ctx context.Context, addr string, handler http.Handler, stdout io.Writer) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	fmt.Fprintf(stdout, "notesd listening on %s\n", ln.Addr())
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
