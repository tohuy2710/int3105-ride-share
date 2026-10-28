package auth

import (
	"context"
	"int3105-ride-share/internal/contracts"
)

type Credentials interface {
	Authenticate(context.Context, string, string) (contracts.User, error)
}
type TokenIssuer interface {
	Issue(contracts.User) (string, error)
}
type LoginResponse struct {
	AccessToken string         `json:"accessToken"`
	TokenType   string         `json:"tokenType"`
	ExpiresIn   int            `json:"expiresIn"`
	User        contracts.User `json:"user"`
}
type RegisterResponse struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Persisted bool   `json:"persisted"`
}
type Service struct {
	Credentials Credentials
	Tokens      TokenIssuer
}

func (s Service) Login(ctx context.Context, email, password string) (LoginResponse, error) {
	user, err := s.Credentials.Authenticate(ctx, email, password)
	if err != nil {
		return LoginResponse{}, err
	}
	token, err := s.Tokens.Issue(user)
	return LoginResponse{token, "Bearer", 3600, user}, err
}
func (s Service) Register(email, role string) RegisterResponse {
	return RegisterResponse{"user_mock", email, role, false}
}
