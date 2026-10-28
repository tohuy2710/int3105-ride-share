package matching

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/api"
)

type Handler struct{ service Service }

func (h *Handler) RequestMatching(c *gin.Context) {
	var fields InputFields
	if !api.BindJSON(c, &fields) {
		return
	}
	input, err := fields.Normalize()
	if err != nil {
		api.Fail(c, 400, "VALIDATION_ERROR", "Invalid matching input", err.Error())
		return
	}
	result, err := h.service.RequestMatching(c.Request.Context(), contracts.MatchingRequest{RideID: input.RideID, PickupLocation: input.PickupLocation, RadiusMeters: input.RadiusMeters})
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(202, result)
}
func (h *Handler) GetMatching(c *gin.Context) {
	id, ok := api.RideID(c)
	if !ok {
		return
	}
	result, err := h.service.GetMatching(c.Request.Context(), id)
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(200, result)
}
