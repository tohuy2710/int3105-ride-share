// Package fixtures is the local credential adapter. It never creates users.
package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
)

type Account struct {
	contracts.User
	PasswordHash string `json:"passwordHash"`
}
type Store struct {
	accounts map[string]Account
	dummy    string
}

func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	if err = json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}
	s := &Store{accounts: make(map[string]Account)}
	ids := map[string]bool{}
	for _, a := range accounts {
		if strings.TrimSpace(a.UserID) == "" || a.Email == "" || (a.Role != "RIDER" && a.Role != "DRIVER") {
			return nil, fmt.Errorf("invalid fixture identity")
		}
		if _, err := bcrypt.Cost([]byte(a.PasswordHash)); err != nil {
			return nil, err
		}
		if _, ok := s.accounts[a.Email]; ok || ids[a.UserID] {
			return nil, fmt.Errorf("duplicate fixture identity")
		}
		s.accounts[a.Email] = a
		ids[a.UserID] = true
		s.dummy = a.PasswordHash
	}
	if len(s.accounts) == 0 {
		return nil, fmt.Errorf("empty fixture file")
	}
	return s, nil
}
func (s *Store) Authenticate(ctx context.Context, email, password string) (contracts.User, error) {
	if err := ctx.Err(); err != nil {
		return contracts.User{}, err
	}
	a, ok := s.accounts[email]
	hash := a.PasswordHash
	if !ok {
		hash = s.dummy
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if !ok || err != nil {
		return contracts.User{}, fault.Unauthorized
	}
	return a.User, nil
}
