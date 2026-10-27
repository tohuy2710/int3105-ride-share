package location

import (
	"context"
	"encoding/json"
	"errors"
	"int3105-ride-share/internal/contracts"
	"math"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// This fake exists only in tests; PostgreSQL tests exercise real persistence.
type repositoryStub struct {
	states       []DriverState
	err          error
	calls        int
	driverID     string
	position     contracts.Coordinates
	availability bool
}

func (r *repositoryStub) UpsertLocation(_ context.Context, id string, position contracts.Coordinates) error {
	r.calls++
	r.driverID, r.position = id, position
	return r.err
}

func (r *repositoryStub) UpsertAvailability(_ context.Context, id string, available bool) error {
	r.calls++
	r.driverID, r.availability = id, available
	return r.err
}

func (r *repositoryStub) ListAvailable(context.Context) ([]DriverState, error) {
	r.calls++
	return r.states, r.err
}

func TestApplicationUpdates(t *testing.T) {
	repo := &repositoryStub{}
	app := NewApplication(repo)
	if repo.calls != 0 {
		t.Fatal("constructor performed I/O")
	}
	for _, position := range []contracts.Coordinates{{}, {Lat: -90, Long: -180}, {Lat: 90, Long: 180}} {
		result, err := app.UpdateLocation(context.Background(), "verified-driver", position)
		if err != nil || result != (LocationResponse{"verified-driver", position.Lat, position.Long}) || repo.driverID != "verified-driver" || repo.position != position {
			t.Fatalf("update location: result=%+v err=%v repo=%+v", result, err, repo)
		}
	}
	for _, available := range []bool{true, false} {
		result, err := app.UpdateAvailability(context.Background(), "verified-driver", available)
		if err != nil || result != (AvailabilityResponse{"verified-driver", available}) || repo.driverID != "verified-driver" || repo.availability != available {
			t.Fatalf("update availability: result=%+v err=%v repo=%+v", result, err, repo)
		}
	}
}

func TestApplicationRejectsInvalidInputBeforeRepository(t *testing.T) {
	repo := &repositoryStub{}
	app := NewApplication(repo)
	ctx := context.Background()
	for _, id := range []string{"", " \t\n"} {
		if _, err := app.UpdateLocation(ctx, id, contracts.Coordinates{}); !errors.Is(err, ErrValidation) {
			t.Fatalf("driver ID %q: %v", id, err)
		}
		if _, err := app.UpdateAvailability(ctx, id, false); !errors.Is(err, ErrValidation) {
			t.Fatalf("driver ID %q: %v", id, err)
		}
	}
	for _, position := range []contracts.Coordinates{
		{Lat: -90.01}, {Lat: 90.01}, {Long: -180.01}, {Long: 180.01},
		{Lat: math.NaN()}, {Lat: math.Inf(1)}, {Lat: math.Inf(-1)},
		{Long: math.NaN()}, {Long: math.Inf(1)}, {Long: math.Inf(-1)},
	} {
		if _, err := app.UpdateLocation(ctx, "driver", position); !errors.Is(err, ErrValidation) {
			t.Fatalf("position %+v: %v", position, err)
		}
		if _, err := app.NearbyDrivers(ctx, contracts.NearbyInput{Lat: position.Lat, Long: position.Long, RadiusMeters: 3000}); !errors.Is(err, ErrValidation) {
			t.Fatalf("nearby position %+v: %v", position, err)
		}
	}
	for _, radius := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := app.NearbyDrivers(ctx, contracts.NearbyInput{RadiusMeters: radius}); !errors.Is(err, ErrValidation) {
			t.Fatalf("radius %v: %v", radius, err)
		}
	}
	if repo.calls != 0 {
		t.Fatalf("invalid input reached repository %d times", repo.calls)
	}
}

func TestApplicationErrorsDoNotAcknowledgeFailedWrites(t *testing.T) {
	for _, cause := range []error{context.DeadlineExceeded, context.Canceled, errors.New("SQL with private DSN")} {
		err := &DependencyError{Cause: cause}
		repo := &repositoryStub{err: err}
		app := NewApplication(repo)
		position, gotErr := app.UpdateLocation(context.Background(), "driver", contracts.Coordinates{})
		if position != (LocationResponse{}) || !errors.Is(gotErr, ErrDependencyUnavailable) || !errors.Is(gotErr, cause) {
			t.Fatalf("failed location: %+v %v", position, gotErr)
		}
		available, gotErr := app.UpdateAvailability(context.Background(), "driver", true)
		if available != (AvailabilityResponse{}) || !errors.Is(gotErr, ErrDependencyUnavailable) || !errors.Is(gotErr, cause) {
			t.Fatalf("failed availability: %+v %v", available, gotErr)
		}
		_, gotErr = app.NearbyDrivers(context.Background(), contracts.NearbyInput{RadiusMeters: 3000})
		if !errors.Is(gotErr, ErrDependencyUnavailable) || !errors.Is(gotErr, cause) || strings.Contains(gotErr.Error(), "private DSN") {
			t.Fatalf("failed read: %v", gotErr)
		}
	}
	unexpected := errors.New("unexpected application failure")
	app := NewApplication(&repositoryStub{err: unexpected})
	if _, err := app.NearbyDrivers(context.Background(), contracts.NearbyInput{RadiusMeters: 1}); !errors.Is(err, unexpected) || errors.Is(err, ErrDependencyUnavailable) {
		t.Fatalf("unexpected failure was misclassified: %v", err)
	}
}

func located(id string, lat, long float64) DriverState {
	return DriverState{DriverID: id, Location: &contracts.Coordinates{Lat: lat, Long: long}, Available: true}
}

