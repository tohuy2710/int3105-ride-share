//go:build integration

package matching_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/matching"
	"int3105-ride-share/internal/matching/postgres"
)

var integrationDB *sql.DB
var integrationDSN string

// Migration is an explicit test setup step, never application startup logic.
// The tag intentionally fails without a real, explicitly selected test DB.
func TestMain(m *testing.M) {
	integrationDSN = os.Getenv("TEST_MATCHING_DATABASE_URL")
	if integrationDSN == "" {
		fmt.Fprintln(os.Stderr, "integration requires TEST_MATCHING_DATABASE_URL for a dedicated PostgreSQL test database")
		os.Exit(1)
	}
	var err error
	integrationDB, err = sql.Open("pgx", integrationDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open matching test database")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err = prepareTestSchema(ctx, integrationDB)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "matching PostgreSQL test setup failed:", err)
		integrationDB.Close()
		os.Exit(1)
	}
	code := m.Run()
	integrationDB.Close()
	os.Exit(code)
}

func prepareTestSchema(ctx context.Context, db *sql.DB) error {
	migration, err := os.ReadFile(filepath.Join("postgres", "migrations", "001_init.sql"))
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("cannot connect to PostgreSQL test database")
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(310502001)"); err != nil {
		return err
	}
	var present sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT to_regclass('matching.requests')::text").Scan(&present); err != nil {
		return err
	}
	if !present.Valid {
		if _, err := tx.ExecContext(ctx, string(migration)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func testPool(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", integrationDSN)
	if err != nil {
		t.Fatal("cannot open test pool")
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })
	return db
}

func testID(t *testing.T) string {
	t.Helper()
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		t.Fatal(err)
	}
	id := "matching_test_" + hex.EncodeToString(bytes[:])
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := integrationDB.ExecContext(ctx, "DELETE FROM matching.requests WHERE ride_id=$1", id); err != nil {
			t.Error("cannot clean up owned matching test row:", err)
		}
	})
	return id
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestPostgresPartialRetryAfterApplicationRecreation(t *testing.T) {
	ctx := testContext(t)
	in := input(testID(t))
	db1, db2 := testPool(t), testPool(t)
	// Calling the same pool inside a remote fake detects an open transaction:
	// a leaked lease would exhaust its only connection and time out.
	db1.SetMaxOpenConns(1)
	repo1, repo2 := postgres.NewRepository(db1), postgres.NewRepository(db2)
	var sent []string
	fail := true
	notifier := notifyFunc(func(ctx context.Context, req contracts.NotificationRequest) (contracts.NotificationResponse, error) {
		job, err := repo1.Get(ctx, in.RideID)
		if err != nil || !reflect.DeepEqual(job.CandidateDriverIDs, []string{"a", "b"}) {
			return contracts.NotificationResponse{}, errors.New("selection is not committed before notification")
		}
		sent = append(sent, req.DriverID)
		if fail && req.DriverID == "b" {
			return contracts.NotificationResponse{}, errors.New("notification temporarily unavailable")
		}
		return acknowledge(ctx, req)
	})
	finder := finderFunc(func(ctx context.Context, q contracts.NearbyInput) (contracts.NearbyResponse, error) {
		if err := db1.PingContext(ctx); err != nil {
			return contracts.NearbyResponse{}, err
		}
		if q.Lat != in.PickupLocation.Lat || q.Long != in.PickupLocation.Long || q.RadiusMeters != in.RadiusMeters {
			return contracts.NearbyResponse{}, errors.New("wrong pickup or radius")
		}
		return nearby("a", "a", "b"), nil
	})
	app := matching.NewApplication(repo1, rideFunc(requestedRide), finder, notifier)
	if _, err := app.Request(ctx, in); !errors.Is(err, matching.ErrUnavailable) {
		t.Fatalf("partial failure = %v", err)
	}
	before, err := repo2.Get(ctx, in.RideID)
	if err != nil || before.Status != matching.Matching || before.CreatedAt.IsZero() {
		t.Fatalf("persisted job = %+v, %v", before, err)
	}
	fail = false
	app = matching.NewApplication(repo2, rideFunc(requestedRide), finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
		return contracts.NearbyResponse{}, errors.New("retry must not find new candidates")
	}), notifier)
	if got, err := app.Request(ctx, in); err != nil || got.Status != matching.Matching {
		t.Fatalf("retry = %+v, %v", got, err)
	}
	after, err := postgres.NewRepository(db2).Get(ctx, in.RideID)
	if err != nil || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(sent, []string{"a", "b", "a", "b"}) {
		t.Fatalf("retry changed persisted selection: %+v, %v, sends %v", after, err, sent)
	}
}

