package security

import (
	"github.com/golang-jwt/jwt/v5"
	"int3105-ride-share/internal/contracts"
	"testing"
	"time"
)

func TestJWTValidation(t *testing.T) {
	secret := "test-secret-that-has-at-least-32-bytes"
	m, err := New(secret, "issuer", "audience")
	if err != nil {
		t.Fatal(err)
	}
	user := contracts.User{UserID: "driver_456", Email: "driver1@example.com", Role: "DRIVER"}
	raw, err := m.Issue(user)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Parse(raw)
	if err != nil || got != user {
		t.Fatalf("%+v %v", got, err)
	}
	other, _ := New(secret+"different", "issuer", "audience")
	if _, err = other.Parse(raw); err == nil {
		t.Fatal("accepted wrong signature")
	}
	for _, tc := range []struct {
		name   string
		change func(*Claims)
		method jwt.SigningMethod
	}{
		{"expired", func(c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }, jwt.SigningMethodHS256},
		{"no-expiry", func(c *Claims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256},
		{"no-issued-at", func(c *Claims) { c.IssuedAt = nil }, jwt.SigningMethodHS256},
		{"future-issued-at", func(c *Claims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }, jwt.SigningMethodHS256},
		{"issuer", func(c *Claims) { c.Issuer = "other" }, jwt.SigningMethodHS256},
		{"audience", func(c *Claims) { c.Audience = jwt.ClaimStrings{"other"} }, jwt.SigningMethodHS256},
		{"role", func(c *Claims) { c.Role = "ADMIN" }, jwt.SigningMethodHS256},
		{"subject", func(c *Claims) { c.Subject = " " }, jwt.SigningMethodHS256},
		{"algorithm", func(c *Claims) {}, jwt.SigningMethodHS384},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Claims{Role: user.Role, Email: user.Email, RegisteredClaims: jwt.RegisteredClaims{Subject: user.UserID, Issuer: "issuer", Audience: jwt.ClaimStrings{"audience"}, IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
			tc.change(&c)
			raw, err := jwt.NewWithClaims(tc.method, c).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.Parse(raw); err == nil {
				t.Fatal("accepted invalid token")
			}
		})
	}
}
