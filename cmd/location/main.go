package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/location"
	"int3105-ride-share/internal/location/postgres"
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

	service := location.NewApplication(postgres.NewRepository(db))
	if err = app.Run("location-service", "8082", func(r *gin.Engine) {
		database.Register(r, db, "location.drivers", timeout)
		location.RegisterRoutes(r, service)
	}); err != nil {
		log.Fatal(err)
	}
}
