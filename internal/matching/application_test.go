package matching_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/matching"
)

type rideFunc func(context.Context, string) (contracts.Ride, error)

func (f rideFunc) GetRide(ctx context.Context, id string) (contracts.Ride, error) {
	return f(ctx, id)
}

type finderFunc func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error)

func (f finderFunc) Nearby(ctx context.Context, in contracts.NearbyInput) (contracts.NearbyResponse, error) {
	return f(ctx, in)
}

type notifyFunc func(context.Context, contracts.NotificationRequest) (contracts.NotificationResponse, error)

func (f notifyFunc) RequestRide(ctx context.Context, in contracts.NotificationRequest) (contracts.NotificationResponse, error) {
	return f(ctx, in)
}

// The in-memory repository is a unit-test fake only. PostgreSQL integration
// below uses the real repository for persistence and cross-application races.
type memoryRepository struct {
	mu   sync.Mutex
	jobs map[string]matching.Job
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{jobs: make(map[string]matching.Job)}
}

func copyJob(job matching.Job) matching.Job {
	job.CandidateDriverIDs = append([]string{}, job.CandidateDriverIDs...)
	return job
}

func (r *memoryRepository) Get(ctx context.Context, id string) (matching.Job, error) {
	if ctx.Err() != nil {
		return matching.Job{}, ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return matching.Job{}, matching.ErrMatchingNotFound
	}
	return copyJob(job), nil
}

