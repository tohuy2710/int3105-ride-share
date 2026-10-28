package notification

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/api"
)

type Handler struct{ service Service }

func (h *Handler) SendRideRequest(c *gin.Context) {
	var input Request
	if !api.BindJSON(c, &input) {
		return
	}
	result, err := h.service.SendRideRequest(c.Request.Context(), contracts.NotificationRequest{RideID: input.RideID, DriverID: input.DriverID, Type: input.Type})
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(202, result)
}

func (h *Handler) GetNotification(c *gin.Context) {
	n, err := h.service.GetNotification(c.Request.Context(), c.Param("notificationId"))
	if err != nil {
		api.ServiceError(c, err)
		return
	}
	c.JSON(200, n.Response())
}