func TestPostgresConcurrentApplicationsUseCommittedWinner(t *testing.T) {
	for _, mode := range []string{"same_input", "different_input", "empty_selection"} {
		t.Run(mode, func(t *testing.T) {
			ctx := testContext(t)
			in := input(testID(t))
			repos := []*postgres.Repository{postgres.NewRepository(testPool(t)), postgres.NewRepository(testPool(t))}
			ready := make(chan struct{}, 2)
			release := make(chan struct{})
			var mu sync.Mutex
			var sent []string
			notifier := notifyFunc(func(ctx context.Context, req contracts.NotificationRequest) (contracts.NotificationResponse, error) {
				job, err := repos[0].Get(ctx, req.RideID)
				if err != nil || len(job.CandidateDriverIDs) != 1 || req.DriverID != job.CandidateDriverIDs[0] || req.Type != "RIDE_REQUEST" {
					return contracts.NotificationResponse{}, errors.New("dispatched uncommitted or losing selection")
				}
				mu.Lock()
				sent = append(sent, req.DriverID)
				mu.Unlock()
				return acknowledge(ctx, req)
			})
			type outcome struct {
				result matching.Result
				err    error
			}
			done := make(chan outcome, 2)
			for i := 0; i < 2; i++ {
				index := i
				request := in
				if mode == "different_input" && index == 1 {
					request.RadiusMeters++
				}
				finder := finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
					ready <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
						return contracts.NearbyResponse{}, ctx.Err()
					}
					if mode == "empty_selection" && index == 0 {
						return nearby(), nil
					}
					return nearby(fmt.Sprintf("driver-%d", index)), nil
				})
				app := matching.NewApplication(repos[index], rideFunc(requestedRide), finder, notifier)
				go func() {
					got, err := app.Request(ctx, request)
					done <- outcome{result: got, err: err}
				}()
			}
			for i := 0; i < 2; i++ {
				select {
				case <-ready:
				case <-ctx.Done():
					t.Fatal("applications did not concurrently reach finder")
				}
			}
			close(release)
			first, second := <-done, <-done
			if mode == "different_input" {
				if (first.err == nil) == (second.err == nil) ||
					(first.err != nil && !errors.Is(first.err, matching.ErrRequestConflict)) ||
					(second.err != nil && !errors.Is(second.err, matching.ErrRequestConflict)) {
					t.Fatalf("conflicting requests: %v, %v", first.err, second.err)
				}
			} else if first.err != nil || second.err != nil || first.result != second.result {
				t.Fatalf("concurrent results: %+v / %+v", first, second)
			}
			var count int
			if err := integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM matching.requests WHERE ride_id=$1", in.RideID).Scan(&count); err != nil || count != 1 {
				t.Fatalf("row count = %d, %v", count, err)
			}
			job, err := repos[0].Get(ctx, in.RideID)
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			wantSends := 2 * len(job.CandidateDriverIDs)
			if mode == "different_input" {
				wantSends = 1
			}
			if len(sent) != wantSends {
				t.Fatalf("dispatch count = %d, want %d", len(sent), wantSends)
			}
		})
	}
}

func TestPostgresTerminalStateSurvivesOldRequest(t *testing.T) {
	for _, terminal := range []string{"ACCEPTED", "CANCELLED"} {
		t.Run(terminal, func(t *testing.T) {
			ctx := testContext(t)
			in := input(testID(t))
			db1 := testPool(t)
			db1.SetMaxOpenConns(1)
			repo1, repo2 := postgres.NewRepository(db1), postgres.NewRepository(testPool(t))
			entered := make(chan struct{})
			release := make(chan struct{})
			notifier := notifyFunc(func(ctx context.Context, req contracts.NotificationRequest) (contracts.NotificationResponse, error) {
				close(entered)
				select {
				case <-release:
					return acknowledge(ctx, req)
				case <-ctx.Done():
					return contracts.NotificationResponse{}, ctx.Err()
				}
			})
			finder := finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
				return nearby("a"), nil
			})
			app := matching.NewApplication(repo1, rideFunc(requestedRide), finder, notifier)
			type outcome struct {
				result matching.Result
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				got, err := app.Request(ctx, in)
				done <- outcome{result: got, err: err}
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("notification not reached")
			}
			before, err := repo1.Get(ctx, in.RideID)
			if err != nil {
				t.Fatal("database connection held during remote call:", err)
			}
			app2 := matching.NewApplication(repo2, rideFunc(func(context.Context, string) (contracts.Ride, error) {
				return contracts.Ride{RideID: in.RideID, Status: terminal}, nil
			}), nil, nil)
			want := matching.Matched
			if terminal == "CANCELLED" {
				want = matching.Cancelled
			}
			if got, err := app2.Get(ctx, in.RideID); err != nil || got.Status != want {
				t.Fatalf("terminal GET = %+v, %v", got, err)
			}
			if got, err := app2.Request(ctx, in); err != nil || got.Status != want {
				t.Fatalf("terminal retry = %+v, %v", got, err)
			}
			close(release)
			finished := <-done
			if finished.err != nil || finished.result.Status != want {
				t.Fatalf("old request = %+v", finished)
			}
			job, err := repo1.CreateOrGet(ctx, before)
			if err != nil || job.Status != want || !reflect.DeepEqual(job.CandidateDriverIDs, before.CandidateDriverIDs) || !job.CreatedAt.Equal(before.CreatedAt) {
				t.Fatalf("old create regressed terminal job: %+v, %v", job, err)
			}
			if _, err := repo1.AdvanceStatus(ctx, in.RideID, matching.Matching); !errors.Is(err, matching.ErrValidation) {
				t.Fatalf("nonterminal write accepted: %v", err)
			}
			other := matching.Cancelled
			if want == matching.Cancelled {
				other = matching.Matched
			}
			if job, err := repo2.AdvanceStatus(ctx, in.RideID, other); err != nil || job.Status != want {
				t.Fatalf("terminal state overwritten: %+v, %v", job, err)
			}
		})
	}
}