func (r *memoryRepository) CreateOrGet(ctx context.Context, proposed matching.Job) (matching.Job, error) {
	if ctx.Err() != nil {
		return matching.Job{}, ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if job, ok := r.jobs[proposed.Input.RideID]; ok {
		if job.Input != proposed.Input {
			return matching.Job{}, matching.ErrRequestConflict
		}
		return copyJob(job), nil
	}
	proposed.CreatedAt = time.Now().UTC()
	proposed.UpdatedAt = proposed.CreatedAt
	r.jobs[proposed.Input.RideID] = copyJob(proposed)
	return copyJob(proposed), nil
}

func (r *memoryRepository) AdvanceStatus(ctx context.Context, id string, status matching.Status) (matching.Job, error) {
	if ctx.Err() != nil {
		return matching.Job{}, ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return matching.Job{}, matching.ErrMatchingNotFound
	}
	if job.Status != matching.Matched && job.Status != matching.Cancelled {
		job.Status = status
		job.UpdatedAt = time.Now().UTC()
		r.jobs[id] = copyJob(job)
	}
	return copyJob(job), nil
}

type repositoryOverride struct {
	matching.MatchingRepository
	get     func(context.Context, string) (matching.Job, error)
	create  func(context.Context, matching.Job) (matching.Job, error)
	advance func(context.Context, string, matching.Status) (matching.Job, error)
}

func (r repositoryOverride) Get(ctx context.Context, id string) (matching.Job, error) {
	if r.get != nil {
		return r.get(ctx, id)
	}
	return r.MatchingRepository.Get(ctx, id)
}

func (r repositoryOverride) CreateOrGet(ctx context.Context, job matching.Job) (matching.Job, error) {
	if r.create != nil {
		return r.create(ctx, job)
	}
	return r.MatchingRepository.CreateOrGet(ctx, job)
}

func (r repositoryOverride) AdvanceStatus(ctx context.Context, id string, status matching.Status) (matching.Job, error) {
	if r.advance != nil {
		return r.advance(ctx, id, status)
	}
	return r.MatchingRepository.AdvanceStatus(ctx, id, status)
}

func input(id string) matching.Input {
	return matching.Input{RideID: id, PickupLocation: contracts.Coordinates{Lat: 21.0285, Long: 105.8542}, RadiusMeters: 3000}
}

func requestedRide(_ context.Context, id string) (contracts.Ride, error) {
	return contracts.Ride{RideID: id, FareID: "fare", RiderID: "rider", Status: "REQUESTED"}, nil
}

func nearby(ids ...string) contracts.NearbyResponse {
	out := contracts.NearbyResponse{Drivers: []contracts.Driver{}}
	for _, id := range ids {
		out.Drivers = append(out.Drivers, contracts.Driver{DriverID: id})
	}
	return out
}

func acknowledge(_ context.Context, in contracts.NotificationRequest) (contracts.NotificationResponse, error) {
	return contracts.NotificationResponse{NotificationID: "notification-" + in.DriverID,
		RideID: in.RideID, DriverID: in.DriverID, Type: in.Type, Status: "QUEUED"}, nil
}

func TestInputBoundaryAndValidation(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{"rideId":"r"}`, false},
		{`{"rideId":"r","pickupLocation":null}`, false},
		{`{"rideId":"r","pickupLocation":{}}`, false},
		{`{"rideId":"r","pickupLocation":{"lat":0}}`, false},
		{`{"rideId":"r","pickupLocation":{"long":0}}`, false},
		{`{"rideId":"r","pickupLocation":{"lat":null,"long":0}}`, false},
		{`{"rideId":" ","pickupLocation":{"lat":0,"long":0}}`, false},
		{`{"rideId":"r","pickupLocation":{"lat":91,"long":0}}`, false},
		{`{"rideId":"r","pickupLocation":{"lat":0,"long":181}}`, false},
		{`{"rideId":"r","pickupLocation":{"lat":0,"long":0},"radiusMeters":0}`, false},
		{`{"rideId":"r","pickupLocation":{"lat":0,"long":0},"radiusMeters":null}`, false},
		{`{"rideId":"r","pickupLocation":{"lat":0,"long":0},"radiusMeters":"3000"}`, false},
		{`{"rideId":"r","pickupLocation":{"lat":0,"long":0}}`, true},
		{`{"rideId":"r","pickupLocation":{"lat":90,"long":-180}}`, true},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var fields matching.InputFields
			if err := json.Unmarshal([]byte(tc.body), &fields); err != nil {
				if !tc.ok && errors.Is(err, matching.ErrValidation) {
					return
				}
				t.Fatal(err)
			}
			got, err := fields.Normalize()
			if tc.ok {
				if err != nil || got.RadiusMeters != 3000 {
					t.Fatalf("Normalize = %+v, %v", got, err)
				}
			} else if !errors.Is(err, matching.ErrValidation) {
				t.Fatalf("Normalize error = %v", err)
			}
		})
	}
	for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		for axis := 0; axis < 3; axis++ {
			in := input("r")
			switch axis {
			case 0:
				in.PickupLocation.Lat = n
			case 1:
				in.PickupLocation.Long = n
			case 2:
				in.RadiusMeters = n
			}
			if !errors.Is(in.Validate(), matching.ErrValidation) {
				t.Fatalf("accepted non-finite value: %+v", in)
			}
		}
	}
	for _, p := range []contracts.Coordinates{{Lat: -90, Long: 180}, {Lat: 0, Long: 0}} {
		in := input("r")
		in.PickupLocation = p
		if err := in.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	badRadius := input("r")
	badRadius.RadiusMeters = -1
	if !errors.Is(badRadius.Validate(), matching.ErrValidation) {
		t.Fatal("negative radius accepted")
	}
}

func TestCandidatesAndStableRetry(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
		want []string
	}{
		{"one", []string{"a"}, []string{"a"}},
		{"ordered_and_deduplicated", []string{"b", "a", "b", "c", "a"}, []string{"b", "a", "c"}},
		{"no_drivers", nil, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newMemoryRepository()
			in := input("r")
			findCalls := 0
			finder := finderFunc(func(_ context.Context, q contracts.NearbyInput) (contracts.NearbyResponse, error) {
				findCalls++
				if q != (contracts.NearbyInput{Lat: in.PickupLocation.Lat, Long: in.PickupLocation.Long, RadiusMeters: in.RadiusMeters}) {
					t.Fatalf("incorrect finder input: %+v", q)
				}
				return nearby(tc.ids...), nil
			})
			var sent []string
			notifier := notifyFunc(func(ctx context.Context, req contracts.NotificationRequest) (contracts.NotificationResponse, error) {
				job, err := repo.Get(ctx, req.RideID)
				if err != nil || !reflect.DeepEqual(job.CandidateDriverIDs, tc.want) || req.Type != "RIDE_REQUEST" {
					t.Fatalf("dispatch before persisted selection: %+v, %v", job, err)
				}
				sent = append(sent, req.DriverID)
				return acknowledge(ctx, req)
			})
			wantStatus := matching.Matching
			if len(tc.want) == 0 {
				wantStatus = matching.NoDrivers
			}
			app := matching.NewApplication(repo, rideFunc(requestedRide), finder, notifier)
			first, err := app.Request(ctx, in)
			if err != nil || first.Status != wantStatus {
				t.Fatalf("first = %+v, %v", first, err)
			}
			saved, _ := repo.Get(ctx, in.RideID)
			// A fresh application must not re-query changing availability.
			restarted := matching.NewApplication(repo, rideFunc(requestedRide), finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
				t.Fatal("retry queried nearby")
				return contracts.NearbyResponse{}, nil
			}), notifier)
			second, err := restarted.Request(ctx, in)
			if err != nil || second != first || findCalls != 1 {
				t.Fatalf("retry = %+v, %v, finder calls %d", second, err, findCalls)
			}
			after, _ := repo.Get(ctx, in.RideID)
			if !reflect.DeepEqual(saved, after) {
				t.Fatal("retry changed stored job")
			}
			wantSent := append(append([]string(nil), tc.want...), tc.want...)
			if !reflect.DeepEqual(sent, wantSent) {
				t.Fatalf("sent = %v, want %v", sent, wantSent)
			}
			conflict := in
			conflict.RadiusMeters++
			if _, err := restarted.Request(ctx, conflict); !errors.Is(err, matching.ErrRequestConflict) {
				t.Fatalf("radius conflict = %v", err)
			}
			conflict = in
			conflict.PickupLocation.Lat++
			if _, err := restarted.Request(ctx, conflict); !errors.Is(err, matching.ErrRequestConflict) {
				t.Fatalf("pickup conflict = %v", err)
			}
		})
	}
}

func TestPartialNotificationFailureAndRetry(t *testing.T) {
	repo := newMemoryRepository()
	var requests []contracts.NotificationRequest
	fail := true
	notifier := notifyFunc(func(ctx context.Context, req contracts.NotificationRequest) (contracts.NotificationResponse, error) {
		requests = append(requests, req)
		if req.DriverID == "b" && fail {
			return contracts.NotificationResponse{}, errors.New("remote failed")
		}
		return acknowledge(ctx, req)
	})
	finder := finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
		return nearby("a", "b", "c"), nil
	})
	app := matching.NewApplication(repo, rideFunc(requestedRide), finder, notifier)
	ctx := context.Background()
	if _, err := app.Request(ctx, input("r")); !errors.Is(err, matching.ErrUnavailable) {
		t.Fatalf("partial failure = %v", err)
	}
	before, _ := repo.Get(ctx, "r")
	if before.Status != matching.Matching {
		t.Fatal("partial failure lost job")
	}
	fail = false
	app = matching.NewApplication(repo, rideFunc(requestedRide), finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
		t.Fatal("retry replaced fixed candidates")
		return nearby("replacement"), nil
	}), notifier)
	if _, err := app.Request(ctx, input("r")); err != nil {
		t.Fatal(err)
	}
	after, _ := repo.Get(ctx, "r")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("partial retry changed job")
	}
	var ids []string
	for _, req := range requests {
		if req.RideID != "r" || req.Type != "RIDE_REQUEST" {
			t.Fatalf("wrong tuple: %+v", req)
		}
		ids = append(ids, req.DriverID)
	}
	if !reflect.DeepEqual(ids, []string{"a", "b", "a", "b", "c"}) {
		t.Fatalf("retry requests = %v", ids)
	}
}

func TestRideStatusesAndGetWithoutDispatch(t *testing.T) {
	for _, status := range []string{"ACCEPTED", "ARRIVED", "IN_PROGRESS", "COMPLETED", "CANCELLED"} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			repo := newMemoryRepository()
			rideStatus := status
			rides := rideFunc(func(_ context.Context, id string) (contracts.Ride, error) {
				return contracts.Ride{RideID: id, Status: rideStatus}, nil
			})
			finder := finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
				t.Fatal("terminal ride queried nearby")
				return contracts.NearbyResponse{}, nil
			})
			notifier := notifyFunc(func(context.Context, contracts.NotificationRequest) (contracts.NotificationResponse, error) {
				t.Fatal("terminal ride or GET dispatched")
				return contracts.NotificationResponse{}, nil
			})
			app := matching.NewApplication(repo, rides, finder, notifier)
			if _, err := app.Request(ctx, input("r")); !errors.Is(err, matching.ErrRideNotMatchable) {
				t.Fatalf("new job on terminal ride = %v", err)
			}
			if _, err := app.Get(ctx, "r"); !errors.Is(err, matching.ErrMatchingNotFound) {
				t.Fatalf("missing job = %v", err)
			}
			if _, err := repo.CreateOrGet(ctx, matching.Job{Input: input("r"), Status: matching.Matching, CandidateDriverIDs: []string{"a"}}); err != nil {
				t.Fatal(err)
			}
			want := matching.Matched
			if status == "CANCELLED" {
				want = matching.Cancelled
			}
			got, err := app.Get(ctx, "r")
			if err != nil || got.Status != want {
				t.Fatalf("GET = %+v, %v", got, err)
			}
			if got, err := app.Request(ctx, input("r")); err != nil || got.Status != want {
				t.Fatalf("terminal retry = %+v, %v", got, err)
			}
			rideStatus = "REQUESTED" // An older snapshot must not regress state.
			if got, err := app.Request(ctx, input("r")); err != nil || got.Status != want {
				t.Fatalf("stale snapshot = %+v, %v", got, err)
			}
		})
	}
}

func TestDependencyFailuresDoNotInventSuccess(t *testing.T) {
	remoteErr := errors.New("sensitive dependency detail")
	for _, stage := range []string{"read_db", "ride_missing", "ride_error", "ride_bad_snapshot", "finder", "finder_bad_id", "create_db", "notify", "bad_ack"} {
		t.Run(stage, func(t *testing.T) {
			repo := repositoryOverride{MatchingRepository: newMemoryRepository()}
			rides := rideFunc(requestedRide)
			finder := finderFunc(func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
				return nearby("a"), nil
			})
			notifier := notifyFunc(acknowledge)
			want := matching.ErrUnavailable
			switch stage {
			case "read_db":
				repo.get = func(context.Context, string) (matching.Job, error) { return matching.Job{}, remoteErr }
			case "ride_missing":
				want = matching.ErrRideNotFound
				rides = func(context.Context, string) (contracts.Ride, error) {
					return contracts.Ride{}, matching.ErrRideNotFound
				}
			case "ride_error":
				rides = func(context.Context, string) (contracts.Ride, error) { return contracts.Ride{}, remoteErr }
			case "ride_bad_snapshot":
				rides = func(context.Context, string) (contracts.Ride, error) {
					return contracts.Ride{RideID: "other", Status: "REQUESTED"}, nil
				}
			case "finder":
				finder = func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
					return contracts.NearbyResponse{}, remoteErr
				}
			case "finder_bad_id":
				finder = func(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
					return nearby(" "), nil
				}
			case "create_db":
				repo.create = func(context.Context, matching.Job) (matching.Job, error) { return matching.Job{}, remoteErr }
			case "notify":
				notifier = func(context.Context, contracts.NotificationRequest) (contracts.NotificationResponse, error) {
					return contracts.NotificationResponse{}, remoteErr
				}
			case "bad_ack":
				notifier = func(context.Context, contracts.NotificationRequest) (contracts.NotificationResponse, error) {
					return contracts.NotificationResponse{NotificationID: "wrong"}, nil
				}
			}
			got, err := matching.NewApplication(repo, rides, finder, notifier).Request(context.Background(), input("r"))
			if !errors.Is(err, want) || got != (matching.Result{}) || err.Error() == remoteErr.Error() {
				t.Fatalf("result = %+v, error = %v, want %v", got, err, want)
			}
		})
	}
}

func TestGetFailurePreservesStoredState(t *testing.T) {
	ctx := context.Background()
	base := newMemoryRepository()
	before, _ := base.CreateOrGet(ctx, matching.Job{Input: input("r"), Status: matching.NoDrivers, CandidateDriverIDs: []string{}})
	for _, stage := range []string{"ride", "save_status"} {
		t.Run(stage, func(t *testing.T) {
			repo := repositoryOverride{MatchingRepository: base}
			rides := rideFunc(func(context.Context, string) (contracts.Ride, error) {
				return contracts.Ride{}, matching.ErrUnavailable
			})
			if stage == "save_status" {
				rides = func(context.Context, string) (contracts.Ride, error) {
					return contracts.Ride{RideID: "r", Status: "ACCEPTED"}, nil
				}
				repo.advance = func(context.Context, string, matching.Status) (matching.Job, error) {
					return matching.Job{}, matching.ErrUnavailable
				}
			}
			app := matching.NewApplication(repo, rides, nil, nil)
			if _, err := app.Get(ctx, "r"); !errors.Is(err, matching.ErrUnavailable) {
				t.Fatalf("GET failure = %v", err)
			}
			after, _ := base.Get(ctx, "r")
			if !reflect.DeepEqual(before, after) {
				t.Fatal("GET failure mutated state")
			}
		})
	}
}

func TestContextForwardedToEveryRemotePort(t *testing.T) {
	for _, stage := range []string{"ride", "finder", "notification"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			entered := make(chan struct{})
			block := func(received context.Context) error {
				if received != ctx {
					t.Error("context replaced")
				}
				close(entered)
				<-received.Done()
				return received.Err()
			}
			rides := rideFunc(func(received context.Context, id string) (contracts.Ride, error) {
				if stage == "ride" {
					return contracts.Ride{}, block(received)
				}
				return requestedRide(received, id)
			})
			finder := finderFunc(func(received context.Context, _ contracts.NearbyInput) (contracts.NearbyResponse, error) {
				if stage == "finder" {
					return contracts.NearbyResponse{}, block(received)
				}
				return nearby("a"), nil
			})
			notifier := notifyFunc(func(received context.Context, _ contracts.NotificationRequest) (contracts.NotificationResponse, error) {
				return contracts.NotificationResponse{}, block(received)
			})
			done := make(chan error, 1)
			go func() {
				_, err := matching.NewApplication(newMemoryRepository(), rides, finder, notifier).Request(ctx, input("r"))
				done <- err
			}()
			select {
			case <-entered:
				cancel()
			case <-ctx.Done():
				t.Fatal("remote port not reached")
			}
			if err := <-done; !errors.Is(err, matching.ErrUnavailable) || !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation = %v", err)
			}
		})
	}
}

func TestValidationAndCancellationBeforeIO(t *testing.T) {
	app := matching.NewApplication(nil, nil, nil, nil)
	if _, err := app.Request(context.Background(), matching.Input{}); !errors.Is(err, matching.ErrValidation) {
		t.Fatalf("invalid input = %v", err)
	}
	if _, err := app.Get(context.Background(), " "); !errors.Is(err, matching.ErrValidation) {
		t.Fatalf("invalid path = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := app.Request(ctx, input("r")); !errors.Is(err, context.Canceled) || !errors.Is(err, matching.ErrUnavailable) {
		t.Fatalf("canceled request = %v", err)
	}
	if _, err := app.Get(ctx, "r"); !errors.Is(err, context.Canceled) || !errors.Is(err, matching.ErrUnavailable) {
		t.Fatalf("canceled GET = %v", err)
	}
	lat, long, radius := 0.0, 0.0, 1250.5
	fields := matching.InputFields{RideID: "r", PickupLocation: &matching.PickupFields{Lat: &lat, Long: &long}, RadiusMeters: &radius}
	if got, err := fields.Normalize(); err != nil || got.RadiusMeters != radius {
		t.Fatalf("explicit radius = %+v, %v", got, err)
	}
}
