package main

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

var validEnv = map[string]string{"NOTES_JWT_SECRET": strings.Repeat("k", 32)}

func TestConfig(t *testing.T) {
	cfg, err := loadConfig(env(validEnv))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.addr != "127.0.0.1:8081" || cfg.issuer != "notesd" || cfg.ttl != time.Hour {
		t.Fatalf("defaults = %+v", cfg)
	}
	for name, values := range map[string]map[string]string{
		"no key":    {},
		"short key": {"NOTES_JWT_SECRET": "short"},
		"bad ttl":   {"NOTES_JWT_SECRET": strings.Repeat("k", 32), "NOTES_TOKEN_TTL": "soon"},
		"zero ttl":  {"NOTES_JWT_SECRET": strings.Repeat("k", 32), "NOTES_TOKEN_TTL": "0s"},
	} {
		if _, err := loadConfig(env(values)); err == nil {
			t.Errorf("%s: loadConfig returned no error", name)
		}
	}
}

func TestConfigAllowedOrigins(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{
		"NOTES_JWT_SECRET":      strings.Repeat("k", 32),
		"NOTES_ALLOWED_ORIGINS": " https://a.example, ,https://b.example ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.origins) != 2 || cfg.origins[0] != "https://a.example" || cfg.origins[1] != "https://b.example" {
		t.Fatalf("origins = %q", cfg.origins)
	}
	plain, err := loadConfig(env(validEnv))
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.origins) != 0 {
		t.Fatalf("origins without the setting = %q", plain.origins)
	}
}

func TestRunWithoutKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), nil, &stdout, &stderr, env(nil)); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "NOTES_JWT_SECRET") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), []string{"extra"}, &stdout, &stderr, env(validEnv)); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestTokenCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), []string{"token", "-sub", "alice", "-scope", "notes:write"}, &stdout, &stderr, env(validEnv)); code != 0 {
		t.Fatalf("exit code = %d; stderr %s", code, stderr.String())
	}
	if parts := strings.Split(strings.TrimSpace(stdout.String()), "."); len(parts) != 3 {
		t.Fatalf("token = %q, want three dot-separated parts", stdout.String())
	}
	stderr.Reset()
	if code := run(t.Context(), []string{"token"}, &stdout, &stderr, env(validEnv)); code != 2 || !strings.Contains(stderr.String(), "-sub is required") {
		t.Fatalf("token without -sub: exit %d, stderr %q", code, stderr.String())
	}
}

type signalWriter chan string

func (w signalWriter) Write(p []byte) (int, error) {
	w <- string(p)
	return len(p), nil
}

func TestServeStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stdout := make(signalWriter, 1)
	done := make(chan error, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	go func() { done <- serve(ctx, "127.0.0.1:0", handler, stdout) }()
	select {
	case line := <-stdout:
		if !strings.HasPrefix(line, "notesd listening on 127.0.0.1:") {
			t.Fatalf("stdout = %q", line)
		}
	case err := <-done:
		t.Fatalf("serve returned early: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not stop after cancel")
	}
}
