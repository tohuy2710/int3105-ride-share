package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/matching"
	"int3105-ride-share/internal/platform/app"
	"log"
)

func main() {
	if err := app.Run("matching-service", "8083", func(r *gin.Engine) { matching.RegisterRoutes(r, matching.MockService{}) }); err != nil {
		log.Fatal(err)
	}
}
