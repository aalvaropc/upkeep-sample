package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q", got)
	}
}

func TestRunRejectsArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), []string{"extra"}, &stdout, &stderr, func(string) string { return "" }); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr = %q", stderr.String())
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
	go func() { done <- serve(ctx, "127.0.0.1:0", newHandler(), stdout) }()
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
