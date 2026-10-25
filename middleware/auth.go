package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aalvaropc/upkeep-sample/auth"
)

const realm = "notes"

type Verifier interface {
	Parse(token string) (auth.Claims, error)
}

type Authenticator struct {
	verifier Verifier
}

func NewAuthenticator(v Verifier) *Authenticator {
	return &Authenticator{verifier: v}
}

type claimsKey struct{}

var (
	errMissingToken   = errors.New("missing bearer token")
	errMalformedToken = errors.New("malformed authorization header")
)

func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			unauthorized(w, "invalid_request", err.Error())
			return
		}
		claims, err := a.verifier.Parse(token)
		switch {
		case errors.Is(err, auth.ErrExpired):
			unauthorized(w, "invalid_token", "the token has expired")
			return
		case err != nil:
			unauthorized(w, "invalid_token", "the token is not valid")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

func RequireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFrom(r.Context())
		if !ok {
			unauthorized(w, "invalid_token", "authentication is required")
			return
		}
		if !claims.HasScope(scope) {
			forbidden(w, scope)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func WithClaims(ctx context.Context, claims auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

func ClaimsFrom(ctx context.Context) (auth.Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(auth.Claims)
	return claims, ok
}

func SubjectFrom(ctx context.Context) (string, bool) {
	claims, ok := ClaimsFrom(ctx)
	if !ok || claims.Subject == "" {
		return "", false
	}
	return claims.Subject, true
}

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errMissingToken
	}
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", errMalformedToken
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, " \t") {
		return "", errMalformedToken
	}
	return token, nil
}

func unauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q, error=%q`, realm, code))
	WriteError(w, http.StatusUnauthorized, "unauthorized", message)
}

func forbidden(w http.ResponseWriter, scope string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q, error="insufficient_scope", scope=%q`, realm, scope))
	WriteError(w, http.StatusForbidden, "forbidden", fmt.Sprintf("the token lacks the %s scope", scope))
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorBody{Error: errorDetail{Code: code, Message: message}})
}
