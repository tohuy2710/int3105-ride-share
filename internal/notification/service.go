package notification

import "context"

// Service is the extension point for a future business implementation.
type Service interface {
	SendRideRequest(ctx context.Context, input Request) (Response, error)
}

// MockService is stateless: no persistence, transitions or inter-service calls.
type MockService struct{}

func (MockService) SendRideRequest(ctx context.Context, input Request) (Response, error) {
	return Response{"notification_123", input.RideID, input.DriverID, input.Type, "QUEUED"}, nil
}
