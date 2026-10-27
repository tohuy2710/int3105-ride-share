// Package postgres implements the Ride data mapper. SQL does not escape this layer.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
)

const columns = "id, fare_id, rider_id, driver_id, status"

type Repository struct{ DB *sql.DB }
type scanner interface{ Scan(...any) error }

func scan(row scanner) (contracts.Ride, error) {
	var r contracts.Ride
	err := row.Scan(&r.RideID, &r.FareID, &r.RiderID, &r.DriverID, &r.Status)
	return r, err
}
func dbError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fault.NotFound
	}
	if err != nil {
		return fault.Unavailable
	}
	return nil
}
func (r *Repository) Create(ctx context.Context, in contracts.Ride) (contracts.Ride, error) {
	out, err := scan(r.DB.QueryRowContext(ctx, "INSERT INTO ride.rides (id,fare_id,rider_id,status) VALUES ($1,$2,$3,'REQUESTED') RETURNING "+columns, in.RideID, in.FareID, in.RiderID))
	return out, dbError(err)
}
func (r *Repository) Get(ctx context.Context, id string) (contracts.Ride, error) {
	out, err := scan(r.DB.QueryRowContext(ctx, "SELECT "+columns+" FROM ride.rides WHERE id=$1", id))
	return out, dbError(err)
}
func (r *Repository) Accept(ctx context.Context, id, driver string) (contracts.Ride, error) {
	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return contracts.Ride{}, fault.Unavailable
	}
	defer tx.Rollback()
	out, err := scan(tx.QueryRowContext(ctx, "UPDATE ride.rides SET driver_id=$2,status='ACCEPTED',updated_at=now() WHERE id=$1 AND status='REQUESTED' AND driver_id IS NULL RETURNING "+columns, id, driver))
	if errors.Is(err, sql.ErrNoRows) {
		out, err = scan(tx.QueryRowContext(ctx, "SELECT "+columns+" FROM ride.rides WHERE id=$1", id))
		if err != nil {
			return out, dbError(err)
		}
		idempotent := out.Status == "ACCEPTED" && out.DriverID != nil && *out.DriverID == driver
		if !idempotent {
			if out.DriverID != nil && *out.DriverID != driver {
				return out, fault.AlreadyAccepted
			}
			return out, fault.InvalidState
		}
	}
	if err != nil {
		return out, dbError(err)
	}
	if err = tx.Commit(); err != nil {
		return contracts.Ride{}, fault.Unavailable
	}
	return out, nil
}
func (r *Repository) Cancel(ctx context.Context, id, rider string) (contracts.Ride, error) {
	out, err := scan(r.DB.QueryRowContext(ctx, "UPDATE ride.rides SET status='CANCELLED',updated_at=now() WHERE id=$1 AND rider_id=$2 AND status='REQUESTED' RETURNING "+columns, id, rider))
	if !errors.Is(err, sql.ErrNoRows) {
		return out, dbError(err)
	}
	out, err = r.Get(ctx, id)
	if err != nil {
		return out, err
	}
	if out.RiderID != rider {
		return out, fault.Forbidden
	}
	if out.Status == "CANCELLED" {
		return out, nil
	}
	return out, fault.InvalidState
}
func (r *Repository) UpdateStatus(ctx context.Context, id, driver, status string) (contracts.Ride, error) {
	previous := map[string]string{"ARRIVED": "ACCEPTED", "IN_PROGRESS": "ARRIVED", "COMPLETED": "IN_PROGRESS"}[status]
	if previous == "" {
		return contracts.Ride{}, fault.InvalidState
	}
	out, err := scan(r.DB.QueryRowContext(ctx, "UPDATE ride.rides SET status=$3,updated_at=now() WHERE id=$1 AND driver_id=$2 AND status=$4 RETURNING "+columns, id, driver, status, previous))
	if !errors.Is(err, sql.ErrNoRows) {
		return out, dbError(err)
	}
	out, err = r.Get(ctx, id)
	if err != nil {
		return out, err
	}
	if out.DriverID == nil || *out.DriverID != driver {
		return out, fault.Forbidden
	}
	if out.Status == status {
		return out, nil
	}
	return out, fault.InvalidState
}
