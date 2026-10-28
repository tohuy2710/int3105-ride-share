// Package postgres implements the Notification data mapper. SQL does not escape this layer.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"int3105-ride-share/internal/notification"
	"int3105-ride-share/internal/platform/fault"
)

const columns = "id, ride_id, driver_id, type, status"

type Repository struct{ DB *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{DB: db} }

type scanner interface{ Scan(...any) error }

func scan(row scanner) (notification.Notification, error) {
	var n notification.Notification
	err := row.Scan(&n.ID, &n.RideID, &n.DriverID, &n.Type, &n.Status)
	return n, err
}
func dbError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return notification.ErrNotFound
	}
	if err != nil {
		return fault.Unavailable
	}
	return nil
}

// Enqueue relies on the unique constraint: a conflicting insert waits for the other writer
// to commit or roll back, then either inserts or yields to the committed row, which the
// follow-up SELECT (a fresh read committed snapshot) returns. Each statement autocommits,
// so a returned row is always durable.
func (r *Repository) Enqueue(ctx context.Context, in notification.Notification) (notification.Notification, error) {
	for attempt := 0; attempt < 3; attempt++ {
		out, err := scan(r.DB.QueryRowContext(ctx, "INSERT INTO notification.requests (id,ride_id,driver_id,type,status) VALUES ($1,$2,$3,$4,'QUEUED') ON CONFLICT ON CONSTRAINT requests_ride_driver_type_key DO NOTHING RETURNING "+columns, in.ID, in.RideID, in.DriverID, in.Type))
		if !errors.Is(err, sql.ErrNoRows) {
			return out, dbError(err)
		}
		out, err = scan(r.DB.QueryRowContext(ctx, "SELECT "+columns+" FROM notification.requests WHERE ride_id=$1 AND driver_id=$2 AND type=$3", in.RideID, in.DriverID, in.Type))
		if !errors.Is(err, sql.ErrNoRows) {
			return out, dbError(err)
		}
		// The conflicting row vanished between statements (external delete); try inserting again.
	}
	return notification.Notification{}, fault.Unavailable
}
func (r *Repository) Get(ctx context.Context, id string) (notification.Notification, error) {
	out, err := scan(r.DB.QueryRowContext(ctx, "SELECT "+columns+" FROM notification.requests WHERE id=$1", id))
	return out, dbError(err)
}
