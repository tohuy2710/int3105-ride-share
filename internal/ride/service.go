package ride

import "context"

// Service is the extension point for a future business implementation.
type Service interface {
	EstimateFare(ctx context.Context, identity string, input FareRequest) (FareResponse, error)
	CreateRide(ctx context.Context, identity string, input CreateRequest) (RideResponse, error)
	GetRide(ctx context.Context, id string, identity string) (DetailsResponse, error)
	RespondToRide(ctx context.Context, id string, identity string, input ActionRequest) (ActionResponse, error)
	UpdateStatus(ctx context.Context, id string, identity string, input StatusRequest) (RideResponse, error)
}

// MockService is stateless: no persistence, transitions or inter-service calls.
type MockService struct{}

func (MockService) EstimateFare(ctx context.Context, identity string, input FareRequest) (FareResponse, error) {
	return FareResponse{"fare_123", 85000, 12}, nil
}
func (MockService) CreateRide(ctx context.Context, identity string, input CreateRequest) (RideResponse, error) {
	return RideResponse{"ride_123", "REQUESTED"}, nil
}
func (MockService) GetRide(ctx context.Context, id string, identity string) (DetailsResponse, error) {
	return DetailsResponse{id, "fare_123", identity, "REQUESTED"}, nil
}
func (MockService) RespondToRide(ctx context.Context, id string, identity string, input ActionRequest) (ActionResponse, error) {
	status := "REQUESTED"
	if input.Action == "ACCEPT" {
		status = "ACCEPTED"
	}
	return ActionResponse{id, input.Action, status}, nil
}
func (MockService) UpdateStatus(ctx context.Context, id string, identity string, input StatusRequest) (RideResponse, error) {
	return RideResponse{id, input.Status}, nil
}
