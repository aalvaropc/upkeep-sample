package auth

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/aalvaropc/upkeep-sample/clock"
)

var testKey = bytes.Repeat([]byte("k"), MinKeyLength)

func newIssuer(t *testing.T, c clock.Clock) *Issuer {
	t.Helper()
	i, err := NewIssuer(testKey, "notesd", time.Hour, c)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func start() time.Time { return time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC) }

func TestRoundTrip(t *testing.T) {
	i := newIssuer(t, clock.NewManual(start()))
	token, err := i.Issue("alice", "notes:read", "notes:write")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := i.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "alice" || claims.Issuer != "notesd" {
		t.Fatalf("claims = %+v", claims)
	}
	if !claims.HasScope("notes:write") || claims.HasScope("notes:admin") {
		t.Fatalf("scopes = %v", claims.Scopes())
	}
	if claims.ExpiresAt != start().Add(time.Hour).Unix() {
		t.Fatalf("expires at %d", claims.ExpiresAt)
	}
}

func TestExpired(t *testing.T) {
	c := clock.NewManual(start())
	i := newIssuer(t, c)
	token, err := i.Issue("alice")
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(2 * time.Hour)
	if _, err := i.Parse(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("Parse error = %v, want ErrExpired", err)
	}
}

func TestWrongKey(t *testing.T) {
	c := clock.NewManual(start())
	other, err := NewIssuer(bytes.Repeat([]byte("o"), MinKeyLength), "notesd", time.Hour, c)
	if err != nil {
		t.Fatal(err)
	}
	token, err := other.Issue("alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newIssuer(t, c).Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse error = %v, want ErrInvalidToken", err)
	}
}

func TestWrongIssuer(t *testing.T) {
	c := clock.NewManual(start())
	other, err := NewIssuer(testKey, "someone-else", time.Hour, c)
	if err != nil {
		t.Fatal(err)
	}
	token, err := other.Issue("alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newIssuer(t, c).Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse error = %v, want ErrInvalidToken", err)
	}
}

func TestUnsignedTokenIsRejected(t *testing.T) {
	c := clock.NewManual(start())
	claims := Claims{StandardClaims: jwt.StandardClaims{Subject: "alice", Issuer: "notesd", ExpiresAt: start().Add(time.Hour).Unix()}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newIssuer(t, c).Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse error = %v, want ErrInvalidToken", err)
	}
}

func TestOtherAlgorithmIsRejected(t *testing.T) {
	c := clock.NewManual(start())
	claims := Claims{StandardClaims: jwt.StandardClaims{Subject: "alice", Issuer: "notesd", ExpiresAt: start().Add(time.Hour).Unix()}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(testKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newIssuer(t, c).Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse error = %v, want ErrInvalidToken", err)
	}
}

func TestEmptySubjectIsRejected(t *testing.T) {
	c := clock.NewManual(start())
	claims := Claims{StandardClaims: jwt.StandardClaims{Issuer: "notesd", ExpiresAt: start().Add(time.Hour).Unix()}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newIssuer(t, c).Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse error = %v, want ErrInvalidToken", err)
	}
	if _, err := newIssuer(t, c).Issue(""); err == nil {
		t.Fatal("Issue accepted an empty subject")
	}
}

func TestGarbageIsRejected(t *testing.T) {
	i := newIssuer(t, clock.NewManual(start()))
	for _, token := range []string{"", "abc", strings.Repeat("a.", 3)} {
		if _, err := i.Parse(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalidToken", token, err)
		}
	}
}

func TestNewIssuerValidates(t *testing.T) {
	if _, err := NewIssuer([]byte("short"), "notesd", time.Hour, clock.System{}); err == nil {
		t.Error("NewIssuer accepted a short key")
	}
	if _, err := NewIssuer(testKey, "notesd", 0, clock.System{}); err == nil {
		t.Error("NewIssuer accepted a zero lifetime")
	}
}
