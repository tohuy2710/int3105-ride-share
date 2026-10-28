// Command benchmark measures synchronized HTTP acceptance bursts against replicas.
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/config"
	"int3105-ride-share/internal/platform/database"
	"int3105-ride-share/internal/platform/metrics"
	"int3105-ride-share/internal/ride/postgres"
)

type sample struct {
	Group        int     `json:"group"`
	RideID       string  `json:"rideId"`
	DriverID     string  `json:"driverId"`
	Result       string  `json:"result"`
	Code         string  `json:"code,omitempty"`
	Status       int     `json:"status"`
	Milliseconds float64 `json:"milliseconds"`
}
type latency struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}
type result struct {
	Mode              string             `json:"mode"`
	Workload          string             `json:"workload"`
	Concurrency       int                `json:"concurrency"`
	Repetition        int                `json:"repetition"`
	Attempts          int                `json:"attempts"`
	Accepted          int                `json:"accepted"`
	Conflicts         int                `json:"conflicts"`
	Errors            int                `json:"errors"`
	AcceptedRides     int                `json:"acceptedRides"`
	MeasuredSeconds   float64            `json:"measuredSeconds"`
	RequestThroughput float64            `json:"requestThroughput"`
	RideThroughput    float64            `json:"rideThroughput"`
	Metrics           metrics.Snapshot   `json:"metrics"`
	Latency           map[string]latency `json:"latency"`
	Samples           []sample           `json:"samples"`
}
type harness struct {
	client                   *http.Client
	urls                     []string
	auth, internal, password string
	rider                    string
	drivers                  []string
	identities               []string
	repo                     *postgres.Repository
}

