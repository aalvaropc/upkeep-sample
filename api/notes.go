package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/aalvaropc/upkeep-sample/middleware"
	"github.com/aalvaropc/upkeep-sample/notes"
)

const maxRequestBytes = 64 << 10

func (s *Server) listNotes(w http.ResponseWriter, r *http.Request) {
	owner, ok := middleware.SubjectFrom(r.Context())
	if !ok {
		writeError(w, errUnauthenticated)
		return
	}
	page, err := pageFrom(r)
	if err != nil {
		writeError(w, err)
		return
	}
	list, err := s.notes.List(r.Context(), owner, page)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request) {
	owner, ok := middleware.SubjectFrom(r.Context())
	if !ok {
		writeError(w, errUnauthenticated)
		return
	}
	in, err := decodeInput(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	n, err := s.notes.Create(r.Context(), owner, in)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Location", "/v1/notes/"+n.ID)
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) getNote(w http.ResponseWriter, r *http.Request) {
	owner, ok := middleware.SubjectFrom(r.Context())
	if !ok {
		writeError(w, errUnauthenticated)
		return
	}
	n, err := s.notes.Get(r.Context(), owner, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) updateNote(w http.ResponseWriter, r *http.Request) {
	owner, ok := middleware.SubjectFrom(r.Context())
	if !ok {
		writeError(w, errUnauthenticated)
		return
	}
	in, err := decodeInput(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	n, err := s.notes.Update(r.Context(), owner, r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	owner, ok := middleware.SubjectFrom(r.Context())
	if !ok {
		writeError(w, errUnauthenticated)
		return
	}
	if err := s.notes.Delete(r.Context(), owner, r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pageFrom(r *http.Request) (notes.Page, error) {
	var page notes.Page
	q := r.URL.Query()
	for _, p := range []struct {
		name string
		dst  *int
	}{{"offset", &page.Offset}, {"limit", &page.Limit}} {
		raw := q.Get(p.name)
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return notes.Page{}, badRequest(fmt.Sprintf("%s must be a non-negative integer", p.name))
		}
		*p.dst = n
	}
	return page, nil
}

func decodeInput(w http.ResponseWriter, r *http.Request) (notes.Input, error) {
	var in notes.Input
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return notes.Input{}, &apiError{status: http.StatusRequestEntityTooLarge, code: "too_large", message: "the request body is too large"}
		}
		return notes.Input{}, badRequest("the request body must be a JSON note")
	}
	if dec.More() {
		return notes.Input{}, badRequest("the request body must contain a single JSON note")
	}
	return in, nil
}
