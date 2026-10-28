package api_test

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/location"
	"int3105-ride-share/internal/matching"
	"int3105-ride-share/internal/notification"
	"int3105-ride-share/internal/platform/api"
	"int3105-ride-share/internal/platform/security"
	"int3105-ride-share/internal/ride"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-with-at-least-thirty-two-bytes")
	os.Setenv("INTERNAL_TOKEN", "test-internal-token")
	os.Exit(m.Run())
}
func router(service string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := api.NewRouter(service+"-service", slog.New(slog.NewTextHandler(io.Discard, nil)))
	switch service {
	case "ride":
		ride.RegisterRoutes(r, ride.MockService{})
	case "location":
		location.RegisterRoutes(r, location.MockService{})
	case "matching":
		matching.RegisterRoutes(r, matching.MockService{})
	case "notification":
		notification.RegisterRoutes(r, notification.MockService{})
	}
	return r
}
func request(t *testing.T, r *gin.Engine, method, path, body string, status int) map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", "test-internal-token")
	user := contracts.User{UserID: "rider_123", Email: "rider@example.com", Role: "RIDER"}
	if method == "PATCH" || strings.HasPrefix(path, "/drivers/") {
		user = contracts.User{UserID: "driver_456", Email: "driver1@example.com", Role: "DRIVER"}
	}
	manager, err := security.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	token, err := manager.Issue(user)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Request-ID", "test-request")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != status {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	if w.Header().Get("X-Request-ID") != "test-request" {
		t.Fatal("request ID not propagated")
	}
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestEndpoints(t *testing.T) {
	cases := []struct {
		service, method, path, body string
		status                      int
		want                        string
	}{
		{"ride", "POST", "/fare", `{"pickupLocation":{"lat":0,"long":0},"destination":{"lat":90,"long":180}}`, 200, `{"fareId":"fare_123","estimatedFare":85000,"etaMinutes":12}`},
		{"ride", "POST", "/rides", `{"fareId":"any-fare","pickupLocation":{"lat":0,"long":0}}`, 201, `{"rideId":"ride_123","status":"REQUESTED"}`},
		{"ride", "GET", "/rides/custom", "", 200, `{"rideId":"custom","fareId":"fare_123","riderId":"rider_123","driverId":null,"status":"REQUESTED"}`},
		{"ride", "PATCH", "/rides/custom", `{"action":"ACCEPT"}`, 200, `{"rideId":"custom","action":"ACCEPT","driverId":"driver_456","status":"ACCEPTED"}`},
		{"ride", "PATCH", "/rides/custom", `{"action":"DENY"}`, 200, `{"rideId":"custom","action":"DENY","driverId":null,"status":"REQUESTED"}`},
		{"ride", "PATCH", "/rides/custom/status", `{"status":"COMPLETED"}`, 200, `{"rideId":"custom","status":"COMPLETED"}`},
		{"location", "POST", "/drivers/location", `{"lat":0,"long":0}`, 200, `{"driverId":"driver_456","lat":0,"long":0}`},
		{"location", "PATCH", "/drivers/availability", `{"available":false}`, 200, `{"driverId":"driver_456","available":false}`},
		{"location", "GET", "/internal/drivers/nearby?lat=0&long=0", "", 200, `{"drivers":[{"driverId":"driver_456","location":{"lat":21.0285,"long":105.8542},"distanceMeters":500}]}`},
		{"matching", "POST", "/internal/matching/requests", `{"rideId":"custom","pickupLocation":{"lat":0,"long":0}}`, 202, `{"rideId":"custom","status":"MATCHING"}`},
		{"matching", "GET", "/internal/matching/requests/custom", "", 200, `{"rideId":"custom","status":"MATCHING"}`},
		{"notification", "POST", "/internal/notifications/ride-requests", `{"rideId":"custom","driverId":"recipient","type":"RIDE_REQUEST"}`, 202, `{"notificationId":"notification_123","rideId":"custom","driverId":"recipient","type":"RIDE_REQUEST","status":"QUEUED"}`},
	}
	for _, tc := range cases {
		t.Run(tc.service+tc.method+tc.path+tc.body, func(t *testing.T) {
			got := request(t, router(tc.service), tc.method, tc.path, tc.body, tc.status)
			encoded, _ := json.Marshal(got)
			var want map[string]any
			_ = json.Unmarshal([]byte(tc.want), &want)
			expected, _ := json.Marshal(want)
			if string(encoded) != string(expected) {
				t.Fatalf("got %s want %s", encoded, expected)
			}
		})
	}
}
func TestValidation(t *testing.T) {
	cases := []struct{ service, method, path, body string }{
		{"ride", "POST", "/fare", `{"pickupLocation":{"lat":0},"destination":{"lat":0,"long":0}}`},
		{"ride", "POST", "/fare", `{"pickupLocation":{"lat":91,"long":0},"destination":{"lat":0,"long":0}}`},
		{"ride", "POST", "/fare", `{"pickupLocation":{"lat":0,"long":181},"destination":{"lat":0,"long":0}}`},
		{"ride", "POST", "/fare", `{"pickupLocation":null}`},
		{"ride", "POST", "/rides", `{`}, {"ride", "POST", "/rides", ``}, {"ride", "POST", "/rides", `null`},
		{"ride", "POST", "/rides", `{"fareId":"f"}`},
		{"ride", "POST", "/rides", `{"fareId":"f","pickupLocation":{"lat":0}}`},
		{"ride", "POST", "/rides", `{"fareId":"f","pickupLocation":{"lat":0,"long":0},"radiusMeters":0}`},
		{"ride", "POST", "/rides", `{"fareId":"f","pickupLocation":{"lat":0,"long":0},"radiusMeters":null}`},
		{"matching", "POST", "/internal/matching/requests", `{"rideId":"r"}`},
		{"matching", "POST", "/internal/matching/requests", `{"rideId":"r","pickupLocation":{"lat":0,"long":0},"radiusMeters":0}`},
		{"matching", "POST", "/internal/matching/requests", `{"rideId":"r","pickupLocation":{"lat":0,"long":0},"radiusMeters":null}`},
		{"ride", "POST", "/rides", `{}`}, {"ride", "POST", "/rides", `{"fareId":"   "}`},
		{"ride", "POST", "/rides", `{"fareId":123}`}, {"ride", "POST", "/rides", `{"fareId":"f"} {}`},
		{"ride", "PATCH", "/rides/custom", `{"action":"UNKNOWN"}`},
		{"ride", "PATCH", "/rides/%20", `{"action":"ACCEPT"}`},
		{"ride", "PATCH", "/rides/custom/status", `{"status":"REQUESTED"}`},
		{"location", "POST", "/drivers/location", `{"lat":-91,"long":0}`},
		{"location", "POST", "/drivers/location", `{"lat":0,"long":-181}`},
		{"location", "PATCH", "/drivers/availability", `{}`},
		{"location", "PATCH", "/drivers/availability", `{"available":"false"}`},
		{"location", "GET", "/internal/drivers/nearby?lat=0", ""},
		{"location", "GET", "/internal/drivers/nearby?lat=abc&long=0", ""},
		{"location", "GET", "/internal/drivers/nearby?lat=0&long=0&radiusMeters=0", ""},
		{"location", "GET", "/internal/drivers/nearby?lat=0&long=0&radiusMeters=-1", ""},
		{"location", "GET", "/internal/drivers/nearby?lat=NaN&long=0", ""},
		{"matching", "POST", "/internal/matching/requests", `{"rideId":" "}`},
		{"notification", "POST", "/internal/notifications/ride-requests", `{"rideId":"r","driverId":"d","type":"OTHER"}`},
		{"notification", "POST", "/internal/notifications/ride-requests", `{"rideId":"r","type":"RIDE_REQUEST"}`},
	}
	for _, tc := range cases {
		t.Run(tc.service+tc.method+tc.path+tc.body, func(t *testing.T) {
			got := request(t, router(tc.service), tc.method, tc.path, tc.body, 400)
			e := got["error"].(map[string]any)
			if e["code"] != "VALIDATION_ERROR" || len(e["details"].([]any)) == 0 {
				t.Fatalf("invalid error: %v", got)
			}
		})
	}
}
func TestStatelessAndEnums(t *testing.T) {
	r := router("ride")
	request(t, r, "PATCH", "/rides/demo", `{"action":"ACCEPT"}`, 200)
	for _, status := range []string{"ARRIVED", "IN_PROGRESS", "COMPLETED"} {
		request(t, r, "PATCH", "/rides/demo/status", `{"status":"`+status+`"}`, 200)
	}
	got := request(t, r, "GET", "/rides/demo", "", 200)
	if got["status"] != "REQUESTED" {
		t.Fatal("mock persisted state")
	}
	r = router("location")
	request(t, r, "POST", "/drivers/location", `{"lat":-90,"long":-180}`, 200)
	request(t, r, "PATCH", "/drivers/availability", `{"available":true}`, 200)
	request(t, r, "GET", "/internal/drivers/nearby?lat=90&long=180&radiusMeters=1", "", 200)
}
func TestFramework(t *testing.T) {
	for _, service := range []string{"ride", "location", "matching", "notification"} {
		r := router(service)
		got := request(t, r, "GET", "/healthz", "", 200)
		if got["service"] != service+"-service" || got["status"] != "ok" {
			t.Fatal(got)
		}
		for _, tc := range []struct {
			method, path, code string
			status             int
		}{{"GET", "/missing", "NOT_FOUND", 404}, {"POST", "/healthz", "METHOD_NOT_ALLOWED", 405}} {
			got = request(t, r, tc.method, tc.path, "", tc.status)
			if got["error"].(map[string]any)["code"] != tc.code {
				t.Fatal(got)
			}
		}
		req := httptest.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if len(w.Header().Get("X-Request-ID")) != 32 {
			t.Fatal("request ID not generated")
		}
	}
	r := router("ride")
	r.GET("/panic", func(c *gin.Context) { panic("secret") })
	got := request(t, r, "GET", "/panic", "", 500)
	if got["error"].(map[string]any)["message"] != "Internal server error" {
		t.Fatal(got)
	}
}
