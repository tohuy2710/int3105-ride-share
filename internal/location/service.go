package location

import (
	"context"
	"int3105-ride-share/internal/platform/api"
)

// Service is the extension point for a future business implementation.
type Service interface {
	UpdateLocation(ctx context.Context, identity string, input LocationRequest) (LocationResponse, error)
	UpdateAvailability(ctx context.Context, identity string, input AvailabilityRequest) (AvailabilityResponse, error)
	NearbyDrivers(ctx context.Context, input NearbyQuery) (NearbyResponse, error)
}

// MockService is stateless: no persistence, transitions or inter-service calls.
type MockService struct{}

func (MockService) UpdateLocation(ctx context.Context, identity string, input LocationRequest) (LocationResponse, error) {
	return LocationResponse{identity, *input.Lat, *input.Long}, nil
}
func (MockService) UpdateAvailability(ctx context.Context, identity string, input AvailabilityRequest) (AvailabilityResponse, error) {
	return AvailabilityResponse{identity, *input.Available}, nil
}
func (MockService) NearbyDrivers(ctx context.Context, input NearbyQuery) (NearbyResponse, error) {
	lat, long := 21.0285, 105.8542
	return NearbyResponse{[]Driver{{"driver_456", api.Coordinates{Lat: &lat, Long: &long}, 500}}}, nil
}
