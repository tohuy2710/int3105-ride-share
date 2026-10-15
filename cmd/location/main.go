package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/location"
	"int3105-ride-share/internal/platform/app"
	"log"
)

func main() {
	if err := app.Run("location-service", "8082", func(r *gin.Engine) { location.RegisterRoutes(r, location.MockService{}) }); err != nil {
		log.Fatal(err)
	}
}
