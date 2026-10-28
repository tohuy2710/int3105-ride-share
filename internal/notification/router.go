package notification

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

func RegisterRoutes(r *gin.Engine, service Service) {
	h := &Handler{service: service}
	r.GET("/internal/notifications/:notificationId", api.Internal(), h.GetNotification)
	r.POST("/internal/notifications/ride-requests", api.Internal(), h.SendRideRequest)
}
