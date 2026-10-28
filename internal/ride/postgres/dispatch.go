package postgres

import (
	"context"
	"int3105-ride-share/internal/contracts"
)

func (r *Repository) CreateWithMatching(ctx context.Context, in contracts.Ride, request contracts.MatchingRequest) (contracts.Ride, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return contracts.Ride{}, dbError(err)
	}
	defer tx.Rollback()
	out, err := scan(tx.QueryRowContext(ctx, "INSERT INTO ride.rides (id,fare_id,rider_id,status) VALUES ($1,$2,$3,'REQUESTED') RETURNING "+columns, in.RideID, in.FareID, in.RiderID))
	if err != nil {
		return contracts.Ride{}, dbError(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ride.matching_dispatch (ride_id,pickup_lat,pickup_long,radius_meters) VALUES ($1,$2,$3,$4)`, in.RideID, request.PickupLocation.Lat, request.PickupLocation.Long, request.RadiusMeters)
	if err != nil {
		return contracts.Ride{}, dbError(err)
	}
	if err = tx.Commit(); err != nil {
		return contracts.Ride{}, dbError(err)
	}
	return out, nil
}

// A short SQL lease supports multiple Ride replicas without holding a database
// transaction over an HTTP request. Expired leases recover after process death.
func (r *Repository) ClaimMatching(ctx context.Context) (contracts.MatchingRequest, error) {
	var in contracts.MatchingRequest
	err := r.DB.QueryRowContext(ctx, `UPDATE ride.matching_dispatch SET next_attempt_at=now()+interval '15 seconds',attempts=attempts+1
WHERE ride_id=(SELECT ride_id FROM ride.matching_dispatch WHERE NOT dispatched AND next_attempt_at<=now() ORDER BY next_attempt_at,ride_id FOR UPDATE SKIP LOCKED LIMIT 1)
RETURNING ride_id,pickup_lat,pickup_long,radius_meters`).Scan(&in.RideID, &in.PickupLocation.Lat, &in.PickupLocation.Long, &in.RadiusMeters)
	return in, err
}
func (r *Repository) CompleteMatching(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, "UPDATE ride.matching_dispatch SET dispatched=true WHERE ride_id=$1", id)
	return err
}
