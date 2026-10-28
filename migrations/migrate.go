// Package migrations embeds the bootstrap so migrations work from any directory.
package migrations

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed 001_rides.sql
var bootstrap string

func Apply(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize explicit migration commands; application replicas never call Apply.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(3105001)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, bootstrap); err != nil {
		return err
	}
	return tx.Commit()
}
