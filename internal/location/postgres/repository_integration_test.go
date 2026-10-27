//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/location"
	"int3105-ride-share/internal/location/postgres"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/001_init.sql
var migration string

func TestPostgresApplication(t *testing.T) {
	dsn := os.Getenv("TEST_LOCATION_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_LOCATION_DATABASE_URL is required with -tags=integration; use a dedicated Location test database")
	}
	openDB := func() *sql.DB {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal("cannot open Location test database (DSN omitted)")
		}
		db.SetMaxOpenConns(8)
		t.Cleanup(func() { db.Close() })
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			t.Fatal("cannot connect to Location test PostgreSQL (DSN omitted)")
		}
		return db
	}
	db := openDB()
	var databaseName string
	if err := db.QueryRow("SELECT current_database()").Scan(&databaseName); err != nil {
		t.Fatal("cannot inspect test database")
	}
	if !strings.HasSuffix(databaseName, "_test") {
		t.Fatal("refusing migration: use a dedicated database whose name ends in _test, for example location_test")
	}
	// Explicit test setup only; production constructors do not execute SQL.
	if _, err := db.Exec(migration); err != nil {
		t.Fatal("cannot explicitly apply Location test migration")
	}
	secondDB := openDB()
	app := location.NewApplication(postgres.NewRepository(db))
	secondApp := location.NewApplication(postgres.NewRepository(secondDB))
	ctx := context.Background()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "location-test-" + hex.EncodeToString(random[:]) + "-"
	var ownedIDs []string
	id := func(name string) string {
		value := prefix + name
		ownedIDs = append(ownedIDs, value)
		return value
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, driverID := range ownedIDs {
			if _, err := db.ExecContext(cleanupCtx, "DELETE FROM location.drivers WHERE driver_id = $1", driverID); err != nil {
				t.Error("could not clean an owned Location test row")
			}
		}
	})

	read := func(t *testing.T, driverID string) location.DriverState {
		t.Helper()
		var state location.DriverState
		var lat, long sql.NullFloat64
		if err := secondDB.QueryRowContext(ctx, `SELECT driver_id, lat, long, available, updated_at FROM location.drivers WHERE driver_id=$1`, driverID).
			Scan(&state.DriverID, &lat, &long, &state.Available, &state.UpdatedAt); err != nil {
			t.Fatal(err)
		}
		if lat.Valid != long.Valid || state.UpdatedAt.IsZero() {
			t.Fatalf("invalid persisted state: %+v lat=%+v long=%+v", state, lat, long)
		}
		if lat.Valid {
			state.Location = &contracts.Coordinates{Lat: lat.Float64, Long: long.Float64}
		}
		return state
	}
	nearby := func(t *testing.T, application *location.Application) map[string]location.Driver {
		t.Helper()
		result, err := application.NearbyDrivers(ctx, contracts.NearbyInput{RadiusMeters: math.Pi*6371000 + 1})
		if err != nil {
			t.Fatal(err)
		}
		found := make(map[string]location.Driver)
		for _, driver := range result.Drivers {
			if strings.HasPrefix(driver.DriverID, prefix) {
				found[driver.DriverID] = driver
			}
		}
		return found
	}

	t.Run("latest_location_defaults_and_independent_updates", func(t *testing.T) {
		driverID := id("position-first")
		if _, err := app.UpdateLocation(ctx, driverID, contracts.Coordinates{}); err != nil {
			t.Fatal(err)
		}
		first := read(t, driverID)
		if first.Available || first.Location == nil || *first.Location != (contracts.Coordinates{}) {
			t.Fatalf("initial position must default unavailable: %+v", first)
		}
		if _, exists := nearby(t, secondApp)[driverID]; exists {
			t.Fatal("unavailable driver appeared nearby")
		}
		if _, err := secondApp.UpdateAvailability(ctx, driverID, true); err != nil {
			t.Fatal(err)
		}
		for _, position := range []contracts.Coordinates{{Lat: -90, Long: -180}, {Lat: 90, Long: 180}, {Lat: 1, Long: 2}} {
			if _, err := app.UpdateLocation(ctx, driverID, position); err != nil {
				t.Fatal(err)
			}
			state := read(t, driverID)
			if !state.Available || state.Location == nil || *state.Location != position || state.UpdatedAt.Before(first.UpdatedAt) {
				t.Fatalf("position update lost availability or latest position: %+v", state)
			}
		}
		if _, err := secondApp.UpdateAvailability(ctx, driverID, false); err != nil {
			t.Fatal(err)
		}
		state := read(t, driverID)
		if state.Available || state.Location == nil || *state.Location != (contracts.Coordinates{Lat: 1, Long: 2}) {
			t.Fatalf("availability update lost position: %+v", state)
		}
	})

	t.Run("availability_first_and_recreated_repository", func(t *testing.T) {
		driverID := id("availability-first")
		if _, err := app.UpdateAvailability(ctx, driverID, true); err != nil {
			t.Fatal(err)
		}
		state := read(t, driverID)
		if !state.Available || state.Location != nil {
			t.Fatalf("availability-first state: %+v", state)
		}
		if _, exists := nearby(t, secondApp)[driverID]; exists {
			t.Fatal("driver without position appeared nearby")
		}
		if _, err := secondApp.UpdateLocation(ctx, driverID, contracts.Coordinates{}); err != nil {
			t.Fatal(err)
		}
		// A new pool and repository must see the committed state, not local RAM.
		recreated := location.NewApplication(postgres.NewRepository(openDB()))
		driver, exists := nearby(t, recreated)[driverID]
		if !exists || driver.Location != (contracts.Coordinates{}) || driver.DistanceMeters != 0 {
			t.Fatalf("recreated application lost persisted state: %+v", driver)
		}
	})

	t.Run("concurrent_insert_and_existing_row_column_updates", func(t *testing.T) {
		for attempt := 0; attempt < 24; attempt++ {
			var suffix [8]byte
			if _, err := rand.Read(suffix[:]); err != nil {
				t.Fatal(err)
			}
			driverID := id("concurrent-" + hex.EncodeToString(suffix[:]))
			for _, available := range []bool{true, false, true} {
				position := contracts.Coordinates{Lat: float64(attempt), Long: float64(attempt + 1)}
				if !available {
					position.Long = -position.Long
				}
				start := make(chan struct{})
				errs := make(chan error, 2)
				go func() {
					<-start
					_, err := app.UpdateLocation(ctx, driverID, position)
					errs <- err
				}()
				go func() {
					<-start
					_, err := secondApp.UpdateAvailability(ctx, driverID, available)
					errs <- err
				}()
				close(start)
				for i := 0; i < 2; i++ {
					if err := <-errs; err != nil {
						t.Fatal(err)
					}
				}
				state := read(t, driverID)
				if state.Location == nil || *state.Location != position || state.Available != available {
					t.Fatalf("concurrent update lost a field: %+v want position=%+v available=%v", state, position, available)
				}
			}
		}
	})

	t.Run("database_constraints", func(t *testing.T) {
		reject := func(query string, driverID string) {
			t.Helper()
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.ExecContext(ctx, query, driverID); err == nil {
				t.Fatal("database accepted an invalid Location state")
			}
		}
		for _, values := range []string{
			"NULL, 0", "0, NULL", "91, 0", "0, -181",
			"'NaN'::float8, 0", "0, 'NaN'::float8",
			"'Infinity'::float8, 0", "0, '-Infinity'::float8",
		} {
			// Only fixed test literals are concatenated. Driver IDs remain parameters.
			driverID := id("constraint-" + values)
			reject(`INSERT INTO location.drivers (driver_id, lat, long, updated_at) VALUES ($1, `+values+`, clock_timestamp())`, driverID)
		}
		for _, blank := range []string{"", " \t\n"} {
			reject(`INSERT INTO location.drivers (driver_id, updated_at) VALUES ($1, clock_timestamp())`, blank)
		}
	})

	t.Run("database_errors_and_context", func(t *testing.T) {
		driverID := id("timeout")
		if _, err := app.UpdateAvailability(ctx, driverID, false); err != nil {
			t.Fatal(err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, `SELECT driver_id FROM location.drivers WHERE driver_id=$1 FOR UPDATE`, driverID); err != nil {
			t.Fatal(err)
		}
		timeout, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
		defer cancel()
		_, err = secondApp.UpdateLocation(timeout, driverID, contracts.Coordinates{Lat: 1})
		if !errors.Is(err, location.ErrDependencyUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("blocked SQL did not return dependency/timeout: %v", err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if state := read(t, driverID); state.Location != nil || state.Available {
			t.Fatalf("timed-out update was acknowledged or changed state: %+v", state)
		}
		canceled, stop := context.WithCancel(ctx)
		stop()
		for _, operation := range []func() error{
			func() error { _, err := app.UpdateLocation(canceled, driverID, contracts.Coordinates{}); return err },
			func() error { _, err := app.UpdateAvailability(canceled, driverID, true); return err },
			func() error {
				_, err := app.NearbyDrivers(canceled, contracts.NearbyInput{RadiusMeters: 1})
				return err
			},
		} {
			if err := operation(); !errors.Is(err, location.ErrDependencyUnavailable) || !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled context: %v", err)
			}
		}
		closedDB := openDB()
		closedDB.Close()
		closed := location.NewApplication(postgres.NewRepository(closedDB))
		for _, operation := range []func() error{
			func() error { _, err := closed.UpdateLocation(ctx, driverID, contracts.Coordinates{}); return err },
			func() error { _, err := closed.UpdateAvailability(ctx, driverID, true); return err },
			func() error { _, err := closed.NearbyDrivers(ctx, contracts.NearbyInput{RadiusMeters: 1}); return err },
		} {
			if err := operation(); !errors.Is(err, location.ErrDependencyUnavailable) || err.Error() != location.ErrDependencyUnavailable.Error() {
				t.Fatalf("database failure was not safely classified: %v", err)
			}
		}
	})
}
