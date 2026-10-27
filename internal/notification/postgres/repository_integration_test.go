//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/notification"
	"int3105-ride-share/internal/notification/postgres"
	"int3105-ride-share/internal/notification/postgres/migrations"
	"int3105-ride-share/internal/platform/fault"
)

const dsnEnv = "TEST_NOTIFICATION_DATABASE_URL"

// open connects to the dedicated Notification test database and applies the service SQL
// explicitly. A missing DSN fails the run; there is no in-memory fallback.
func open(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Fatal("set " + dsnEnv + " to a dedicated notification test database (e.g. notification_test)")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, migrations.Init); err != nil {
		t.Fatal(err)
	}
	return db
}

// prefix namespaces this test's ride IDs; cleanup deletes only those rows.
func prefix(t *testing.T, db *sql.DB) string {
	t.Helper()
	var b [6]byte
	rand.Read(b[:])
	p := "itest_" + hex.EncodeToString(b[:]) + "_"
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		db.ExecContext(ctx, "DELETE FROM notification.requests WHERE ride_id LIKE $1", p+"%")
	})
	return p
}
func app(db *sql.DB) *notification.Application {
	return notification.NewApplication(postgres.NewRepository(db))
}
func notificationRequest(ride, driver string) contracts.NotificationRequest {
	return contracts.NotificationRequest{RideID: ride, DriverID: driver, Type: notification.TypeRideRequest}
}
func count(t *testing.T, db *sql.DB, ride, driver string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM notification.requests WHERE ride_id=$1 AND driver_id=$2 AND type='RIDE_REQUEST'", ride, driver).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type rowState struct {
	created time.Time
	xmin    string
}