func TestNearbyFilteringAndUnroundedOrdering(t *testing.T) {
	// Both sub-metre distances round to zero. Raw distance must take priority
	// over driver ID, while identical positions must use the ID tie-break.
	degreesPerMetre := 180 / (math.Pi * 6371000)
	unavailable := located("unavailable", 0, 0)
	unavailable.Available = false
	repo := &repositoryStub{states: []DriverState{
		located("a-farther", 0, 0.4*degreesPerMetre),
		located("b-closer", 0, 0.1*degreesPerMetre),
		located("b-tie", 0, 0), located("a-tie", 0, 0),
		located("outside", 0, 2*degreesPerMetre), unavailable,
		{DriverID: "missing", Available: true},
	}}
	result, err := NewApplication(repo).NearbyDrivers(context.Background(), contracts.NearbyInput{RadiusMeters: 1})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, driver := range result.Drivers {
		ids = append(ids, driver.DriverID)
		if driver.DistanceMeters != 0 {
			t.Fatalf("rounding %+v", driver)
		}
	}
	if want := []string{"a-tie", "b-tie", "b-closer", "a-farther"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("got %v want %v", ids, want)
	}
}

func TestNearbyBoundaryUsesRawDistance(t *testing.T) {
	position := contracts.Coordinates{Long: 0.001}
	distance := haversine(contracts.Coordinates{}, position)
	app := NewApplication(&repositoryStub{states: []DriverState{located("boundary", 0, 0.001)}})
	for _, tc := range []struct {
		radius float64
		count  int
	}{
		{distance, 1}, {math.Nextafter(distance, 0), 0},
		// Rounded output is smaller than this radius, but the driver is outside.
		{math.Round(distance) + 0.01, 0}, {distance + 1, 1},
	} {
		result, err := app.NearbyDrivers(context.Background(), contracts.NearbyInput{RadiusMeters: tc.radius})
		if err != nil || len(result.Drivers) != tc.count {
			t.Fatalf("radius %.17g: %+v %v", tc.radius, result, err)
		}
		if tc.count == 1 && result.Drivers[0].DistanceMeters != int(math.Round(distance)) {
			t.Fatalf("wrong rounded distance: %+v", result)
		}
	}
}

func TestNearbyEmptyIsJSONArray(t *testing.T) {
	result, err := NewApplication(&repositoryStub{}).NearbyDrivers(context.Background(), contracts.NearbyInput{RadiusMeters: 1})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil || string(encoded) != `{"drivers":[]}` {
		t.Fatalf("empty response %s: %v", encoded, err)
	}
}

func TestHaversine(t *testing.T) {
	for _, tc := range []struct {
		from, to contracts.Coordinates
		want     float64
	}{
		{contracts.Coordinates{}, contracts.Coordinates{}, 0},
		{contracts.Coordinates{}, contracts.Coordinates{Long: 1}, 111194.92664455874},
		{contracts.Coordinates{}, contracts.Coordinates{Long: 180}, math.Pi * 6371000},
		{contracts.Coordinates{Lat: 36.12, Long: -86.67}, contracts.Coordinates{Lat: -36.12, Long: 93.33}, math.Pi * 6371000},
		{contracts.Coordinates{Long: 179.9}, contracts.Coordinates{Long: -179.9}, 22238.98532891175},
		{contracts.Coordinates{Lat: 90, Long: -180}, contracts.Coordinates{Lat: 90, Long: 180}, 0},
	} {
		got := haversine(tc.from, tc.to)
		if !finite(got) || math.Abs(got-tc.want) > 0.2 {
			t.Fatalf("%+v to %+v: got %v want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

// Keep the existing boundary's presence semantics explicit for later wiring:
// plain business values alone cannot distinguish missing from zero/false.
func TestTargetMappingPreservesMissingVersusZeroAndFalse(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{}`, false}, {`{"lat":0}`, false}, {`{"long":0}`, false},
		{`{"lat":null,"long":0}`, false}, {`{"lat":0,"long":0}`, true},
		{`{"lat":90,"long":-180}`, true}, {`{"lat":91,"long":0}`, false},
	} {
		var request LocationRequest
		if err := json.Unmarshal([]byte(tc.body), &request); err != nil {
			t.Fatal(err)
		}
		if err := binding.Validator.ValidateStruct(request); (err == nil) != tc.ok {
			t.Fatalf("location %s: %v", tc.body, err)
		}
	}
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{}`, false}, {`{"available":null}`, false},
		{`{"available":false}`, true}, {`{"available":true}`, true},
	} {
		var request AvailabilityRequest
		if err := json.Unmarshal([]byte(tc.body), &request); err != nil {
			t.Fatal(err)
		}
		if err := binding.Validator.ValidateStruct(request); (err == nil) != tc.ok {
			t.Fatalf("availability %s: %v", tc.body, err)
		}
	}
}

func TestTargetNearbyMappingRequiresCoordinatesAndDefaultsOnlyMissingRadius(t *testing.T) {
	for _, tc := range []struct {
		query  string
		ok     bool
		radius float64
	}{
		{"", false, 0}, {"lat=0", false, 0}, {"long=0", false, 0},
		{"lat=0&long=0", true, 3000},
		{"lat=0&long=0&radiusMeters=0", false, 0},
		{"lat=0&long=0&radiusMeters=-1", false, 0},
		{"lat=0&long=0&radiusMeters=0.25", true, 0.25},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)
		var request NearbyQuery
		err := c.ShouldBindQuery(&request)
		if (err == nil) != tc.ok || (tc.ok && (request.Lat == nil || request.Long == nil || request.RadiusMeters != tc.radius)) {
			t.Fatalf("query %q: %+v %v", tc.query, request, err)
		}
	}
}
