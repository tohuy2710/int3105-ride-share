package ride

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

func RegisterRoutes(r *gin.Engine, service Service) {
	h := &Handler{service: service}
	r.POST("/fare", api.Auth("RIDER"), h.EstimateFare)
	r.POST("/rides", api.Auth("RIDER"), h.CreateRide)
	r.GET("/rides/:rideId", api.Auth(""), h.GetRide)
	r.PATCH("/rides/:rideId", api.Auth("DRIVER"), h.RespondToRide)
	r.PATCH("/rides/:rideId/status", api.Auth("DRIVER"), h.UpdateStatus)
	r.DELETE("/rides/:rideId", api.Auth("RIDER"), h.CancelRide)
	r.GET("/internal/rides/:rideId", api.Internal(), h.InternalRide)
}
