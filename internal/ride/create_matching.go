package ride

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
	"math"
	"strings"
)

type MatchingCreator interface {
	CreateWithMatching(context.Context, contracts.Ride, contracts.MatchingRequest) (contracts.Ride, error)
}

// Creation commits both the ride and its durable dispatch request. A downstream
// outage cannot roll back a committed ride or silently discard its matching work.
func (s *Application) CreateRideWithMatching(ctx context.Context, identity, fare string, request contracts.MatchingRequest) (RideResponse, error) {
	p := request.PickupLocation
	if strings.TrimSpace(identity) == "" || strings.TrimSpace(fare) == "" || math.IsNaN(p.Lat) || math.IsNaN(p.Long) || p.Lat < -90 || p.Lat > 90 || p.Long < -180 || p.Long > 180 || math.IsNaN(request.RadiusMeters) || math.IsInf(request.RadiusMeters, 0) || request.RadiusMeters <= 0 {
		return RideResponse{}, fault.New("VALIDATION_ERROR", "Invalid pickup or matching radius")
	}
	repo, ok := s.Repo.(MatchingCreator)
	if !ok {
		return RideResponse{}, fault.Unavailable
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return RideResponse{}, fault.Unavailable
	}
	request.RideID = "ride_" + hex.EncodeToString(id[:])
	ctx, cancel := s.timeout(ctx)
	defer cancel()
	r, err := repo.CreateWithMatching(ctx, contracts.Ride{RideID: request.RideID, FareID: fare, RiderID: identity, Status: "REQUESTED"}, request)
	return RideResponse{RideID: r.RideID, Status: r.Status}, err
}

func (m MockService) CreateRideWithMatching(ctx context.Context, identity, fare string, _ contracts.MatchingRequest) (RideResponse, error) {
	return m.CreateRide(ctx, identity, fare)
}
