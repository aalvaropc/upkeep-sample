package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/aalvaropc/upkeep-sample/middleware"
	"github.com/aalvaropc/upkeep-sample/notes"
)

type apiError struct {
	status  int
	code    string
	message string
}

func (e *apiError) Error() string { return e.message }

var errUnauthenticated = &apiError{status: http.StatusUnauthorized, code: "unauthorized", message: "authentication is required"}

func badRequest(message string) error {
	return &apiError{status: http.StatusBadRequest, code: "bad_request", message: message}
}

func writeError(w http.ResponseWriter, err error) {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
		middleware.WriteError(w, ae.status, ae.code, ae.message)
	case errors.Is(err, notes.ErrNotFound):
		middleware.WriteError(w, http.StatusNotFound, "not_found", "note not found")
	case errors.Is(err, notes.ErrInvalid):
		middleware.WriteError(w, http.StatusUnprocessableEntity, "invalid", strings.TrimPrefix(err.Error(), notes.ErrInvalid.Error()+": "))
	default:
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