func TestPostgresNoDriversSurvivesRestartAndConflict(t *testing.T) {
	ctx := testContext(t)
	in := input(testID(t))
	repo1 := postgres.NewRepository(testPool(t))
	app := matching.NewApplication(repo1, rideFunc(requestedRide), finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) { return nearby(), nil }), nil)
	if got, err := app.Request(ctx, in); err != nil || got.Status != matching.NoDrivers {
		t.Fatalf("empty selection = %+v, %v", got, err)
	}
	repo2 := postgres.NewRepository(testPool(t))
	app = matching.NewApplication(repo2, rideFunc(requestedRide), nil, nil)
	if got, err := app.Request(ctx, in); err != nil || got.Status != matching.NoDrivers {
		t.Fatalf("NO_DRIVERS retry = %+v, %v", got, err)
	}
	for _, axis := range []string{"lat", "long", "radius"} {
		changed := in
		switch axis {
		case "lat":
			changed.PickupLocation.Lat++
		case "long":
			changed.PickupLocation.Long++
		case "radius":
			changed.RadiusMeters++
		}
		if _, err := repo2.CreateOrGet(ctx, matching.Job{Input: changed, Status: matching.NoDrivers}); !errors.Is(err, matching.ErrRequestConflict) {
			t.Fatalf("%s conflict = %v", axis, err)
		}
	}
}

func TestPostgresCancellationAndDatabaseFailure(t *testing.T) {
	ctx := testContext(t)
	in := input(testID(t))
	db := testPool(t)
	repo := postgres.NewRepository(db)
	if _, err := repo.CreateOrGet(ctx, matching.Job{Input: in, Status: matching.Matching, CandidateDriverIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT ride_id FROM matching.requests WHERE ride_id=$1 FOR UPDATE", in.RideID); err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	other := postgres.NewRepository(testPool(t))
	if _, err := other.AdvanceStatus(short, in.RideID, matching.Cancelled); !errors.Is(err, matching.ErrUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked update timeout = %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if job, err := repo.Get(ctx, in.RideID); err != nil || job.Status != matching.Matching {
		t.Fatalf("canceled update changed job: %+v, %v", job, err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	newInput := input(testID(t))
	if _, err := other.CreateOrGet(cancelled, matching.Job{Input: newInput, Status: matching.NoDrivers}); !errors.Is(err, matching.ErrUnavailable) {
		t.Fatalf("canceled create = %v", err)
	}
	if _, err := other.Get(ctx, newInput.RideID); !errors.Is(err, matching.ErrMatchingNotFound) {
		t.Fatalf("canceled create persisted: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	app := matching.NewApplication(repo, rideFunc(requestedRide), nil, nil)
	if _, err := app.Request(ctx, in); !errors.Is(err, matching.ErrUnavailable) {
		t.Fatalf("closed database produced success: %v", err)
	}
}

func TestPostgresSchemaConstraints(t *testing.T) {
	for _, tc := range []struct {
		name   string
		column int
		value  any
	}{
		{"blank_ride", 0, " "},
		{"null_lat", 1, nil},
		{"latitude_bounds", 1, 91.0},
		{"longitude_bounds", 2, -181.0},
		{"nan_latitude", 1, math.NaN()},
		{"infinite_longitude", 2, math.Inf(1)},
		{"zero_radius", 3, 0.0},
		{"negative_radius", 3, -1.0},
		{"infinite_radius", 3, math.Inf(1)},
		{"nan_radius", 3, math.NaN()},
		{"invalid_status", 4, "ASSIGNED"},
		{"empty_matching", 4, "MATCHING"},
		{"null_candidates", 5, nil},
		{"non_array_candidates", 5, `{}`},
		{"no_drivers_with_candidate", 5, `["a"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testContext(t)
			tx, err := integrationDB.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			args := []any{testID(t), 0.0, 0.0, 3000.0, "NO_DRIVERS", "[]"}
			args[tc.column] = tc.value
			_, err = tx.ExecContext(ctx, `INSERT INTO matching.requests
(ride_id,pickup_lat,pickup_long,radius_meters,status,candidate_driver_ids)
VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, args...)
			if err == nil {
				t.Fatal("schema accepted invalid input")
			}
		})
	}
}
