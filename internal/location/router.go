package location

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

func RegisterRoutes(r *gin.Engine, service Service) {
	h := &Handler{service: service}
	r.POST("/drivers/location", api.Identity("driverId", "driver_456"), h.UpdateLocation)
	r.PATCH("/drivers/availability", api.Identity("driverId", "driver_456"), h.UpdateAvailability)
	r.GET("/internal/drivers/nearby", h.NearbyDrivers)
}
