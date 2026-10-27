//go:build integration

package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/api"
	"int3105-ride-share/internal/platform/fault"
	"int3105-ride-share/internal/platform/security"
	"int3105-ride-share/internal/ride/redisgate"
	"int3105-ride-share/internal/ride/runtime"
	"int3105-ride-share/migrations"
)

func setup(t *testing.T, mode string) (*runtime.Runtime, *runtime.Runtime) {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" || os.Getenv("TEST_REDIS_URL") == "" {
		t.Fatal("set TEST_DATABASE_URL and TEST_REDIS_URL, or run python3 scripts/native_check.py")
	}
	t.Setenv("DATABASE_URL", os.Getenv("TEST_DATABASE_URL"))
	t.Setenv("REDIS_URL", os.Getenv("TEST_REDIS_URL"))
	t.Setenv("ACCEPT_MODE", mode)
	t.Setenv("JWT_SECRET", "integration-test-secret-with-at-least-32-bytes")
	t.Setenv("INTERNAL_TOKEN", "integration-internal-token")
	a, err := runtime.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	b, err := runtime.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = migrations.Apply(ctx, a.DB); err != nil {
		t.Fatal(err)
	}
	return a, b
}
func create(t *testing.T, a *runtime.Runtime) string {
	t.Helper()
	r, err := a.App.CreateRide(context.Background(), "rider_123", "fare_123")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		a.DB.ExecContext(ctx, "DELETE FROM ride.rides WHERE id=$1", r.RideID)
	})
	return r.RideID
}
func token(t *testing.T, id, role string) string {
	t.Helper()
	m, err := security.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	v, err := m.Issue(contracts.User{UserID: id, Email: id + "@example.com", Role: role})
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func server(t *testing.T, rt *runtime.Runtime) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := api.NewRouter("ride-service", slog.New(slog.NewTextHandler(io.Discard, nil)))
	rt.Register(router)
	s := httptest.NewServer(router)
	t.Cleanup(s.Close)
	return s
}
func call(method, url, token, body string) (int, []byte, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}
func TestConcurrentAcceptanceAcrossReplicas(t *testing.T) {
	for _, mode := range []string{"sql", "redis_sql"} {
		t.Run(mode, func(t *testing.T) {
			a, b := setup(t, mode)
			servers := []*httptest.Server{server(t, a), server(t, b)}
			for _, n := range []int{2, 50} {
				t.Run(fmt.Sprint(n), func(t *testing.T) {
					for round := 0; round < 5; round++ {
						id := create(t, a)
						tokens := make([]string, n)
						for i := range tokens {
							tokens[i] = token(t, fmt.Sprintf("driver_%d", i), "DRIVER")
						}
						type response struct {
							status int
							data   []byte
							err    error
						}
						out := make([]response, n)
						start := make(chan struct{})
						var wg sync.WaitGroup
						for i := range tokens {
							wg.Add(1)
							go func(i int) {
								defer wg.Done()
								<-start
								out[i].status, out[i].data, out[i].err = call("PATCH", servers[i%2].URL+"/rides/"+id, tokens[i], `{"action":"ACCEPT"}`)
							}(i)
						}
						close(start)
						wg.Wait()
						winner := -1
						for i, r := range out {
							if r.err != nil {
								t.Fatal(r.err)
							}
							switch r.status {
							case 200:
								if winner != -1 {
									t.Fatal("multiple winners")
								}
								winner = i
							case 409:
								var v struct {
									Error struct {
										Code string `json:"code"`
									} `json:"error"`
								}
								if json.Unmarshal(r.data, &v) != nil || (v.Error.Code != "RIDE_ALREADY_ACCEPTED" && v.Error.Code != "ACCEPT_IN_PROGRESS") {
									t.Fatalf("unexpected conflict: %s", r.data)
								}
							default:
								t.Fatalf("status=%d body=%s", r.status, r.data)
							}
						}
						if winner < 0 {
							t.Fatal("no winner")
						}
						stored, err := b.App.InternalRide(context.Background(), id)
						if err != nil || stored.DriverID == nil || *stored.DriverID != fmt.Sprintf("driver_%d", winner) {
							t.Fatalf("stored=%+v err=%v", stored, err)
						}
						status, data, err := call("PATCH", servers[1].URL+"/rides/"+id, tokens[winner], `{"action":"ACCEPT"}`)
						if err != nil || status != 200 {
							t.Fatalf("idempotent retry: %d %s %v", status, data, err)
						}
					}
				})
			}
		})
	}
}
func TestCancellationRacesAcceptance(t *testing.T) {
	a, b := setup(t, "redis_sql")
	for round := 0; round < 30; round++ {
		id := create(t, a)
		start := make(chan struct{})
		var acceptErr, cancelErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, acceptErr = a.App.RespondToRide(context.Background(), id, "driver_456", "ACCEPT")
		}()
		go func() {
			defer wg.Done()
			<-start
			_, cancelErr = b.App.CancelRide(context.Background(), id, "rider_123")
		}()
		close(start)
		wg.Wait()
		if (acceptErr == nil) == (cancelErr == nil) {
			t.Fatalf("exactly one mutation must win: accept=%v cancel=%v", acceptErr, cancelErr)
		}
		r, err := a.App.InternalRide(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if acceptErr == nil && (r.Status != "ACCEPTED" || r.DriverID == nil) {
			t.Fatalf("bad accepted ride: %+v", r)
		}
		if cancelErr == nil && (r.Status != "CANCELLED" || r.DriverID != nil) {
			t.Fatalf("bad cancelled ride: %+v", r)
		}
	}
}
func TestStateAndOwnership(t *testing.T) {
	a, _ := setup(t, "sql")
	id := create(t, a)
	ctx := context.Background()
	if _, err := a.App.CancelRide(ctx, id, "other_rider"); err != fault.Forbidden {
		t.Fatal(err)
	}
	if _, err := a.App.GetRide(ctx, id, contracts.User{UserID: "driver_456", Role: "DRIVER"}); err != fault.Forbidden {
		t.Fatal(err)
	}
	if _, err := a.App.RespondToRide(ctx, id, "driver_456", "ACCEPT"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.App.UpdateStatus(ctx, id, "driver_456", "COMPLETED"); err != fault.InvalidState {
		t.Fatal(err)
	}
	if _, err := a.App.UpdateStatus(ctx, id, "driver_457", "ARRIVED"); err != fault.Forbidden {
		t.Fatal(err)
	}
	for _, status := range []string{"ARRIVED", "IN_PROGRESS", "COMPLETED"} {
		for i := 0; i < 2; i++ {
			r, err := a.App.UpdateStatus(ctx, id, "driver_456", status)
			if err != nil || r.Status != status {
				t.Fatalf("%+v %v", r, err)
			}
		}
	}
	if _, err := a.App.RespondToRide(ctx, id, "driver_456", "ACCEPT"); err != fault.InvalidState {
		t.Fatal(err)
	}
	if _, err := a.App.RespondToRide(ctx, "missing", "driver_456", "ACCEPT"); err != fault.NotFound {
		t.Fatal(err)
	}
	id = create(t, a)
	for i := 0; i < 2; i++ {
		if _, err := a.App.CancelRide(ctx, id, "rider_123"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.App.RespondToRide(ctx, id, "driver_456", "ACCEPT"); err != fault.InvalidState {
		t.Fatal(err)
	}
}
func TestRedisFailureFallsBackAndDBFailureIs503(t *testing.T) {
	a, _ := setup(t, "redis_sql")
	id := create(t, a)
	unavailable := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 20 * time.Millisecond, MaxRetries: -1, ContextTimeoutEnabled: true})
	defer unavailable.Close()
	a.App.Gate = &redisgate.Gate{Client: unavailable, Timeout: 20 * time.Millisecond}
	if _, err := a.App.RespondToRide(context.Background(), id, "driver_456", "ACCEPT"); err != nil {
		t.Fatal(err)
	}
	if a.Metrics.Snapshot().RedisFallback != 1 || a.Metrics.Snapshot().SQLAttempts != 1 {
		t.Fatal(a.Metrics.Snapshot())
	}
	s := server(t, a)
	a.Redis.Close()
	status, data, err := call("GET", s.URL+"/readyz", "", "")
	if err != nil || status != 200 || !bytes.Contains(data, []byte(`"status":"degraded"`)) {
		t.Fatalf("redis readiness: %d %s %v", status, data, err)
	}
	a.DB.Close()
	status, _, err = call("GET", s.URL+"/rides/"+id, token(t, "rider_123", "RIDER"), "")
	if err != nil || status != 503 {
		t.Fatalf("%d %v", status, err)
	}
	status, _, err = call("GET", s.URL+"/readyz", "", "")
	if err != nil || status != 503 {
		t.Fatalf("%d %v", status, err)
	}
	status, _, err = call("GET", s.URL+"/healthz", "", "")
	if err != nil || status != 200 {
		t.Fatalf("%d %v", status, err)
	}
}
func TestGateRejectionSkipsSQL(t *testing.T) {
	a, _ := setup(t, "redis_sql")
	id := create(t, a)
	key := "ride:accept:" + id
	if err := a.Redis.Set(context.Background(), key, "another-owner", time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	defer a.Redis.Del(context.Background(), key)
	_, err := a.App.RespondToRide(context.Background(), id, "driver_456", "ACCEPT")
	if err != fault.InProgress || a.Metrics.Snapshot().SQLAttempts != 0 || a.Metrics.Snapshot().RedisRejected != 1 {
		t.Fatalf("%v %+v", err, a.Metrics.Snapshot())
	}
}
func TestExpiredLockCannotDeleteNewOwner(t *testing.T) {
	a, _ := setup(t, "redis_sql")
	ctx := context.Background()
	id := create(t, a)
	gate := &redisgate.Gate{Client: a.Redis, TTL: 30 * time.Millisecond, Timeout: 100 * time.Millisecond}
	old, ok, err := gate.Acquire(ctx, id)
	if err != nil || !ok {
		t.Fatalf("%t %v", ok, err)
	}
	time.Sleep(50 * time.Millisecond)
	newer := &redisgate.Gate{Client: a.Redis, TTL: time.Second, Timeout: 100 * time.Millisecond}
	release, ok, err := newer.Acquire(ctx, id)
	if err != nil || !ok {
		t.Fatalf("%t %v", ok, err)
	}
	defer release()
	before := a.Redis.Get(ctx, "ride:accept:"+id).Val()
	old()
	after := a.Redis.Get(ctx, "ride:accept:"+id).Val()
	if before == "" || before != after {
		t.Fatal("old owner removed new lock")
	}
	// Simulate stale holder entering SQL after lease expiration: SQL still arbitrates.
	var wg sync.WaitGroup
	wg.Add(2)
	errors := make([]error, 2)
	for i := range errors {
		go func(i int) { defer wg.Done(); _, errors[i] = a.App.Repo.Accept(ctx, id, fmt.Sprintf("driver_%d", i)) }(i)
	}
	wg.Wait()
	if (errors[0] == nil) == (errors[1] == nil) {
		t.Fatalf("SQL did not arbitrate: %v", errors)
	}
}
func TestLockOwnerProcess(t *testing.T) {
	if os.Getenv("LOCK_HELPER") != "1" {
		return
	}
	opts, err := redis.ParseURL(os.Getenv("TEST_REDIS_URL"))
	if err != nil {
		os.Exit(2)
	}
	gate := &redisgate.Gate{Client: redis.NewClient(opts), TTL: 2 * time.Second, Timeout: time.Second}
	_, ok, err := gate.Acquire(context.Background(), os.Getenv("LOCK_HELPER_ID"))
	if err != nil || !ok {
		os.Exit(3)
	}
	fmt.Println("LOCKED")
	time.Sleep(time.Minute) // parent kills the process while it still owns the lease
	os.Exit(4)
}
func TestCrashedOwnerRecoversAfterTTL(t *testing.T) {
	a, _ := setup(t, "redis_sql")
	id := create(t, a)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, exe, "-test.run=^TestLockOwnerProcess$")
	command.Env = append(os.Environ(), "LOCK_HELPER=1", "LOCK_HELPER_ID="+id)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if command.ProcessState == nil {
			command.Process.Kill()
			command.Wait()
		}
	}()
	locked := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(stdout).ReadString('\n'); locked <- line }()
	select {
	case line := <-locked:
		if strings.TrimSpace(line) != "LOCKED" {
			t.Fatalf("helper: %s", line)
		}
	case <-ctx.Done():
		t.Fatal("helper readiness timeout")
	}
	if err = command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	if _, err = a.App.RespondToRide(context.Background(), id, "driver_456", "ACCEPT"); err != fault.InProgress {
		t.Fatalf("crashed holder lease missing: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for a.Redis.Exists(context.Background(), "ride:accept:"+id).Val() != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if _, err = a.App.RespondToRide(context.Background(), id, "driver_456", "ACCEPT"); err != nil {
		t.Fatal(err)
	}
}
