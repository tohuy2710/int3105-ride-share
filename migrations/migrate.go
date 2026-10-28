// Package migrations applies explicit, versioned schema changes. Application replicas never call Apply.
package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
)

//go:embed 001_rides.sql
var bootstrap string

//go:embed 002_matching_flow.sql
var matchingFlow string

func Apply(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(3105001)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS platform;
CREATE TABLE IF NOT EXISTS platform.schema_migrations (version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	for index, source := range []string{bootstrap, matchingFlow} {
		version := index + 1
		checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
		var existing string
		err = tx.QueryRowContext(ctx, "SELECT checksum FROM platform.schema_migrations WHERE version=$1", version).Scan(&existing)
		if err == nil {
			if existing != checksum {
				return fmt.Errorf("migration %d checksum mismatch", version)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err = tx.ExecContext(ctx, source); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO platform.schema_migrations(version,checksum) VALUES ($1,$2)", version, checksum); err != nil {
			return err
		}
	}
	return tx.Commit()
}
