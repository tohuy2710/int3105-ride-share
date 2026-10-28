package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/clients"
	"int3105-ride-share/internal/matching"
	"int3105-ride-share/internal/matching/postgres"
	"int3105-ride-share/internal/platform/app"
	"int3105-ride-share/internal/platform/config"
	"int3105-ride-share/internal/platform/database"
	"log"
	"os"
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
	rides, err := clients.NewHTTP(config.Env("RIDE_URL", "http://localhost:8081"), os.Getenv("INTERNAL_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	drivers, err := clients.NewHTTP(config.Env("LOCATION_URL", "http://localhost:8082"), os.Getenv("INTERNAL_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	notifications, err := clients.NewHTTP(config.Env("NOTIFICATION_URL", "http://localhost:8084"), os.Getenv("INTERNAL_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	service := matching.NewApplication(postgres.NewRepository(db), rides, drivers, notifications)
	if err = app.Run("matching-service", "8083", func(r *gin.Engine) {
		database.Register(r, db, "matching.requests", timeout)
		matching.RegisterRoutes(r, service)
	}); err != nil {
		log.Fatal(err)
	}
}
