package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func preflight(t *testing.T, handler http.Handler, origin string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodOptions, "/v1/notes", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestWithCORSAllowsListedOrigins(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	handler := WithCORS(next, []string{"https://app.example"})

	rec := preflight(t, handler, "https://app.example")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the listed origin", got)
	}
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatal("the preflight did not allow the Authorization header")
	}
}

func TestWithCORSRejectsOtherOrigins(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	handler := WithCORS(next, []string{"https://app.example"})

	rec := preflight(t, handler, "https://evil.example")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q for an unlisted origin", got)
	}
}

func TestWithCORSWithoutOriginsIsTransparent(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	rec := preflight(t, WithCORS(next, nil), "https://app.example")
	if rec.Code != http.StatusTeapot || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("code %d with headers %v, want the wrapped handler untouched", rec.Code, rec.Header())
	}
}
