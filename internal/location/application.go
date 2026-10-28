package location

import (
	"context"
	"errors"
	"fmt"
	"int3105-ride-share/internal/contracts"
	"math"
	"sort"
	"strings"
	"time"
)

var (
	ErrValidation            = errors.New("invalid location input")
	ErrDependencyUnavailable = errors.New("location dependency unavailable")
)

// DependencyError keeps infrastructure details out of public error messages,
// while allowing errors.Is/As to inspect the cause (including context errors).
type DependencyError struct{ Cause error }

func (e *DependencyError) Error() string { return ErrDependencyUnavailable.Error() }
func (e *DependencyError) Unwrap() error { return e.Cause }
func (e *DependencyError) Is(target error) bool {
	return target == ErrDependencyUnavailable
}

// DriverState is owned by Location. A nil Location means neither coordinate
// has been recorded; availability can be set before the first position.
type DriverState struct {
	DriverID  string
	Location  *contracts.Coordinates
	Available bool
	UpdatedAt time.Time
}

// Repository writes independent column groups atomically. Implementations must
// not replace a complete row from an earlier snapshot, even on initial insert.
type Repository interface {
	UpsertLocation(context.Context, string, contracts.Coordinates) error
	UpsertAvailability(context.Context, string, bool) error
	ListAvailable(context.Context) ([]DriverState, error)
}

// Application persists driver state and finds nearby available drivers.
type Application struct{ repository Repository }

func NewApplication(repository Repository) *Application {
	return &Application{repository: repository}
}

var _ Service = (*Application)(nil)

func (a *Application) UpdateLocation(ctx context.Context, driverID string, input contracts.Coordinates) (LocationResponse, error) {
	if strings.TrimSpace(driverID) == "" || !validCoordinates(input) {
		return LocationResponse{}, fmt.Errorf("%w: driver ID and finite coordinates in range are required", ErrValidation)
	}
	if err := a.repository.UpsertLocation(ctx, driverID, input); err != nil {
		return LocationResponse{}, err
	}
	return LocationResponse{DriverID: driverID, Lat: input.Lat, Long: input.Long}, nil
}

func (a *Application) UpdateAvailability(ctx context.Context, driverID string, available bool) (AvailabilityResponse, error) {
	if strings.TrimSpace(driverID) == "" {
		return AvailabilityResponse{}, fmt.Errorf("%w: driver ID is required", ErrValidation)
	}
	if err := a.repository.UpsertAvailability(ctx, driverID, available); err != nil {
		return AvailabilityResponse{}, err
	}
	return AvailabilityResponse{DriverID: driverID, Available: available}, nil
}

func (a *Application) NearbyDrivers(ctx context.Context, input contracts.NearbyInput) (NearbyResponse, error) {
	origin := contracts.Coordinates{Lat: input.Lat, Long: input.Long}
	if !validCoordinates(origin) || !finite(input.RadiusMeters) || input.RadiusMeters <= 0 {
		return NearbyResponse{}, fmt.Errorf("%w: finite coordinates in range and positive finite radius are required", ErrValidation)
	}
	states, err := a.repository.ListAvailable(ctx)
	if err != nil {
		return NearbyResponse{}, err
	}
	type candidate struct {
		state    DriverState
		distance float64
	}
	candidates := make([]candidate, 0, len(states))
	for _, state := range states {
		if !state.Available || state.Location == nil {
			continue
		}
		distance := haversine(origin, *state.Location)
		if distance <= input.RadiusMeters {
			candidates = append(candidates, candidate{state: state, distance: distance})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance == candidates[j].distance {
			return candidates[i].state.DriverID < candidates[j].state.DriverID
		}
		return candidates[i].distance < candidates[j].distance
	})
	result := NearbyResponse{Drivers: make([]Driver, 0, len(candidates))}
	for _, candidate := range candidates {
		result.Drivers = append(result.Drivers, Driver{
			DriverID:       candidate.state.DriverID,
			Location:       *candidate.state.Location,
			DistanceMeters: int(math.Round(candidate.distance)),
		})
	}
	return result, nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validCoordinates(value contracts.Coordinates) bool {
	return finite(value.Lat) && finite(value.Long) && value.Lat >= -90 && value.Lat <= 90 && value.Long >= -180 && value.Long <= 180
}

func haversine(from, to contracts.Coordinates) float64 {
	const radians = math.Pi / 180
	dLat := (to.Lat - from.Lat) * radians
	dLong := (to.Long - from.Long) * radians
	sinLat, sinLong := math.Sin(dLat/2), math.Sin(dLong/2)
	h := sinLat*sinLat + math.Cos(from.Lat*radians)*math.Cos(to.Lat*radians)*sinLong*sinLong
	h = math.Max(0, math.Min(1, h))
	return 6371000 * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}
