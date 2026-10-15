package ride

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

type Handler struct{ service Service }

func (h *Handler) EstimateFare(c *gin.Context) {
	var input FareRequest
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.EstimateFare(c.Request.Context(), c.GetString("riderId"), input)
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(200, result)
}
func (h *Handler) CreateRide(c *gin.Context) {
	var input CreateRequest
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.CreateRide(c.Request.Context(), c.GetString("riderId"), input)
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(201, result)
}
func (h *Handler) GetRide(c *gin.Context) {
	id, ok := api.RideID(c)
	if !ok {
		return
	}
	result, err := h.service.GetRide(c.Request.Context(), id, c.GetString("riderId"))
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(200, result)
}
func (h *Handler) RespondToRide(c *gin.Context) {
	id, ok := api.RideID(c)
	if !ok {
		return
	}
	var input ActionRequest
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.RespondToRide(c.Request.Context(), id, c.GetString("driverId"), input)
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(200, result)
}
func (h *Handler) UpdateStatus(c *gin.Context) {
	id, ok := api.RideID(c)
	if !ok {
		return
	}
	var input StatusRequest
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.UpdateStatus(c.Request.Context(), id, c.GetString("driverId"), input)
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(200, result)
}
