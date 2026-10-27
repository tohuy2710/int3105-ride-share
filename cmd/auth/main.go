package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/auth"
	"int3105-ride-share/internal/auth/fixtures"
	"int3105-ride-share/internal/platform/app"
	"int3105-ride-share/internal/platform/security"
	"log"
	"os"
)

func main() {
	path := os.Getenv("AUTH_FIXTURES")
	if path == "" {
		path = "fixtures/accounts.json"
	}
	store, err := fixtures.Load(path)
	if err != nil {
		log.Fatal(err)
	}
	tokens, err := security.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if err = app.Run("auth-service", "8085", func(r *gin.Engine) { auth.RegisterRoutes(r, auth.Service{Credentials: store, Tokens: tokens}) }); err != nil {
		log.Fatal(err)
	}
}
