//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/clients"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/location"
	locationpg "int3105-ride-share/internal/location/postgres"
	"int3105-ride-share/internal/matching"
	matchingpg "int3105-ride-share/internal/matching/postgres"
	"int3105-ride-share/internal/notification"
	notificationpg "int3105-ride-share/internal/notification/postgres"
	"int3105-ride-share/internal/platform/api"
	"int3105-ride-share/internal/platform/database"
	ridepg "int3105-ride-share/internal/ride/postgres"
	"int3105-ride-share/migrations"
)

func eventually(t *testing.T, description string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out: " + description)
}

func flowCall(t *testing.T, method, url, bearer, body string, expected int) map[string]any {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	} else {
		req.Header.Set("X-Internal-Token", "integration-internal-token")
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != expected {
		t.Fatalf("%s %s: %d, want %d: %s", method, url, response.StatusCode, expected, data)
	}
	var result map[string]any
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// This uses the real routers, PostgreSQL repositories and HTTP adapters. Lost
// acknowledgements and a Ride process restart must not drop or duplicate work.
func TestPersistedMatchingFlowAndRestartRecovery(t *testing.T) {
	a, b := setup(t, "redis_sql")
	rideServer := server(t, a)
	survivingRideServer := server(t, b)
	router := func(name string, register func(*gin.Engine)) *httptest.Server {
		r := api.NewRouter(name, slog.New(slog.NewTextHandler(io.Discard, nil)))
		register(r)
		s := httptest.NewServer(r)
		t.Cleanup(s.Close)
		return s
	}
	loc := location.NewApplication(locationpg.NewRepository(b.DB))
	locServer := router("location-service", func(r *gin.Engine) {
		database.Register(r, b.DB, "location.drivers", 2*time.Second)
		location.RegisterRoutes(r, loc)
	})
	note := notification.NewApplication(notificationpg.NewRepository(b.DB))
	noteRouter := api.NewRouter("notification-service", slog.New(slog.NewTextHandler(io.Discard, nil)))
	database.Register(noteRouter, b.DB, "notification.requests", 2*time.Second)
	notification.RegisterRoutes(noteRouter, note)
	var loseAck atomic.Bool
	loseAck.Store(true)
	noteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && loseAck.Load() {
			rec := httptest.NewRecorder()
			noteRouter.ServeHTTP(rec, r)
			// The DB write commits, but Matching sees a failed acknowledgement.
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":{"code":"DEPENDENCY_UNAVAILABLE"}}`))
			return
		}
		noteRouter.ServeHTTP(w, r)
	}))
	t.Cleanup(noteServer.Close)
	client := func(base string) *clients.HTTP {
		c, err := clients.NewHTTP(base, "integration-internal-token")
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	matcher := matching.NewApplication(matchingpg.NewRepository(b.DB), client(survivingRideServer.URL), client(locServer.URL), client(noteServer.URL))
	matchServer := router("matching-service", func(r *gin.Engine) {
		database.Register(r, b.DB, "matching.requests", 2*time.Second)
		matching.RegisterRoutes(r, matcher)
	})
	t.Setenv("MATCHING_URL", matchServer.URL)
	rider := token(t, "rider_123", "RIDER")
	ids := []string{fmt.Sprintf("flow_driver_%d_a", time.Now().UnixNano()), fmt.Sprintf("flow_driver_%d_b", time.Now().UnixNano()), fmt.Sprintf("flow_driver_%d_far", time.Now().UnixNano())}
	for i, id := range ids {
		bearer := token(t, id, "DRIVER")
		coords := `{"lat":0,"long":0}`
		if i == 2 {
			coords = `{"lat":60,"long":60}`
		}
		flowCall(t, "POST", locServer.URL+"/drivers/location", bearer, coords, 200)
		flowCall(t, "PATCH", locServer.URL+"/drivers/availability", bearer, `{"available":true}`, 200)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			b.DB.Exec("DELETE FROM location.drivers WHERE driver_id=$1", id)
		}
	})
	// Missing pickup is rejected before creating any ride.
	flowCall(t, "POST", rideServer.URL+"/rides", rider, `{"fareId":"fare_123"}`, 400)
	created := flowCall(t, "POST", rideServer.URL+"/rides", rider, `{"fareId":"fare_123","pickupLocation":{"lat":0,"long":0}}`, 201)
	id := created["rideId"].(string)
	t.Cleanup(func() {
		b.DB.Exec("DELETE FROM notification.requests WHERE ride_id=$1", id)
		b.DB.Exec("DELETE FROM matching.requests WHERE ride_id=$1", id)
		b.DB.Exec("DELETE FROM ride.rides WHERE id=$1", id)
	})
	if err := a.StartDispatcher(); err != nil {
		t.Fatal(err)
	}
	var firstID string
	eventually(t, "first durable notification before lost acknowledgement", func() bool {
		return b.DB.QueryRow("SELECT id FROM notification.requests WHERE ride_id=$1", id).Scan(&firstID) == nil
	})
	// Stopping the dispatcher models a Ride restart. A fresh replica recovers
	// the leased row after its expiry; no in-memory work is transferred.
	a.Close()
	loseAck.Store(false)
	if err := b.StartDispatcher(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "retry after restart queues exactly the nearby drivers", func() bool {
		var count int
		var dispatched bool
		err := b.DB.QueryRow("SELECT count(*) FROM notification.requests WHERE ride_id=$1", id).Scan(&count)
		if err != nil || count != 2 {
			return false
		}
		return b.DB.QueryRow("SELECT dispatched FROM ride.matching_dispatch WHERE ride_id=$1", id).Scan(&dispatched) == nil && dispatched
	})
	stored := flowCall(t, "GET", noteServer.URL+"/internal/notifications/"+firstID, "", "", 200)
	if stored["rideId"] != id || stored["status"] != "QUEUED" {
		t.Fatal(stored)
	}
	// Idempotent concurrent replay must not duplicate either notification.
	input := contracts.MatchingRequest{RideID: id, PickupLocation: contracts.Coordinates{}, RadiusMeters: 3000}
	var wg sync.WaitGroup
	matchClient := client(matchServer.URL)
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := matchClient.Request(context.Background(), input); errors <- err }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := b.DB.QueryRow("SELECT count(*) FROM notification.requests WHERE ride_id=$1", id).Scan(&count); err != nil || count != 2 {
		t.Fatalf("duplicate notifications: %d %v", count, err)
	}
	var far int
	if err := b.DB.QueryRow("SELECT count(*) FROM notification.requests WHERE ride_id=$1 AND driver_id=$2", id, ids[2]).Scan(&far); err != nil || far != 0 {
		t.Fatalf("far driver notified: %d %v", far, err)
	}
	snapshot := flowCall(t, "GET", matchServer.URL+"/internal/matching/requests/"+id, "", "", 200)
	if snapshot["status"] != "MATCHING" {
		t.Fatal(snapshot)
	}
	flowCall(t, "POST", matchServer.URL+"/internal/matching/requests", "", fmt.Sprintf(`{"rideId":%q,"pickupLocation":{"lat":1,"long":0}}`, id), 409)
	flowCall(t, "GET", matchServer.URL+"/internal/matching/requests/missing", "", "", 404)
	flowCall(t, "GET", noteServer.URL+"/internal/notifications/missing", "", "", 404)
	flowCall(t, "PATCH", survivingRideServer.URL+"/rides/"+id, token(t, ids[0], "DRIVER"), `{"action":"ACCEPT"}`, 200)
	snapshot = flowCall(t, "GET", matchServer.URL+"/internal/matching/requests/"+id, "", "", 200)
	if snapshot["status"] != "MATCHED" {
		t.Fatal(snapshot)
	}
	for _, tc := range []struct {
		lat    float64
		cancel bool
	}{{-60, false}, {-70, true}} {
		created := flowCall(t, "POST", survivingRideServer.URL+"/rides", rider, fmt.Sprintf(`{"fareId":"fare_123","pickupLocation":{"lat":%g,"long":0}}`, tc.lat), 201)
		otherID := created["rideId"].(string)
		t.Cleanup(func() {
			b.DB.Exec("DELETE FROM notification.requests WHERE ride_id=$1", otherID)
			b.DB.Exec("DELETE FROM matching.requests WHERE ride_id=$1", otherID)
			b.DB.Exec("DELETE FROM ride.rides WHERE id=$1", otherID)
		})
		eventually(t, "NO_DRIVERS without notifications", func() bool {
			var status string
			return b.DB.QueryRow("SELECT status FROM matching.requests WHERE ride_id=$1", otherID).Scan(&status) == nil && status == "NO_DRIVERS"
		})
		if tc.cancel {
			flowCall(t, "DELETE", survivingRideServer.URL+"/rides/"+otherID, rider, "", 200)
			snapshot = flowCall(t, "GET", matchServer.URL+"/internal/matching/requests/"+otherID, "", "", 200)
			if snapshot["status"] != "CANCELLED" {
				t.Fatal(snapshot)
			}
		}
		if err := b.DB.QueryRow("SELECT count(*) FROM notification.requests WHERE ride_id=$1", otherID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("NO_DRIVERS notifications: %d %v", count, err)
		}
	}
	// Persistent service readiness fails when its database cannot be reached.
	broken, err := database.Open()
	if err != nil {
		t.Fatal(err)
	}
	broken.Close()
	brokenServer := router("location-service", func(r *gin.Engine) { database.Register(r, broken, "location.drivers", time.Second) })
	flowCall(t, "GET", brokenServer.URL+"/readyz", "", "", 503)
}

