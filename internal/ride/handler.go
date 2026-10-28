package ride

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/api"
)

type Handler struct{ service Service }

func (h *Handler) EstimateFare(c *gin.Context) {
	var input FareRequest
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.EstimateFare(c.Request.Context(), c.GetString("riderId"), FareInput{
		PickupLocation: contracts.Coordinates{Lat: *input.PickupLocation.Lat, Long: *input.PickupLocation.Long},
		Destination:    contracts.Coordinates{Lat: *input.Destination.Lat, Long: *input.Destination.Long},
	})
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *Handler) CreateRide(c *gin.Context) {
	var input CreateRequest
	if !api.BindJSON(c, &input) {
		return
	}
	radius := 3000.0
	if input.RadiusMeters != nil {
		radius = *input.RadiusMeters
	}
	result, err := h.service.CreateRideWithMatching(c.Request.Context(), c.GetString("riderId"), input.FareID, contracts.MatchingRequest{PickupLocation: contracts.Coordinates{Lat: *input.PickupLocation.Lat, Long: *input.PickupLocation.Long}, RadiusMeters: radius})
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(201, result)
}
func (h *Handler) GetRide(c *gin.Context) {
	id, ok := api.RideID(c)
	if !ok {
		return
	}
	result, err := h.service.GetRide(c.Request.Context(), id, api.User(c))
	if err != nil {
		api.ServiceError(c, err)
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
	result, err := h.service.RespondToRide(c.Request.Context(), id, c.GetString("driverId"), input.Action)
	if err != nil {
		api.ServiceError(c, err)
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
	result, err := h.service.UpdateStatus(c.Request.Context(), id, c.GetString("driverId"), input.Status)
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(200, result)
}

func (h *Handler) CancelRide(c *gin.Context) {
	id, ok := api.RideID(c)
	if !ok {
		return
	}
	result, err := h.service.CancelRide(c.Request.Context(), id, c.GetString("riderId"))
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *Handler) InternalRide(c *gin.Context) {
	id, ok := api.RideID(c)
	if !ok {
		return
	}
	result, err := h.service.InternalRide(c.Request.Context(), id)
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(200, result)
}
