package matching

import (
	"context"
	"errors"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
)

func transportError(err error) error {
	switch {
	case errors.Is(err, ErrValidation):
		return fault.New("VALIDATION_ERROR", "Invalid matching input")
	case errors.Is(err, ErrMatchingNotFound):
		return fault.New("MATCHING_NOT_FOUND", "Matching job not found")
	case errors.Is(err, ErrRideNotFound), errors.Is(err, fault.NotFound):
		return fault.NotFound
	case errors.Is(err, ErrRideNotMatchable):
		return fault.InvalidState
	case errors.Is(err, ErrRequestConflict):
		return fault.New("MATCHING_REQUEST_CONFLICT", "Matching input conflicts with existing job")
	case errors.Is(err, ErrUnavailable):
		return fault.Unavailable
	default:
		return err
	}
}

func (a *Application) RequestMatching(ctx context.Context, in contracts.MatchingRequest) (Response, error) {
	result, err := a.Request(ctx, Input{RideID: in.RideID, PickupLocation: in.PickupLocation, RadiusMeters: in.RadiusMeters})
	return Response{RideID: result.RideID, Status: string(result.Status)}, transportError(err)
}
func (a *Application) GetMatching(ctx context.Context, id string) (Response, error) {
	result, err := a.Get(ctx, id)
	return Response{RideID: result.RideID, Status: string(result.Status)}, transportError(err)
}
