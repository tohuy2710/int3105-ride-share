package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/notification"
	"int3105-ride-share/internal/platform/app"
	"log"
)

func main() {
	if err := app.Run("notification-service", "8084", func(r *gin.Engine) { notification.RegisterRoutes(r, notification.MockService{}) }); err != nil {
		log.Fatal(err)
	}
}
