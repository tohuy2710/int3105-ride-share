package auth_test

import (
	"bytes"
	"encoding/json"
	"int3105-ride-share/internal/auth"
	"int3105-ride-share/internal/auth/fixtures"
	"int3105-ride-share/internal/platform/api"
	"int3105-ride-share/internal/platform/security"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

func TestFixtureLoginAndMockRegistration(t *testing.T) {
	t.Setenv("JWT_SECRET", "auth-test-secret-of-at-least-32-bytes")
	store, err := fixtures.Load("../../fixtures/accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := security.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	router := api.NewRouter("auth-service", slog.New(slog.NewTextHandler(io.Discard, nil)))
	auth.RegisterRoutes(router, auth.Service{Credentials: store, Tokens: manager})
	call := func(path, body string, want int) map[string]any {
		req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := call("/auth/register", `{"email":"new@example.com","password":"RideShare123!","role":"DRIVER"}`, 201)
	if out["persisted"] != false || out["userId"] != "user_mock" {
		t.Fatal(out)
	}
	call("/auth/login", `{"email":"new@example.com","password":"RideShare123!"}`, 401)
	call("/auth/login", `{"email":"driver1@example.com","password":"incorrect"}`, 401)
	call("/auth/register", `{"email":"x","password":"short","role":"ADMIN"}`, 400)
	out = call("/auth/login", `{"email":"driver1@example.com","password":"RideShare123!"}`, 200)
	token := out["accessToken"].(string)
	user, err := manager.Parse(token)
	if err != nil || user.UserID != "driver_456" || user.Role != "DRIVER" {
		t.Fatalf("%+v %v", user, err)
	}
	req := httptest.NewRequest("GET", "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}
