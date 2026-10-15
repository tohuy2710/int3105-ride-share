package notification

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, service Service) {
	h := &Handler{service: service}
	r.POST("/internal/notifications/ride-requests", h.SendRideRequest)
}
