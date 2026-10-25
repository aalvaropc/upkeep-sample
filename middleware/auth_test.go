package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aalvaropc/upkeep-sample/auth"
	"github.com/aalvaropc/upkeep-sample/clock"
)

func setup(t *testing.T) (*auth.Issuer, *clock.Manual, http.Handler) {
	t.Helper()
	c := clock.NewManual(time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC))
	issuer, err := auth.NewIssuer(bytes.Repeat([]byte("k"), auth.MinKeyLength), "notesd", time.Hour, c)
	if err != nil {
		t.Fatal(err)
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, _ := SubjectFrom(r.Context())
		w.Write([]byte("hello " + sub))
	})
	authn := NewAuthenticator(issuer)
	mux := http.NewServeMux()
	mux.Handle("GET /read", authn.RequireAuth(ok))
	mux.Handle("GET /write", authn.RequireAuth(RequireScope("notes:write", ok)))
	return issuer, c, mux
}

func do(h http.Handler, path, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("error body is not JSON: %v", err)
	}
	return body.Error.Code
}

func TestRequireAuth(t *testing.T) {
	issuer, c, h := setup(t)
	token, err := issuer.Issue("alice", "notes:read")
	if err != nil {
		t.Fatal(err)
	}

	rec := do(h, "/read", "Bearer "+token)
	if rec.Code != http.StatusOK || rec.Body.String() != "hello alice" {
		t.Fatalf("valid token: %d %q", rec.Code, rec.Body.String())
	}

	tests := []struct {
		name, header, challenge string
	}{
		{"no header", "", `error="invalid_request"`},
		{"basic scheme", "Basic dXNlcg==", `error="invalid_request"`},
		{"empty token", "Bearer ", `error="invalid_request"`},
		{"two tokens", "Bearer a b", `error="invalid_request"`},
		{"garbage", "Bearer abc.def.ghi", `error="invalid_token"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(h, "/read", tt.header)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, tt.challenge) {
				t.Fatalf("WWW-Authenticate = %q, want %s", got, tt.challenge)
			}
			if code := errorCode(t, rec); code != "unauthorized" {
				t.Fatalf("error code = %q", code)
			}
		})
	}

	c.Advance(2 * time.Hour)
	rec = do(h, "/read", "bearer "+token)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "expired") {
		t.Fatalf("expired token: %d %q", rec.Code, rec.Body.String())
	}
}

func TestRequireScope(t *testing.T) {
	issuer, _, h := setup(t)
	reader, err := issuer.Issue("alice", "notes:read")
	if err != nil {
		t.Fatal(err)
	}
	writer, err := issuer.Issue("bob", "notes:read", "notes:write")
	if err != nil {
		t.Fatal(err)
	}
	rec := do(h, "/write", "Bearer "+reader)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reader on /write: status = %d, want 403", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, `error="insufficient_scope"`) || !strings.Contains(got, `scope="notes:write"`) {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	if code := errorCode(t, rec); code != "forbidden" {
		t.Fatalf("error code = %q", code)
	}
	if rec := do(h, "/write", "Bearer "+writer); rec.Code != http.StatusOK || rec.Body.String() != "hello bob" {
		t.Fatalf("writer on /write: %d %q", rec.Code, rec.Body.String())
	}
}

func TestRequireScopeWithoutAuth(t *testing.T) {
	h := RequireScope("notes:write", http.NotFoundHandler())
	rec := do(h, "/", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestSubjectFromEmptyContext(t *testing.T) {
	if _, ok := SubjectFrom(t.Context()); ok {
		t.Fatal("SubjectFrom found a subject in an empty context")
	}
}
