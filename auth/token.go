package auth

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/aalvaropc/upkeep-sample/clock"
)

const MinKeyLength = 32

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpired      = errors.New("token expired")
)

type Claims struct {
	Scope string `json:"scope,omitempty"`
	jwt.StandardClaims
}

func (c Claims) Scopes() []string { return strings.Fields(c.Scope) }

func (c Claims) HasScope(scope string) bool { return slices.Contains(c.Scopes(), scope) }

type Issuer struct {
	key   []byte
	name  string
	ttl   time.Duration
	clock clock.Clock
}

func NewIssuer(key []byte, name string, ttl time.Duration, c clock.Clock) (*Issuer, error) {
	if len(key) < MinKeyLength {
		return nil, fmt.Errorf("signing key must be at least %d bytes", MinKeyLength)
	}
	if ttl <= 0 {
		return nil, errors.New("token lifetime must be positive")
	}
	return &Issuer{key: slices.Clone(key), name: name, ttl: ttl, clock: c}, nil
}

func (i *Issuer) Issue(subject string, scopes ...string) (string, error) {
	if subject == "" {
		return "", errors.New("subject is required")
	}
	now := i.clock.Now()
	claims := Claims{
		Scope: strings.Join(scopes, " "),
		StandardClaims: jwt.StandardClaims{
			Subject:   subject,
			Issuer:    i.name,
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(i.ttl).Unix(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.key)
}

func (i *Issuer) Parse(token string) (Claims, error) {
	var claims Claims
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithoutClaimsValidation())
	_, err := parser.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return i.key, nil
	})
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	now := i.clock.Now().Unix()
	if !claims.VerifyExpiresAt(now, true) {
		return Claims{}, ErrExpired
	}
	if !claims.VerifyIssuer(i.name, true) || claims.Subject == "" {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}
