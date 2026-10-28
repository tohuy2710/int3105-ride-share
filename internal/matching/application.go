package matching

import (
	"context"
	"errors"
	"strings"

	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
)

// Application implements persistent matching using transport-independent ports.
type Application struct {
	repository MatchingRepository
	rides      RideReader
	drivers    DriverFinder
	notifier   NotificationRequester
}

func NewApplication(repository MatchingRepository, rides RideReader, drivers DriverFinder, notifier NotificationRequester) *Application {
	return &Application{repository: repository, rides: rides, drivers: drivers, notifier: notifier}
}

func (a *Application) Request(ctx context.Context, in Input) (Result, error) {
	if err := in.Validate(); err != nil {
		return Result{}, err
	}
	if ctx.Err() != nil {
		return Result{}, dependencyError(ctx, ctx.Err())
	}
	job, err := a.repository.Get(ctx, in.RideID)
	exists := err == nil
	if err != nil && !errors.Is(err, ErrMatchingNotFound) {
		return Result{}, dependencyError(ctx, err)
	}
	if exists && job.Input != in {
		return Result{}, ErrRequestConflict
	}
	ride, err := a.readRide(ctx, in.RideID)
	if err != nil {
		return Result{}, err
	}
	if !exists {
		if ride.Status != "REQUESTED" {
			return Result{}, ErrRideNotMatchable
		}
		nearby, err := a.drivers.Nearby(ctx, contracts.NearbyInput{
			Lat: in.PickupLocation.Lat, Long: in.PickupLocation.Long, RadiusMeters: in.RadiusMeters,
		})
		if err != nil {
			return Result{}, dependencyError(ctx, err)
		}
		candidates := make([]string, 0, len(nearby.Drivers))
		seen := make(map[string]bool, len(nearby.Drivers))
		for _, driver := range nearby.Drivers {
			if strings.TrimSpace(driver.DriverID) == "" {
				return Result{}, ErrUnavailable
			}
			if !seen[driver.DriverID] {
				seen[driver.DriverID] = true
				candidates = append(candidates, driver.DriverID)
			}
		}
		status := Matching
		if len(candidates) == 0 {
			status = NoDrivers
		}
		job, err = a.repository.CreateOrGet(ctx, Job{Input: in, Status: status, CandidateDriverIDs: candidates})
		if errors.Is(err, ErrRequestConflict) {
			return Result{}, ErrRequestConflict
		}
		if err != nil {
			return Result{}, dependencyError(ctx, err)
		}
		if job.Input != in {
			return Result{}, ErrRequestConflict
		}
	}
	job, err = a.refresh(ctx, job, ride.Status)
	if err != nil {
		return Result{}, err
	}
	if job.Status != Matching {
		return result(job), nil
	}
	for _, driverID := range job.CandidateDriverIDs {
		request := contracts.NotificationRequest{RideID: in.RideID, DriverID: driverID, Type: "RIDE_REQUEST"}
		ack, err := a.notifier.RequestRide(ctx, request)
		if err != nil {
			return Result{}, dependencyError(ctx, err)
		}
		if strings.TrimSpace(ack.NotificationID) == "" || ack.RideID != request.RideID ||
			ack.DriverID != request.DriverID || ack.Type != request.Type || ack.Status != "QUEUED" {
			return Result{}, ErrUnavailable
		}
	}
	// Never return an obsolete MATCHING snapshot over a concurrently saved
	// terminal state. No status write occurs after dispatch.
	job, err = a.repository.Get(ctx, in.RideID)
	if err != nil {
		return Result{}, dependencyError(ctx, err)
	}
	return result(job), nil
}

func (a *Application) Get(ctx context.Context, rideID string) (Result, error) {
	if strings.TrimSpace(rideID) == "" {
		return Result{}, ErrValidation
	}
	if ctx.Err() != nil {
		return Result{}, dependencyError(ctx, ctx.Err())
	}
	job, err := a.repository.Get(ctx, rideID)
	if errors.Is(err, ErrMatchingNotFound) {
		return Result{}, ErrMatchingNotFound
	}
	if err != nil {
		return Result{}, dependencyError(ctx, err)
	}
	ride, err := a.readRide(ctx, rideID)
	if err != nil {
		return Result{}, err
	}
	job, err = a.refresh(ctx, job, ride.Status)
	if err != nil {
		return Result{}, err
	}
	return result(job), nil
}

func (a *Application) readRide(ctx context.Context, rideID string) (contracts.Ride, error) {
	ride, err := a.rides.GetRide(ctx, rideID)
	if errors.Is(err, ErrRideNotFound) || errors.Is(err, fault.NotFound) {
		return contracts.Ride{}, ErrRideNotFound
	}
	if err != nil {
		return contracts.Ride{}, dependencyError(ctx, err)
	}
	if ride.RideID != rideID {
		return contracts.Ride{}, ErrUnavailable
	}
	switch ride.Status {
	case "REQUESTED", "ACCEPTED", "ARRIVED", "IN_PROGRESS", "COMPLETED", "CANCELLED":
		return ride, nil
	default:
		return contracts.Ride{}, ErrUnavailable
	}
}

func (a *Application) refresh(ctx context.Context, job Job, rideStatus string) (Job, error) {
	var status Status
	switch rideStatus {
	case "ACCEPTED", "ARRIVED", "IN_PROGRESS", "COMPLETED":
		status = Matched
	case "CANCELLED":
		status = Cancelled
	}
	var err error
	if status != "" {
		job, err = a.repository.AdvanceStatus(ctx, job.Input.RideID, status)
	} else {
		job, err = a.repository.Get(ctx, job.Input.RideID)
	}
	if err != nil {
		return Job{}, dependencyError(ctx, err)
	}
	return job, nil
}

func result(job Job) Result { return Result{RideID: job.Input.RideID, Status: job.Status} }