func (h *harness) call(method, url, token string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Internal-Token", h.internal)
	resp, err := h.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, err
}
func (h *harness) login(email string) (string, string, error) {
	status, data, err := h.call("POST", h.auth+"/auth/login", "", map[string]string{"email": email, "password": h.password})
	if err != nil {
		return "", "", err
	}
	if status != 200 {
		return "", "", fmt.Errorf("login %s: %d %s", email, status, data)
	}
	var out struct {
		AccessToken string         `json:"accessToken"`
		User        contracts.User `json:"user"`
	}
	if err = json.Unmarshal(data, &out); err != nil {
		return "", "", err
	}
	if out.AccessToken == "" || out.User.UserID == "" {
		return "", "", fmt.Errorf("invalid login response")
	}
	return out.AccessToken, out.User.UserID, nil
}
func (h *harness) create() (string, error) {
	status, data, err := h.call("POST", h.urls[0]+"/rides", h.rider, map[string]any{"fareId": "fare_123", "pickupLocation": contracts.Coordinates{Lat: 0, Long: 0}})
	if err != nil {
		return "", err
	}
	if status != 201 {
		return "", fmt.Errorf("create: %d %s", status, data)
	}
	var out struct {
		RideID string `json:"rideId"`
		Status string `json:"status"`
	}
	err = json.Unmarshal(data, &out)
	if err != nil || out.RideID == "" || out.Status != "REQUESTED" {
		return "", fmt.Errorf("invalid create response: %s", data)
	}
	return out.RideID, nil
}
func (h *harness) snapshot() (metrics.Snapshot, error) {
	var total metrics.Snapshot
	for _, base := range h.urls {
		status, data, err := h.call("GET", base+"/internal/metrics", "", nil)
		if err != nil {
			return total, err
		}
		if status != 200 {
			return total, fmt.Errorf("metrics: %d", status)
		}
		var v metrics.Snapshot
		if err = json.Unmarshal(data, &v); err != nil {
			return total, err
		}
		total.SQLAttempts += v.SQLAttempts
		total.RedisRejected += v.RedisRejected
		total.RedisFallback += v.RedisFallback
	}
	return total, nil
}
func (h *harness) group(n, group int, workload string) ([]sample, time.Duration, int, error) {
	ids := make([]string, n)
	for i := range ids {
		if i > 0 && workload == "hot" {
			ids[i] = ids[0]
			continue
		}
		id, err := h.create()
		if err != nil {
			return nil, 0, 0, err
		}
		ids[i] = id
	}
	samples := make([]sample, n)
	barrier := make(chan struct{})
	var wg sync.WaitGroup
	var ready sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		ready.Add(1)
		go func(i int) {
			defer wg.Done()
			ready.Done()
			<-barrier
			started := time.Now()
			status, data, err := h.call("PATCH", h.urls[i%len(h.urls)]+"/rides/"+ids[i], h.drivers[i], map[string]string{"action": "ACCEPT"})
			s := sample{Group: group, RideID: ids[i], DriverID: h.identities[i], Status: status, Milliseconds: float64(time.Since(started).Microseconds()) / 1000, Result: "error"}
			if err != nil {
				s.Code = "TRANSPORT_ERROR"
			} else if status == 200 {
				var out struct {
					RideID   string  `json:"rideId"`
					DriverID *string `json:"driverId"`
					Action   string  `json:"action"`
					Status   string  `json:"status"`
				}
				if json.Unmarshal(data, &out) == nil && out.RideID == ids[i] && out.DriverID != nil && *out.DriverID == h.identities[i] && out.Action == "ACCEPT" && out.Status == "ACCEPTED" {
					s.Result = "accepted"
				} else {
					s.Code = "INVALID_RESPONSE"
				}
			} else {
				var out struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				_ = json.Unmarshal(data, &out)
				s.Code = out.Error.Code
				if status == 409 && (s.Code == "ACCEPT_IN_PROGRESS" || s.Code == "RIDE_ALREADY_ACCEPTED") {
					s.Result = "conflict"
				}
			}
			samples[i] = s
		}(i)
	}
	ready.Wait()
	started := time.Now()
	close(barrier)
	wg.Wait()
	elapsed := time.Since(started)
	winners := map[string]string{}
	for _, s := range samples {
		if s.Result == "accepted" {
			if _, ok := winners[s.RideID]; ok {
				return samples, elapsed, 0, fmt.Errorf("multiple winners for %s", s.RideID)
			}
			winners[s.RideID] = s.DriverID
		}
	}
	expected := n
	if workload == "hot" {
		expected = 1
	}
	if len(winners) != expected {
		return samples, elapsed, len(winners), fmt.Errorf("group %d: expected %d winners, got %d", group, expected, len(winners))
	}
	for id, driver := range winners {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		r, err := h.repo.Get(ctx, id)
		cancel()
		if err != nil || r.Status != "ACCEPTED" || r.DriverID == nil || *r.DriverID != driver {
			return samples, elapsed, len(winners), fmt.Errorf("database verification failed for %s: %v", id, err)
		}
	}
	for _, s := range samples {
		if s.Result == "error" {
			return samples, elapsed, len(winners), fmt.Errorf("unexpected response %d %s", s.Status, s.Code)
		}
	}
	return samples, elapsed, len(winners), nil
}
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	i := int(math.Ceil(float64(len(values))*p)) - 1
	if i < 0 {
		i = 0
	}
	return values[i]
}
func (r *result) finish() {
	values := map[string][]float64{}
	for _, s := range r.Samples {
		r.Attempts++
		switch s.Result {
		case "accepted":
			r.Accepted++
		case "conflict":
			r.Conflicts++
		default:
			r.Errors++
		}
		values[s.Result] = append(values[s.Result], s.Milliseconds)
		values["all"] = append(values["all"], s.Milliseconds)
	}
	r.Latency = map[string]latency{}
	for key, v := range values {
		sort.Float64s(v)
		r.Latency[key] = latency{percentile(v, .50), percentile(v, .95), percentile(v, .99)}
	}
	if r.MeasuredSeconds > 0 {
		r.RequestThroughput = float64(r.Attempts) / r.MeasuredSeconds
		r.RideThroughput = float64(r.AcceptedRides) / r.MeasuredSeconds
	}
}
func run() error {
	urls := flag.String("urls", config.Env("RIDE_URLS", "http://localhost:8081,http://localhost:9081"), "comma-separated replica URLs")
	auth := flag.String("auth", config.Env("AUTH_URL", "http://localhost:8085"), "Auth URL")
	mode := flag.String("mode", config.Env("ACCEPT_MODE", "redis_sql"), "mode label (services must use the same mode)")
	levels := flag.String("concurrency", "2,10,50,100", "concurrency levels")
	groups := flag.Int("groups", 100, "measured bursts per case")
	warmup := flag.Int("warmup", 10, "warm-up bursts per case")
	repeats := flag.Int("repeats", 5, "repetitions")
	output := flag.String("out", ".cache/benchmark/results.json", "output JSON (CSV written alongside)")
	flag.Parse()
	if *groups < 1 || *warmup < 0 || *repeats < 1 {
		return fmt.Errorf("invalid group/repetition count")
	}
	if *mode != "sql" && *mode != "redis_sql" {
		return fmt.Errorf("invalid mode")
	}
	var concurrency []int
	max := 0
	for _, s := range strings.Split(*levels, ",") {
		n, err := strconv.Atoi(s)
		if err != nil || n < 2 {
			return fmt.Errorf("concurrency must be >= 2")
		}
		concurrency = append(concurrency, n)
		if n > max {
			max = n
		}
	}
	h := &harness{urls: strings.Split(*urls, ","), auth: *auth, internal: os.Getenv("INTERNAL_TOKEN"), password: config.Env("FIXTURE_PASSWORD", "RideShare123!")}
	if len(h.urls) < 2 {
		return fmt.Errorf("at least two replicas required")
	}
	for i := range h.urls {
		h.urls[i] = strings.TrimRight(strings.TrimSpace(h.urls[i]), "/")
	}
	h.client = &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{MaxIdleConns: 2 * max, MaxIdleConnsPerHost: max, MaxConnsPerHost: max}}
	defer h.client.CloseIdleConnections()
	db, err := database.Open()
	if err != nil {
		return err
	}
	defer db.Close()
	h.repo = &postgres.Repository{DB: db}
	h.rider, _, err = h.login("rider@example.com")
	if err != nil {
		return err
	}
	for i := 1; i <= max; i++ {
		token, id, err := h.login(fmt.Sprintf("driver%d@example.com", i))
		if err != nil {
			return err
		}
		h.drivers = append(h.drivers, token)
		h.identities = append(h.identities, id)
	}
	var results []result
	var runErr error
