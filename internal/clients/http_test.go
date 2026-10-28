package clients_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"int3105-ride-share/internal/clients"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
)

func TestHTTPRejectsFailedAndMalformedDependencies(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"outage", `{}`, 503}, {"unauthorized", `{}`, 401}, {"invalid-json", `{`, 200},
		{"null", `null`, 200}, {"missing-drivers", `{}`, 200}, {"null-drivers", `{"drivers":null}`, 200},
		{"invalid-driver", `{"drivers":[{"driverId":"","location":{"lat":0,"long":0},"distanceMeters":0}]}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Internal-Token") != "test-internal-token" {
					t.Error("missing internal credentials")
				}
				if r.URL.Query().Get("lat") != "0" || r.URL.Query().Get("radiusMeters") != "3000" {
					t.Error("coordinates lost")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			c, err := clients.NewHTTP(s.URL, "test-internal-token")
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Nearby(context.Background(), contracts.NearbyInput{RadiusMeters: 3000})
			if !errors.Is(err, fault.Unavailable) {
				t.Fatalf("expected unavailable: %v", err)
			}
		})
	}
}

func TestHTTP404DeadlineAndRedirect(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer s.Close()
	c, _ := clients.NewHTTP(s.URL, "test-internal-token")
	if _, err := c.GetRide(context.Background(), "missing"); !errors.Is(err, fault.NotFound) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.GetRide(ctx, "missing"); !errors.Is(err, fault.Unavailable) {
		t.Fatal(err)
	}
	var reached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached.Store(true) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer redirect.Close()
	c, _ = clients.NewHTTP(redirect.URL, "test-internal-token")
	if _, err := c.GetRide(context.Background(), "ride"); !errors.Is(err, fault.Unavailable) || reached.Load() {
		t.Fatalf("redirect followed: %v", err)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	c, _ = clients.NewHTTP(slow.URL, "test-internal-token")
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.GetRide(ctx, "ride"); !errors.Is(err, fault.Unavailable) {
		t.Fatal(err)
	}
}

func TestHTTPDoesNotAcknowledgeMalformedMatching(t *testing.T) {
	for _, body := range []string{`null`, `{"rideId":"different","status":"MATCHING"}`, `{"rideId":"ride","status":"UNKNOWN"}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202); _, _ = w.Write([]byte(body)) }))
		c, _ := clients.NewHTTP(s.URL, "test-internal-token")
		if _, err := c.Request(context.Background(), contracts.MatchingRequest{RideID: "ride"}); !errors.Is(err, fault.Unavailable) {
			t.Error(err)
		}
		s.Close()
	}
}
