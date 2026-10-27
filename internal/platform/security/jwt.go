package security

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"int3105-ride-share/internal/contracts"
)

const Lifetime = time.Hour

type Claims struct {
	Role  string `json:"role"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}
type Manager struct {
	secret           []byte
	issuer, audience string
}

func New(secret, issuer, audience string) (*Manager, error) {
	if len(secret) < 32 || issuer == "" || audience == "" {
		return nil, fmt.Errorf("JWT_SECRET must have at least 32 bytes; issuer and audience are required")
	}
	return &Manager{[]byte(secret), issuer, audience}, nil
}
func FromEnv() (*Manager, error) {
	issuer, audience := os.Getenv("JWT_ISSUER"), os.Getenv("JWT_AUDIENCE")
	if issuer == "" {
		issuer = "ride-share"
	}
	if audience == "" {
		audience = "ride-share-api"
	}
	return New(os.Getenv("JWT_SECRET"), issuer, audience)
}
func (m *Manager) Issue(user contracts.User) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{Role: user.Role, Email: user.Email, RegisteredClaims: jwt.RegisteredClaims{
		Subject: user.UserID, Issuer: m.issuer, Audience: jwt.ClaimStrings{m.audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(Lifetime)),
	}}).SignedString(m.secret)
}
func (m *Manager) Parse(raw string) (contracts.User, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) { return m.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		return contracts.User{}, err
	}
	if strings.TrimSpace(c.Subject) == "" || c.IssuedAt == nil || c.Email == "" || (c.Role != "RIDER" && c.Role != "DRIVER") {
		return contracts.User{}, fmt.Errorf("invalid identity claims")
	}
	return contracts.User{UserID: c.Subject, Email: c.Email, Role: c.Role}, nil
}
