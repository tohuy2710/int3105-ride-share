package matching

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

func RegisterRoutes(r *gin.Engine, service Service) {
	h := &Handler{service: service}
	r.POST("/internal/matching/requests", api.Internal(), h.RequestMatching)
	r.GET("/internal/matching/requests/:rideId", api.Internal(), h.GetMatching)
}
