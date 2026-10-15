package notification

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

type Handler struct{ service Service }

func (h *Handler) SendRideRequest(c *gin.Context) {
	var input Request
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.SendRideRequest(c.Request.Context(), input)
	if err != nil {
		api.Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
		return
	}
	c.JSON(202, result)
}
