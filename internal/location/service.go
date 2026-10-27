package location

import (
	"context"
	"int3105-ride-share/internal/contracts"
)

// Service is the extension point for a future business implementation.
type Service interface {
	UpdateLocation(ctx context.Context, identity string, input contracts.Coordinates) (LocationResponse, error)
	UpdateAvailability(ctx context.Context, identity string, available bool) (AvailabilityResponse, error)
	NearbyDrivers(ctx context.Context, input contracts.NearbyInput) (NearbyResponse, error)
}

// MockService is stateless: no persistence, transitions or inter-service calls.
type MockService struct{}

func (MockService) UpdateLocation(ctx context.Context, identity string, input contracts.Coordinates) (LocationResponse, error) {
	return LocationResponse{identity, input.Lat, input.Long}, nil
}
func (MockService) UpdateAvailability(ctx context.Context, identity string, available bool) (AvailabilityResponse, error) {
	return AvailabilityResponse{identity, available}, nil
}
func (MockService) NearbyDrivers(ctx context.Context, input contracts.NearbyInput) (NearbyResponse, error) {
	lat, long := 21.0285, 105.8542
	return NearbyResponse{Drivers: []Driver{{DriverID: "driver_456", Location: contracts.Coordinates{Lat: lat, Long: long}, DistanceMeters: 500}}}, nil
}
