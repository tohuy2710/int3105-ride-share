// Package postgres persists matching jobs only. It never calls remote ports or
// runs migrations; the integrator owns runtime wiring and schema deployment.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"int3105-ride-share/internal/matching"
)

type Repository struct{ db *sql.DB }

var _ matching.MatchingRepository = (*Repository)(nil)

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const columns = `ride_id, pickup_lat, pickup_long, radius_meters, status,
candidate_driver_ids, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (matching.Job, error) {
	var job matching.Job
	var candidates []byte
	err := row.Scan(&job.Input.RideID, &job.Input.PickupLocation.Lat, &job.Input.PickupLocation.Long,
		&job.Input.RadiusMeters, &job.Status, &candidates, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return matching.Job{}, err
	}
	if err := json.Unmarshal(candidates, &job.CandidateDriverIDs); err != nil {
		return matching.Job{}, err
	}
	return job, nil
}

func dbError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return matching.ErrMatchingNotFound
	}
	if ctx.Err() != nil {
		return errors.Join(matching.ErrUnavailable, ctx.Err())
	}
	return matching.ErrUnavailable
}

func (r *Repository) Get(ctx context.Context, rideID string) (matching.Job, error) {
	job, err := scan(r.db.QueryRowContext(ctx, "SELECT "+columns+" FROM matching.requests WHERE ride_id=$1", rideID))
	return job, dbError(ctx, err)
}

func (r *Repository) CreateOrGet(ctx context.Context, proposed matching.Job) (matching.Job, error) {
	if err := proposed.Input.Validate(); err != nil {
		return matching.Job{}, err
	}
	if (proposed.Status != matching.Matching || len(proposed.CandidateDriverIDs) == 0) &&
		(proposed.Status != matching.NoDrivers || len(proposed.CandidateDriverIDs) != 0) {
		return matching.Job{}, matching.ErrValidation
	}
	seen := make(map[string]bool, len(proposed.CandidateDriverIDs))
	for _, id := range proposed.CandidateDriverIDs {
		if strings.TrimSpace(id) == "" || seen[id] {
			return matching.Job{}, matching.ErrValidation
		}
		seen[id] = true
	}
	ids := proposed.CandidateDriverIDs
	if ids == nil {
		ids = []string{}
	}
	candidates, err := json.Marshal(ids)
	if err != nil {
		return matching.Job{}, matching.ErrValidation
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return matching.Job{}, dbError(ctx, err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO matching.requests
(ride_id, pickup_lat, pickup_long, radius_meters, status, candidate_driver_ids)
VALUES ($1,$2,$3,$4,$5,$6::jsonb) ON CONFLICT (ride_id) DO NOTHING`,
		proposed.Input.RideID, proposed.Input.PickupLocation.Lat, proposed.Input.PickupLocation.Long,
		proposed.Input.RadiusMeters, proposed.Status, string(candidates))
	if err != nil {
		return matching.Job{}, dbError(ctx, err)
	}
	// A separate statement at READ COMMITTED sees the winning row even when
	// the INSERT waited for another transaction. A single CTE would not.
	job, err := scan(tx.QueryRowContext(ctx, "SELECT "+columns+" FROM matching.requests WHERE ride_id=$1", proposed.Input.RideID))
	if err != nil {
		return matching.Job{}, dbError(ctx, err)
	}
	if job.Input != proposed.Input {
		return matching.Job{}, matching.ErrRequestConflict
	}
	if err := tx.Commit(); err != nil {
		return matching.Job{}, dbError(ctx, err)
	}
	return job, nil
}

func (r *Repository) AdvanceStatus(ctx context.Context, rideID string, status matching.Status) (matching.Job, error) {
	if status != matching.Matched && status != matching.Cancelled {
		return matching.Job{}, matching.ErrValidation
	}
	job, err := scan(r.db.QueryRowContext(ctx, `UPDATE matching.requests
SET status=$2, updated_at=now()
WHERE ride_id=$1 AND status IN ('MATCHING','NO_DRIVERS') RETURNING `+columns, rideID, status))
	if errors.Is(err, sql.ErrNoRows) {
		return r.Get(ctx, rideID)
	}
	return job, dbError(ctx, err)
}
