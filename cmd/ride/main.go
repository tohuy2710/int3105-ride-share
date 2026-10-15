package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/app"
	"int3105-ride-share/internal/ride"
	"log"
)

func main() {
	if err := app.Run("ride-service", "8081", func(r *gin.Engine) { ride.RegisterRoutes(r, ride.MockService{}) }); err != nil {
		log.Fatal(err)
	}
}
