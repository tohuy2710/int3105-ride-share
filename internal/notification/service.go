package notification

import (
	"context"
	"int3105-ride-share/internal/contracts"
)

// Service is the extension point for a future business implementation.
type Service interface {
	SendRideRequest(ctx context.Context, input contracts.NotificationRequest) (Response, error)
	GetNotification(context.Context, string) (Notification, error)
}

// MockService is stateless: no persistence, transitions or inter-service calls.
type MockService struct{}

func (MockService) SendRideRequest(ctx context.Context, input contracts.NotificationRequest) (Response, error) {
	return Response{NotificationID: "notification_123", RideID: input.RideID, DriverID: input.DriverID, Type: input.Type, Status: "QUEUED"}, nil
}
