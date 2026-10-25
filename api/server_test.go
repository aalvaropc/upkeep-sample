package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aalvaropc/upkeep-sample/auth"
	"github.com/aalvaropc/upkeep-sample/clock"
	"github.com/aalvaropc/upkeep-sample/middleware"
	"github.com/aalvaropc/upkeep-sample/notes"
	"github.com/aalvaropc/upkeep-sample/store"
)

type client struct {
	t      *testing.T
	srv    *httptest.Server
	issuer *auth.Issuer
}

func newClient(t *testing.T) *client {
	t.Helper()
	c := clock.NewManual(time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC))
	issuer, err := auth.NewIssuer(bytes.Repeat([]byte("k"), auth.MinKeyLength), "notesd", time.Hour, c)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewServer(notes.NewService(store.NewMemory(), c), middleware.NewAuthenticator(issuer)))
	t.Cleanup(srv.Close)
	return &client{t: t, srv: srv, issuer: issuer}
}

func (c *client) token(sub string, scopes ...string) string {
	c.t.Helper()
	token, err := c.issuer.Issue(sub, scopes...)
	if err != nil {
		c.t.Fatal(err)
	}
	return token
}

func (c *client) do(method, path, token, body string) (*http.Response, []byte) {
	c.t.Helper()
	req, err := http.NewRequestWithContext(c.t.Context(), method, c.srv.URL+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.srv.Client().Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return resp, data
}

func decode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return v
}

type apiErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestHealthz(t *testing.T) {
	c := newClient(t)
	resp, body := c.do(http.MethodGet, "/healthz", "", "")
	if resp.StatusCode != http.StatusOK || string(body) != "{\"status\":\"ok\"}\n" {
		t.Fatalf("healthz: %d %s", resp.StatusCode, body)
	}
}

func TestCRUD(t *testing.T) {
	c := newClient(t)
	writer := c.token("alice", WriteScope)

	resp, body := c.do(http.MethodPost, "/v1/notes", writer, `{"title":"Groceries","body":"milk","tags":["home"]}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	created := decode[notes.Note](t, body)
	if resp.Header.Get("Location") != "/v1/notes/"+created.ID || created.Owner != "alice" {
		t.Fatalf("create: location %q, note %+v", resp.Header.Get("Location"), created)
	}

	resp, body = c.do(http.MethodGet, "/v1/notes/"+created.ID, writer, "")
	if resp.StatusCode != http.StatusOK || decode[notes.Note](t, body).Title != "Groceries" {
		t.Fatalf("get: %d %s", resp.StatusCode, body)
	}

	resp, body = c.do(http.MethodPut, "/v1/notes/"+created.ID, writer, `{"title":"Groceries","body":"milk and eggs"}`)
	if resp.StatusCode != http.StatusOK || decode[notes.Note](t, body).Body != "milk and eggs" {
		t.Fatalf("update: %d %s", resp.StatusCode, body)
	}

	resp, body = c.do(http.MethodGet, "/v1/notes?limit=5", writer, "")
	list := decode[notes.List](t, body)
	if resp.StatusCode != http.StatusOK || list.Total != 1 || list.Limit != 5 || len(list.Notes) != 1 {
		t.Fatalf("list: %d %s", resp.StatusCode, body)
	}

	resp, _ = c.do(http.MethodDelete, "/v1/notes/"+created.ID, writer, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, body = c.do(http.MethodGet, "/v1/notes/"+created.ID, writer, "")
	if resp.StatusCode != http.StatusNotFound || decode[apiErrorBody](t, body).Error.Code != "not_found" {
		t.Fatalf("get after delete: %d %s", resp.StatusCode, body)
	}
}

func TestAuthorization(t *testing.T) {
	c := newClient(t)
	reader := c.token("alice")

	if resp, body := c.do(http.MethodGet, "/v1/notes", "", ""); resp.StatusCode != http.StatusUnauthorized ||
		decode[apiErrorBody](t, body).Error.Code != "unauthorized" {
		t.Fatalf("list without a token: %d %s", resp.StatusCode, body)
	}
	if resp, _ := c.do(http.MethodGet, "/v1/notes", reader, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("list with a read token: %d", resp.StatusCode)
	}
	if resp, body := c.do(http.MethodPost, "/v1/notes", reader, `{"title":"x"}`); resp.StatusCode != http.StatusForbidden ||
		decode[apiErrorBody](t, body).Error.Code != "forbidden" {
		t.Fatalf("create with a read token: %d %s", resp.StatusCode, body)
	}
	if resp, _ := c.do(http.MethodDelete, "/v1/notes/abc", reader, ""); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("delete with a read token: %d", resp.StatusCode)
	}
}

func TestBadRequests(t *testing.T) {
	c := newClient(t)
	writer := c.token("alice", WriteScope)
	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"not json", http.MethodPost, "/v1/notes", "title=x", http.StatusBadRequest, "bad_request"},
		{"unknown field", http.MethodPost, "/v1/notes", `{"title":"x","color":"red"}`, http.StatusBadRequest, "bad_request"},
		{"two objects", http.MethodPost, "/v1/notes", `{"title":"x"}{"title":"y"}`, http.StatusBadRequest, "bad_request"},
		{"empty title", http.MethodPost, "/v1/notes", `{"title":" "}`, http.StatusUnprocessableEntity, "invalid"},
		{"too large", http.MethodPost, "/v1/notes", `{"title":"` + strings.Repeat("a", maxRequestBytes) + `"}`, http.StatusRequestEntityTooLarge, "too_large"},
		{"bad limit", http.MethodGet, "/v1/notes?limit=many", "", http.StatusBadRequest, "bad_request"},
		{"negative offset", http.MethodGet, "/v1/notes?offset=-1", "", http.StatusBadRequest, "bad_request"},
		{"update missing", http.MethodPut, "/v1/notes/missing", `{"title":"x"}`, http.StatusNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := c.do(tt.method, tt.path, writer, tt.body)
			if resp.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d; body %s", resp.StatusCode, tt.status, body)
			}
			if got := decode[apiErrorBody](t, body).Error.Code; got != tt.code {
				t.Fatalf("code = %q, want %q", got, tt.code)
			}
		})
	}
}

func TestOwnersAreIsolated(t *testing.T) {
	c := newClient(t)
	resp, body := c.do(http.MethodPost, "/v1/notes", c.token("alice", WriteScope), `{"title":"mine"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	id := decode[notes.Note](t, body).ID
	if resp, _ := c.do(http.MethodGet, "/v1/notes/"+id, c.token("bob"), ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("bob reading alice's note: %d", resp.StatusCode)
	}
}