cases:
	for rep := 1; rep <= *repeats; rep++ {
		for _, n := range concurrency {
			for _, workload := range []string{"hot", "disjoint"} {
				for i := 0; i < *warmup; i++ {
					if _, _, _, err = h.group(n, -1, workload); err != nil {
						return fmt.Errorf("warmup: %w", err)
					}
				}
				before, err := h.snapshot()
				if err != nil {
					return err
				}
				r := result{Mode: *mode, Workload: workload, Concurrency: n, Repetition: rep}
				for i := 0; i < *groups; i++ {
					samples, elapsed, winners, err := h.group(n, i, workload)
					r.Samples = append(r.Samples, samples...)
					r.MeasuredSeconds += elapsed.Seconds()
					r.AcceptedRides += winners
					if err != nil {
						runErr = err
						break
					}
				}
				after, err := h.snapshot()
				if err != nil {
					return err
				}
				r.Metrics = metrics.Snapshot{SQLAttempts: after.SQLAttempts - before.SQLAttempts, RedisRejected: after.RedisRejected - before.RedisRejected, RedisFallback: after.RedisFallback - before.RedisFallback}
				r.finish()
				results = append(results, r)
				fmt.Printf("%s %s c=%d rep=%d rides/s=%.1f sql=%d conflicts=%d errors=%d\n", *mode, workload, n, rep, r.RideThroughput, r.Metrics.SQLAttempts, r.Conflicts, r.Errors)
				if runErr != nil {
					break cases
				}
			}
		}
	}
	if err = os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
		return err
	}
	f, err := os.Create(strings.TrimSuffix(*output, filepath.Ext(*output)) + ".csv")
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"mode", "workload", "concurrency", "repetition", "group", "rideId", "driverId", "result", "code", "status", "milliseconds"})
	for _, r := range results {
		for _, s := range r.Samples {
			_ = w.Write([]string{r.Mode, r.Workload, strconv.Itoa(r.Concurrency), strconv.Itoa(r.Repetition), strconv.Itoa(s.Group), s.RideID, s.DriverID, s.Result, s.Code, strconv.Itoa(s.Status), strconv.FormatFloat(s.Milliseconds, 'f', 3, 64)})
		}
	}
	w.Flush()
	writeErr := w.Error()
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return runErr
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
