package matching

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"int3105-ride-share/internal/contracts"
)

type Status string

const (
	Matching  Status = "MATCHING"
	NoDrivers Status = "NO_DRIVERS"
	Matched   Status = "MATCHED"
	Cancelled Status = "CANCELLED"
)

var (
	ErrValidation       = errors.New("invalid matching input")
	ErrMatchingNotFound = errors.New("matching job not found")
	ErrRideNotFound     = errors.New("ride not found")
	ErrRideNotMatchable = errors.New("ride is not requested")
	ErrRequestConflict  = errors.New("matching input conflicts with existing job")
	ErrUnavailable      = errors.New("matching dependency unavailable")
)

// Input contains explicit pickup coordinates and a boundary-normalized radius.
// A zero coordinate is valid; a zero radius is not a default at this layer.
type Input struct {
	RideID         string
	PickupLocation contracts.Coordinates
	RadiusMeters   float64
}

func (in Input) Validate() error {
	p := in.PickupLocation
	if strings.TrimSpace(in.RideID) == "" || !finite(p.Lat) || !finite(p.Long) ||
		p.Lat < -90 || p.Lat > 90 || p.Long < -180 || p.Long > 180 ||
		!finite(in.RadiusMeters) || in.RadiusMeters <= 0 {
		return ErrValidation
	}
	return nil
}

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

type Job struct {
	Input              Input
	Status             Status
	CandidateDriverIDs []string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Result struct {
	RideID string `json:"rideId"`
	Status Status `json:"status"`
}

// MatchingRepository never holds a transaction while the application calls
// remote ports. CreateOrGet returns the committed winner, not a tentative job.
type MatchingRepository interface {
	Get(ctx context.Context, rideID string) (Job, error)
	CreateOrGet(ctx context.Context, proposed Job) (Job, error)
	// AdvanceStatus accepts terminal statuses only. Existing terminal states
	// are immutable, including when an older snapshot finishes later.
	AdvanceStatus(ctx context.Context, rideID string, status Status) (Job, error)
}

// RideReader must map a confirmed 404 to ErrRideNotFound. It has no write port.
type RideReader interface {
	GetRide(ctx context.Context, rideID string) (contracts.Ride, error)
}

type DriverFinder interface {
	Nearby(ctx context.Context, input contracts.NearbyInput) (contracts.NearbyResponse, error)
}

// NotificationRequester must durably acknowledge and deduplicate each
// (rideID, driverID, RIDE_REQUEST) tuple. A retry can repeat earlier requests.
type NotificationRequester interface {
	RequestRide(ctx context.Context, input contracts.NotificationRequest) (contracts.NotificationResponse, error)
}

func dependencyError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return errors.Join(ErrUnavailable, ctx.Err())
	}
	if errors.Is(err, context.Canceled) {
		return errors.Join(ErrUnavailable, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(ErrUnavailable, context.DeadlineExceeded)
	}
	return ErrUnavailable
}
