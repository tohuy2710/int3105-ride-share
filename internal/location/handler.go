package location

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/api"
)

type Handler struct{ service Service }

func (h *Handler) UpdateLocation(c *gin.Context) {
	var input LocationRequest
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.UpdateLocation(c.Request.Context(), c.GetString("driverId"), contracts.Coordinates{Lat: *input.Lat, Long: *input.Long})
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(200, result)
}
func (h *Handler) UpdateAvailability(c *gin.Context) {
	var input AvailabilityRequest
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.UpdateAvailability(c.Request.Context(), c.GetString("driverId"), *input.Available)
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(200, result)
}
func (h *Handler) NearbyDrivers(c *gin.Context) {
	var input NearbyQuery
	if !api.BindQuery(c, &input) {
		return
	}
	result, err := h.service.NearbyDrivers(c.Request.Context(), contracts.NearbyInput{Lat: *input.Lat, Long: *input.Long, RadiusMeters: input.RadiusMeters})
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(200, result)
}
