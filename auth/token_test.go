package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hash"
	"strings"
	"testing"
	"time"

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

func forge(t *testing.T, alg string, claims map[string]any, key []byte) string {
	t.Helper()
	encode := func(v any) string {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(data)
	}
	signingInput := encode(map[string]string{"alg": alg, "typ": "JWT"}) + "." + encode(claims)
	var newHash func() hash.Hash
	switch alg {
	case "HS256":
		newHash = sha256.New
	case "HS512":
		newHash = sha512.New
	case "none":
		return signingInput + "."
	default:
		t.Fatalf("unsupported algorithm %q", alg)
	}
	mac := hmac.New(newHash, key)
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validClaims() map[string]any {
	return map[string]any{"sub": "alice", "iss": "notesd", "exp": start().Add(time.Hour).Unix()}
}

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
	if want := start().Add(time.Hour); !claims.Expiry().Equal(want) {
		t.Fatalf("expires at %v, want %v", claims.Expiry(), want)
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
	token := forge(t, "none", validClaims(), nil)
	if _, err := newIssuer(t, c).Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse error = %v, want ErrInvalidToken", err)
	}
}

func TestOtherAlgorithmIsRejected(t *testing.T) {
	c := clock.NewManual(start())
	token := forge(t, "HS512", validClaims(), testKey)
	if _, err := newIssuer(t, c).Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse error = %v, want ErrInvalidToken", err)
	}
}

func TestEmptySubjectIsRejected(t *testing.T) {
	c := clock.NewManual(start())
	claims := validClaims()
	delete(claims, "sub")
	token := forge(t, "HS256", claims, testKey)
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

func TestForgedValidTokenIsAccepted(t *testing.T) {
	c := clock.NewManual(start())
	token := forge(t, "HS256", validClaims(), testKey)
	claims, err := newIssuer(t, c).Parse(token)
	if err != nil {
		t.Fatalf("a hand-built HS256 token with valid claims was rejected: %v", err)
	}
	if claims.Subject != "alice" {
		t.Fatalf("subject = %q", claims.Subject)
	}
}
