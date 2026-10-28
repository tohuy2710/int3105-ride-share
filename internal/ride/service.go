package ride

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
	"time"
)

type FareInput struct{ PickupLocation, Destination contracts.Coordinates }
type Service interface {
	EstimateFare(context.Context, string, FareInput) (FareResponse, error)
	CreateRide(context.Context, string, string) (RideResponse, error)
	GetRide(context.Context, string, contracts.User) (DetailsResponse, error)
	InternalRide(context.Context, string) (DetailsResponse, error)
	RespondToRide(context.Context, string, string, string) (ActionResponse, error)
	UpdateStatus(context.Context, string, string, string) (RideResponse, error)
	CancelRide(context.Context, string, string) (RideResponse, error)
}
type Repository interface {
	Create(context.Context, contracts.Ride) (contracts.Ride, error)
	Get(context.Context, string) (contracts.Ride, error)
	Accept(context.Context, string, string) (contracts.Ride, error)
	Cancel(context.Context, string, string) (contracts.Ride, error)
	UpdateStatus(context.Context, string, string, string) (contracts.Ride, error)
}
type Gate interface {
	Acquire(context.Context, string) (release func(), acquired bool, err error)
}
type Counters interface {
	SQLAttempt()
	RedisRejected()
	RedisFallback()
}
type Application struct {
	Repo      Repository
	Gate      Gate // nil in sql mode
	Metrics   Counters
	DBTimeout time.Duration
}

func (s *Application) timeout(ctx context.Context) (context.Context, context.CancelFunc) {
	d := s.DBTimeout
	if d <= 0 {
		d = 2 * time.Second
	}
	return context.WithTimeout(ctx, d)
}
func (*Application) EstimateFare(context.Context, string, FareInput) (FareResponse, error) {
	return FareResponse{"fare_123", 85000, 12}, nil
}
func (s *Application) CreateRide(ctx context.Context, identity, fare string) (RideResponse, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return RideResponse{}, err
	}
	ctx, cancel := s.timeout(ctx)
	defer cancel()
	r, err := s.Repo.Create(ctx, contracts.Ride{RideID: "ride_" + hex.EncodeToString(id[:]), FareID: fare, RiderID: identity, Status: "REQUESTED"})
	return RideResponse{r.RideID, r.Status}, err
}
func (s *Application) InternalRide(ctx context.Context, id string) (DetailsResponse, error) {
	ctx, cancel := s.timeout(ctx)
	defer cancel()
	return s.Repo.Get(ctx, id)
}
func (s *Application) GetRide(ctx context.Context, id string, user contracts.User) (DetailsResponse, error) {
	r, err := s.InternalRide(ctx, id)
	if err != nil {
		return r, err
	}
	if (user.Role == "RIDER" && r.RiderID == user.UserID) || (user.Role == "DRIVER" && r.DriverID != nil && *r.DriverID == user.UserID) {
		return r, nil
	}
	return DetailsResponse{}, fault.Forbidden
}
func (s *Application) RespondToRide(ctx context.Context, id, driver, action string) (ActionResponse, error) {
	if action != "ACCEPT" && action != "DENY" {
		return ActionResponse{}, fault.InvalidState
	}
	if action == "DENY" {
		r, err := s.InternalRide(ctx, id)
		return ActionResponse{r.RideID, action, r.DriverID, r.Status}, err
	}
	if s.Gate != nil {
		release, ok, err := s.Gate.Acquire(ctx, id)
		if err != nil {
			if s.Metrics != nil {
				s.Metrics.RedisFallback()
			}
		} else if !ok {
			if s.Metrics != nil {
				s.Metrics.RedisRejected()
			}
			return ActionResponse{}, fault.InProgress
		} else {
			defer release()
		}
	}
	ctx, cancel := s.timeout(ctx)
	defer cancel()
	if s.Metrics != nil {
		s.Metrics.SQLAttempt()
	}
	r, err := s.Repo.Accept(ctx, id, driver)
	return ActionResponse{r.RideID, action, r.DriverID, r.Status}, err
}
func (s *Application) CancelRide(ctx context.Context, id, rider string) (RideResponse, error) {
	ctx, cancel := s.timeout(ctx)
	defer cancel()
	r, err := s.Repo.Cancel(ctx, id, rider)
	return RideResponse{r.RideID, r.Status}, err
}
func (s *Application) UpdateStatus(ctx context.Context, id, driver, status string) (RideResponse, error) {
	ctx, cancel := s.timeout(ctx)
	defer cancel()
	r, err := s.Repo.UpdateStatus(ctx, id, driver, status)
	return RideResponse{r.RideID, r.Status}, err
}

// MockService only exists for isolated contract tests; production uses Application.
type MockService struct{}

func (MockService) EstimateFare(context.Context, string, FareInput) (FareResponse, error) {
	return FareResponse{"fare_123", 85000, 12}, nil
}
func (MockService) CreateRide(context.Context, string, string) (RideResponse, error) {
	return RideResponse{"ride_123", "REQUESTED"}, nil
}
func (MockService) GetRide(_ context.Context, id string, user contracts.User) (DetailsResponse, error) {
	return contracts.Ride{RideID: id, FareID: "fare_123", RiderID: user.UserID, Status: "REQUESTED"}, nil
}
func (MockService) InternalRide(_ context.Context, id string) (DetailsResponse, error) {
	return contracts.Ride{RideID: id, FareID: "fare_123", RiderID: "rider_123", Status: "REQUESTED"}, nil
}
func (MockService) RespondToRide(_ context.Context, id, driver, action string) (ActionResponse, error) {
	status := "REQUESTED"
	var owner *string
	if action == "ACCEPT" {
		status = "ACCEPTED"
		owner = &driver
	}
	return ActionResponse{id, action, owner, status}, nil
}
func (MockService) UpdateStatus(_ context.Context, id, driver, status string) (RideResponse, error) {
	return RideResponse{id, status}, nil
}
func (MockService) CancelRide(_ context.Context, id, rider string) (RideResponse, error) {
	return RideResponse{id, "CANCELLED"}, nil
}
