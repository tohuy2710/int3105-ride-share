package location

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

func RegisterRoutes(r *gin.Engine, service Service) {
	h := &Handler{service: service}
	r.POST("/drivers/location", api.Auth("DRIVER"), h.UpdateLocation)
	r.PATCH("/drivers/availability", api.Auth("DRIVER"), h.UpdateAvailability)
	r.GET("/internal/drivers/nearby", api.Internal(), h.NearbyDrivers)
}
