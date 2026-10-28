// Package postgres persists Location state only. Migrations are applied
// explicitly by the integrator or integration-test setup, never by NewRepository.
package postgres

import (
	"context"
	"database/sql"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/location"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

var _ location.Repository = (*Repository)(nil)

func (r *Repository) UpsertLocation(ctx context.Context, driverID string, position contracts.Coordinates) error {
	return r.write(ctx, `
		INSERT INTO location.drivers (driver_id, lat, long, updated_at)
		VALUES ($1, $2, $3, clock_timestamp())
		ON CONFLICT (driver_id) DO UPDATE
		SET lat = EXCLUDED.lat, long = EXCLUDED.long, updated_at = clock_timestamp()`,
		driverID, position.Lat, position.Long)
}

func (r *Repository) UpsertAvailability(ctx context.Context, driverID string, available bool) error {
	return r.write(ctx, `
		INSERT INTO location.drivers (driver_id, available, updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (driver_id) DO UPDATE
		SET available = EXCLUDED.available, updated_at = clock_timestamp()`, driverID, available)
}

// An explicit transaction prevents a statement canceled while waiting for a
// row lock from being committed later if server-side cancellation races with
// the lock release. Success is returned only after Commit succeeds. As with
// any database client, a connection failure during Commit can be ambiguous.
func (r *Repository) write(ctx context.Context, query string, args ...any) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return dependencyError(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return dependencyError(err)
	}
	return dependencyError(tx.Commit())
}

func (r *Repository) ListAvailable(ctx context.Context) ([]location.DriverState, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT driver_id, lat, long, available, updated_at
		FROM location.drivers
		WHERE available = true AND lat IS NOT NULL AND long IS NOT NULL`)
	if err != nil {
		return nil, dependencyError(err)
	}
	defer rows.Close()
	states := make([]location.DriverState, 0)
	for rows.Next() {
		var state location.DriverState
		var position contracts.Coordinates
		if err := rows.Scan(&state.DriverID, &position.Lat, &position.Long, &state.Available, &state.UpdatedAt); err != nil {
			return nil, dependencyError(err)
		}
		state.Location = &position
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, dependencyError(err)
	}
	return states, nil
}

func dependencyError(err error) error {
	if err == nil {
		return nil
	}
	return &location.DependencyError{Cause: err}
}
