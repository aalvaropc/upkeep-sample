package api

import (
	"net/http"

	"github.com/rs/cors"
)

func WithCORS(next http.Handler, origins []string) http.Handler {
	if len(origins) == 0 {
		return next
	}
	return cors.New(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         600,
	}).Handler(next)
}