func state(t *testing.T, db *sql.DB, id string) rowState {
	t.Helper()
	var s rowState
	if err := db.QueryRow("SELECT created_at, xmin::text FROM notification.requests WHERE id=$1", id).Scan(&s.created, &s.xmin); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEnqueuePersistsAndGetReturnsSnapshot(t *testing.T) {
	db := open(t)
	p := prefix(t, db)
	s := app(db)
	ctx := context.Background()
	n, err := s.EnqueueRideRequest(ctx, p+"ride", "driver_456", notification.TypeRideRequest)
	if err != nil {
		t.Fatal(err)
	}
	if n.ID == "" || n.ID == "notification_123" || n.Status != notification.StatusQueued || n.RideID != p+"ride" || n.DriverID != "driver_456" || n.Type != notification.TypeRideRequest {
		t.Fatalf("%+v", n)
	}
	var stored notification.Notification
	if err = db.QueryRow("SELECT id, ride_id, driver_id, type, status FROM notification.requests WHERE id=$1", n.ID).Scan(&stored.ID, &stored.RideID, &stored.DriverID, &stored.Type, &stored.Status); err != nil || stored != n {
		t.Fatalf("row %+v %v", stored, err)
	}
	before := state(t, db, n.ID)
	for i := 0; i < 3; i++ {
		got, err := s.GetNotification(ctx, n.ID)
		if err != nil || got != n {
			t.Fatalf("get %+v %v", got, err)
		}
	}
	if after := state(t, db, n.ID); after != before || count(t, db, p+"ride", "driver_456") != 1 {
		t.Fatalf("GET changed row: %+v -> %+v", before, after)
	}
	if r, err := s.SendRideRequest(ctx, notificationRequest(p+"ride", "driver_456")); err != nil || r != n.Response() {
		t.Fatalf("service adapter %+v %v", r, err)
	}
}
func TestValidationAndNotFound(t *testing.T) {
	db := open(t)
	p := prefix(t, db)
	s := app(db)
	ctx := context.Background()
	for _, tc := range [][3]string{{"", "d", "RIDE_REQUEST"}, {p + "r", " ", "RIDE_REQUEST"}, {p + "r", "d", "OTHER"}} {
		if _, err := s.EnqueueRideRequest(ctx, tc[0], tc[1], tc[2]); !errors.Is(err, notification.ErrValidation) {
			t.Fatalf("%q: %v", tc, err)
		}
	}
	if count(t, db, p+"r", "d") != 0 {
		t.Fatal("invalid input stored")
	}
	if _, err := s.GetNotification(ctx, "notification_"+p+"missing"); !errors.Is(err, notification.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.GetNotification(ctx, ""); !errors.Is(err, notification.ErrValidation) {
		t.Fatal(err)
	}
}
func TestSchemaRejectsInvalidRows(t *testing.T) {
	db := open(t)
	p := prefix(t, db)
	for _, args := range [][5]string{{"x" + p, p + "r", "d", "OTHER", "QUEUED"}, {"y" + p, p + "r", "d", "RIDE_REQUEST", "SENT"}, {"z" + p, p + "r", " ", "RIDE_REQUEST", "QUEUED"}, {" ", p + "r", "d", "RIDE_REQUEST", "QUEUED"}} {
		if _, err := db.Exec("INSERT INTO notification.requests (id,ride_id,driver_id,type,status) VALUES ($1,$2,$3,$4,$5)", args[0], args[1], args[2], args[3], args[4]); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}
func TestSequentialRetryKeepsIDAndCreatedAt(t *testing.T) {
	db := open(t)
	p := prefix(t, db)
	s := app(db)
	first, err := s.EnqueueRideRequest(context.Background(), p+"ride", "driver", notification.TypeRideRequest)
	if err != nil {
		t.Fatal(err)
	}
	before := state(t, db, first.ID)
	for i := 0; i < 5; i++ {
		again, err := s.EnqueueRideRequest(context.Background(), p+"ride", "driver", notification.TypeRideRequest)
		if err != nil || again != first {
			t.Fatalf("retry %d: %+v %v", i, again, err)
		}
	}
	if count(t, db, p+"ride", "driver") != 1 || state(t, db, first.ID) != before {
		t.Fatal("retry inserted or rewrote the row")
	}
}
func TestConcurrentRequestsAcrossTwoApplications(t *testing.T) {
	db := open(t)
	p := prefix(t, db)
	other, err := sql.Open("pgx", os.Getenv(dsnEnv))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.Close() })
	apps := []*notification.Application{app(db), app(other)}
	const workers = 32
	ids := make([]string, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			n, err := apps[i%2].EnqueueRideRequest(context.Background(), p+"ride", "driver", notification.TypeRideRequest)
			ids[i], errs[i] = n.ID, err
		}(i)
	}
	close(start)
	wg.Wait()
	for i := range ids {
		if errs[i] != nil || ids[i] == "" || ids[i] != ids[0] {
			t.Fatalf("worker %d: %q %v (first %q)", i, ids[i], errs[i], ids[0])
		}
	}
	if count(t, db, p+"ride", "driver") != 1 {
		t.Fatal("duplicate rows")
	}
}
func TestDifferentRideOrDriverGetsDifferentID(t *testing.T) {
	db := open(t)
	p := prefix(t, db)
	s := app(db)
	seen := map[string]bool{}
	for _, k := range [][2]string{{p + "r1", "d1"}, {p + "r1", "d2"}, {p + "r2", "d1"}, {p + "r2", "d2"}} {
		n, err := s.EnqueueRideRequest(context.Background(), k[0], k[1], notification.TypeRideRequest)
		if err != nil || seen[n.ID] {
			t.Fatalf("%q: %+v %v", k, n, err)
		}
		seen[n.ID] = true
	}
}
func TestRecreatedApplicationReadsAndDeduplicates(t *testing.T) {
	db := open(t)
	p := prefix(t, db)
	first, err := app(db).EnqueueRideRequest(context.Background(), p+"ride", "driver", notification.TypeRideRequest)
	if err != nil {
		t.Fatal(err)
	}
	db.Close() // simulate process restart: a new pool and application, same database
	fresh, err := sql.Open("pgx", os.Getenv(dsnEnv))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fresh.Close() })
	t.Cleanup(func() { fresh.Exec("DELETE FROM notification.requests WHERE ride_id LIKE $1", p+"%") })
	s := app(fresh)
	if got, err := s.GetNotification(context.Background(), first.ID); err != nil || got != first {
		t.Fatalf("get %+v %v", got, err)
	}
	if again, err := s.EnqueueRideRequest(context.Background(), p+"ride", "driver", notification.TypeRideRequest); err != nil || again != first {
		t.Fatalf("dedupe %+v %v", again, err)
	}
}
func TestDatabaseFailuresAreNotSuccess(t *testing.T) {
	db := open(t)
	p := prefix(t, db)

	closed, err := sql.Open("pgx", os.Getenv(dsnEnv))
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	s := app(closed)
	if n, err := s.EnqueueRideRequest(context.Background(), p+"closed", "driver", notification.TypeRideRequest); !errors.Is(err, fault.Unavailable) || n != (notification.Notification{}) {
		t.Fatalf("closed pool enqueue %+v %v", n, err)
	}
	if _, err := s.GetNotification(context.Background(), "any"); !errors.Is(err, fault.Unavailable) {
		t.Fatalf("closed pool get %v", err)
	}

	unreachable, err := sql.Open("pgx", "postgres://nobody:none@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { unreachable.Close() })
	if _, err := app(unreachable).EnqueueRideRequest(context.Background(), p+"down", "driver", notification.TypeRideRequest); !errors.Is(err, fault.Unavailable) {
		t.Fatalf("unreachable %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := app(db).EnqueueRideRequest(ctx, p+"cancelled", "driver", notification.TypeRideRequest); !errors.Is(err, fault.Unavailable) {
		t.Fatalf("cancelled %v", err)
	}
	if count(t, db, p+"closed", "driver")+count(t, db, p+"cancelled", "driver") != 0 {
		t.Fatal("failed request stored a row")
	}
}

// A writer holding the same key blocks Enqueue past its DB timeout: the call must fail rather
// than acknowledge. The server may still commit the abandoned INSERT (pgx cancels
// asynchronously), so the outcome is uncertain; a retry must converge on one durable row.
func TestTimeoutWhileKeyLockedIsNotSuccess(t *testing.T) {
	db := open(t)
	p := prefix(t, db)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO notification.requests (id,ride_id,driver_id,type) VALUES ($1,$2,'driver','RIDE_REQUEST')", "notification_"+p+"held", p+"ride"); err != nil {
		t.Fatal(err)
	}
	s := app(db)
	s.DBTimeout = 300 * time.Millisecond
	if n, err := s.EnqueueRideRequest(context.Background(), p+"ride", "driver", notification.TypeRideRequest); !errors.Is(err, fault.Unavailable) || n != (notification.Notification{}) {
		t.Fatalf("%+v %v", n, err)
	}
	tx.Rollback()
	s.DBTimeout = 0
	retry, err := s.EnqueueRideRequest(context.Background(), p+"ride", "driver", notification.TypeRideRequest)
	if err != nil || retry.ID == "notification_"+p+"held" {
		t.Fatalf("retry %+v %v", retry, err)
	}
	if got, err := s.GetNotification(context.Background(), retry.ID); err != nil || got != retry || count(t, db, p+"ride", "driver") != 1 {
		t.Fatalf("retry not durable/unique: %+v %v", got, err)
	}
}

// Enqueue racing an uncommitted writer must return whichever row actually commits.
func TestEnqueueWaitsForConflictingTransaction(t *testing.T) {
	for _, commit := range []bool{false, true} {
		db := open(t)
		p := prefix(t, db)
		held := "notification_" + p + "held"
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec("INSERT INTO notification.requests (id,ride_id,driver_id,type) VALUES ($1,$2,'driver','RIDE_REQUEST')", held, p+"ride"); err != nil {
			t.Fatal(err)
		}
		type result struct {
			n   notification.Notification
			err error
		}
		done := make(chan result, 1)
		go func() {
			n, err := app(db).EnqueueRideRequest(context.Background(), p+"ride", "driver", notification.TypeRideRequest)
			done <- result{n, err}
		}()
		select {
		case r := <-done:
			t.Fatalf("returned before the conflicting transaction finished: %+v %v", r.n, r.err)
		case <-time.After(200 * time.Millisecond):
		}
		if commit {
			err = tx.Commit()
		} else {
			err = tx.Rollback()
		}
		if err != nil {
			t.Fatal(err)
		}
		r := <-done
		if r.err != nil {
			t.Fatal(r.err)
		}
		if commit != (r.n.ID == held) {
			t.Fatalf("commit=%v returned %q (held %q)", commit, r.n.ID, held)
		}
		if got, err := app(db).GetNotification(context.Background(), r.n.ID); err != nil || got != r.n || count(t, db, p+"ride", "driver") != 1 {
			t.Fatalf("returned ID not durable: %+v %v", got, err)
		}
	}
}
