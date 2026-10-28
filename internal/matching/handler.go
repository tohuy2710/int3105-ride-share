package matching

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/api"
)

type Handler struct{ service Service }

func (h *Handler) RequestMatching(c *gin.Context) {
	var input Request
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.RequestMatching(c.Request.Context(), contracts.MatchingRequest{RideID: input.RideID})
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
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
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(200, result)
}
