package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/notification"
	"int3105-ride-share/internal/notification/postgres"
	"int3105-ride-share/internal/platform/app"
	"int3105-ride-share/internal/platform/config"
	"int3105-ride-share/internal/platform/database"
	"log"
	"time"
)

func main() {
	db, err := database.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	timeout, err := config.Duration("DB_TIMEOUT", 2*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	service := notification.NewApplication(postgres.NewRepository(db))
	if err = app.Run("notification-service", "8084", func(r *gin.Engine) {
		database.Register(r, db, "notification.requests", timeout)
		notification.RegisterRoutes(r, service)
	}); err != nil {
		log.Fatal(err)
	}
}
