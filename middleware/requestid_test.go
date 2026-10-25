package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func requestID(t *testing.T, incoming string) string {
	t.Helper()
	h := RequestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if incoming != "" {
		req.Header.Set(RequestIDHeader, incoming)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Header().Get(RequestIDHeader)
}

func TestRequestIDIsGeneratedWhenMissing(t *testing.T) {
	got := requestID(t, "")
	if _, err := ulid.ParseStrict(got); err != nil {
		t.Fatalf("generated id %q is not a ULID: %v", got, err)
	}
}

func TestRequestIDKeepsAValidIncomingID(t *testing.T) {
	incoming := ulid.Make().String()
	if got := requestID(t, incoming); got != incoming {
		t.Fatalf("id = %q, want the incoming %q", got, incoming)
	}
}

func TestRequestIDReplacesAnythingElse(t *testing.T) {
	for _, bad := range []string{"abc", "../../etc/passwd", strings.Repeat("a", 64)} {
		got := requestID(t, bad)
		if got == bad {
			t.Errorf("the id %q was passed through", bad)
		}
		if _, err := ulid.ParseStrict(got); err != nil {
			t.Errorf("replacement %q for %q is not a ULID: %v", got, bad, err)
		}
	}
}
