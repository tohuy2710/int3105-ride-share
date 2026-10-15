package matching

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, service Service) {
	h := &Handler{service: service}
	r.POST("/internal/matching/requests", h.RequestMatching)
	r.GET("/internal/matching/requests/:rideId", h.GetMatching)
}
