package api

import (
	"io"
	"net/http"

	"github.com/aalvaropc/upkeep-sample/middleware"
	"github.com/aalvaropc/upkeep-sample/notes"
)

const WriteScope = "notes:write"

type Server struct {
	notes *notes.Service
	authn *middleware.Authenticator
}

func NewServer(svc *notes.Service, authn *middleware.Authenticator) http.Handler {
	s := &Server{notes: svc, authn: authn}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("GET /v1/notes", s.authn.RequireAuth(http.HandlerFunc(s.listNotes)))
	mux.Handle("POST /v1/notes", s.writer(s.createNote))
	mux.Handle("GET /v1/notes/{id}", s.authn.RequireAuth(http.HandlerFunc(s.getNote)))
	mux.Handle("PUT /v1/notes/{id}", s.writer(s.updateNote))
	mux.Handle("DELETE /v1/notes/{id}", s.writer(s.deleteNote))
	return mux
}

func (s *Server) writer(h http.HandlerFunc) http.Handler {
	return s.authn.RequireAuth(middleware.RequireScope(WriteScope, h))
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, "{\"status\":\"ok\"}\n")
}