func TestMatchingDispatchAtomicityAndLease(t *testing.T) {
	a, b := setup(t, "sql")
	repo := &ridepg.Repository{DB: a.DB}
	id := fmt.Sprintf("atomic_%d", time.Now().UnixNano())
	in := contracts.Ride{RideID: id, FareID: "fare_123", RiderID: "rider_123"}
	request := contracts.MatchingRequest{RideID: id, PickupLocation: contracts.Coordinates{}, RadiusMeters: -1}
	if _, err := repo.CreateWithMatching(context.Background(), in, request); err == nil {
		t.Fatal("invalid dispatch accepted")
	}
	var count int
	if err := a.DB.QueryRow("SELECT count(*) FROM ride.rides WHERE id=$1", id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial creation: %d %v", count, err)
	}
	request.RadiusMeters = 3000
	if _, err := repo.CreateWithMatching(context.Background(), in, request); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.DB.Exec("DELETE FROM ride.rides WHERE id=$1", id) })
	// Lease competition is scoped to this row; unrelated test rides are already cleaned.
	repos := []*ridepg.Repository{repo, {DB: b.DB}}
	var wg sync.WaitGroup
	var wins atomic.Int32
	for _, r := range repos {
		wg.Add(1)
		go func(r *ridepg.Repository) {
			defer wg.Done()
			got, err := r.ClaimMatching(context.Background())
			if err == nil && got.RideID == id {
				wins.Add(1)
			}
		}(r)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("lease winners: %d", wins.Load())
	}
	// Reapplying the migration must preserve the existing pending row.
	if err := migrations.Apply(context.Background(), b.DB); err != nil {
		t.Fatal(err)
	}
	if err := b.DB.QueryRow("SELECT count(*) FROM ride.matching_dispatch WHERE ride_id=$1", id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration lost pending work: %d %v", count, err)
	}
}
