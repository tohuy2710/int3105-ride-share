package main

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/app"
	"int3105-ride-share/internal/ride/runtime"
	"log"
)

func main() {
	rt, err := runtime.New()
	if err != nil {
		log.Fatal(err)
	}
	defer rt.Close()
	if err := rt.StartDispatcher(); err != nil {
		log.Fatal(err)
	}
	if err := app.Run("ride-service", "8081", func(r *gin.Engine) { rt.Register(r) }); err != nil {
		log.Fatal(err)
	}
}
