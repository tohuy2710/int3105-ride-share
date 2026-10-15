package ride

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

func RegisterRoutes(r *gin.Engine, service Service) {
	h := &Handler{service: service}
	r.POST("/fare", api.Identity("riderId", "rider_123"), h.EstimateFare)
	r.POST("/rides", api.Identity("riderId", "rider_123"), h.CreateRide)
	r.GET("/rides/:rideId", api.Identity("riderId", "rider_123"), h.GetRide)
	r.PATCH("/rides/:rideId", api.Identity("driverId", "driver_456"), h.RespondToRide)
	r.PATCH("/rides/:rideId/status", api.Identity("driverId", "driver_456"), h.UpdateStatus)
}
