package contracttest

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/auth"
	"int3105-ride-share/internal/auth/fixtures"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/location"
	"int3105-ride-share/internal/matching"
	"int3105-ride-share/internal/notification"
	"int3105-ride-share/internal/platform/api"
	"int3105-ride-share/internal/platform/security"
	"int3105-ride-share/internal/ride"
)

func TestOpenAPIAndActualResponses(t *testing.T) {
	t.Setenv("JWT_SECRET", "contract-test-secret-with-at-least-32-bytes")
	t.Setenv("INTERNAL_TOKEN", "contract-internal-token")
	manager, err := security.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	cases := []struct {
		service, method, path, body, role string
		status                            int
	}{
		{"ride", "POST", "/fare", `{"pickupLocation":{"lat":0,"long":0},"destination":{"lat":90,"long":180}}`, "RIDER", 200},
		{"ride", "POST", "/rides", `{"fareId":"fare_123","pickupLocation":{"lat":0,"long":0}}`, "RIDER", 201},
		{"ride", "GET", "/rides/ride_123", "", "RIDER", 200},
		{"ride", "PATCH", "/rides/ride_123", `{"action":"ACCEPT"}`, "DRIVER", 200},
		{"ride", "PATCH", "/rides/ride_123", `{"action":"DENY"}`, "DRIVER", 200},
		{"ride", "PATCH", "/rides/ride_123/status", `{"status":"ARRIVED"}`, "DRIVER", 200},
		{"ride", "DELETE", "/rides/ride_123", "", "RIDER", 200},
		{"ride", "GET", "/internal/rides/ride_123", "", "internal", 200},
		{"location", "POST", "/drivers/location", `{"lat":0,"long":0}`, "DRIVER", 200},
		{"location", "PATCH", "/drivers/availability", `{"available":false}`, "DRIVER", 200},
		{"location", "GET", "/internal/drivers/nearby?lat=0&long=0", "", "internal", 200},
		{"matching", "POST", "/internal/matching/requests", `{"rideId":"ride_123","pickupLocation":{"lat":0,"long":0}}`, "internal", 202},
		{"matching", "GET", "/internal/matching/requests/ride_123", "", "internal", 200},
		{"notification", "POST", "/internal/notifications/ride-requests", `{"rideId":"ride_123","driverId":"driver_456","type":"RIDE_REQUEST"}`, "internal", 202},
		{"notification", "GET", "/internal/notifications/notification_123", "", "internal", 200},
		{"auth", "POST", "/auth/register", `{"email":"new@example.com","password":"RideShare123!","role":"RIDER"}`, "", 201},
		{"auth", "POST", "/auth/login", `{"email":"driver1@example.com","password":"RideShare123!"}`, "", 200},
		{"auth", "GET", "/auth/me", "", "DRIVER", 200},
	}
	for index, name := range []string{"ride", "location", "matching", "notification", "auth"} {
		t.Run(name, func(t *testing.T) {
			loader := openapi3.NewLoader()
			doc, err := loader.LoadFromFile("../../docs/openapi/" + name + ".yaml")
			if err != nil {
				t.Fatal(err)
			}
			if err = doc.Validate(context.Background()); err != nil {
				t.Fatal(err)
			}
			specRouter, err := legacy.NewRouter(doc)
			if err != nil {
				t.Fatal(err)
			}
			router := api.NewRouter(name+"-service", slog.New(slog.NewTextHandler(io.Discard, nil)))
			switch name {
			case "ride":
				ride.RegisterRoutes(router, ride.MockService{})
			case "location":
				location.RegisterRoutes(router, location.MockService{})
			case "matching":
				matching.RegisterRoutes(router, matching.MockService{})
			case "notification":
				notification.RegisterRoutes(router, notification.MockService{})
			case "auth":
				store, err := fixtures.Load("../../fixtures/accounts.json")
				if err != nil {
					t.Fatal(err)
				}
				auth.RegisterRoutes(router, auth.Service{Credentials: store, Tokens: manager})
			}
			parameter := regexp.MustCompile(`:([A-Za-z][A-Za-z0-9]*)`)
			for _, route := range router.Routes() {
				path := parameter.ReplaceAllString(route.Path, `{$1}`)
				item := doc.Paths.Value(path)
				if item == nil || item.GetOperation(route.Method) == nil {
					t.Errorf("undocumented route: %s %s", route.Method, path)
				}
			}
			for _, tc := range cases {
				if tc.service != name {
					continue
				}
				t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
					req := httptest.NewRequest(tc.method, "http://localhost:"+strconv.Itoa(8081+index)+tc.path, bytes.NewBufferString(tc.body))
					req.Header.Set("Content-Type", "application/json")
					if tc.role == "internal" {
						req.Header.Set("X-Internal-Token", "contract-internal-token")
					} else if tc.role != "" {
						user := contracts.User{UserID: "rider_123", Email: "rider@example.com", Role: tc.role}
						if tc.role == "DRIVER" {
							user.UserID = "driver_456"
							user.Email = "driver1@example.com"
						}
						token, err := manager.Issue(user)
						if err != nil {
							t.Fatal(err)
						}
						req.Header.Set("Authorization", "Bearer "+token)
					}
					route, params, err := specRouter.FindRoute(req)
					if err != nil {
						t.Fatal(err)
					}
					input := &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route, Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}
					if err = openapi3filter.ValidateRequest(context.Background(), input); err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					router.ServeHTTP(rec, req)
					if rec.Code != tc.status {
						t.Fatalf("%d %s", rec.Code, rec.Body.String())
					}
					response := &openapi3filter.ResponseValidationInput{RequestValidationInput: input, Status: rec.Code, Header: rec.Header(), Body: io.NopCloser(bytes.NewReader(rec.Body.Bytes()))}
					if err = openapi3filter.ValidateResponse(context.Background(), response); err != nil {
						t.Fatal(err)
					}
				})
			}
			// Security is exercised by middleware, independently of schema authentication hooks.
			for _, tc := range cases {
				if tc.service != name || tc.role == "" {
					continue
				}
				req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Driver-ID", "driver_456")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if rec.Code != http.StatusUnauthorized {
					t.Fatalf("identity bypass: %s %s: %d", tc.method, tc.path, rec.Code)
				}
			}
		})
	}
}
