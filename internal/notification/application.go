package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
	"strings"
	"time"
)

const (
	TypeRideRequest = "RIDE_REQUEST"
	StatusQueued    = "QUEUED" // durably stored; says nothing about broker or device delivery
)

// Notification is the stored request snapshot; it deliberately has no timestamps or delivery fields.
type Notification struct{ ID, RideID, DriverID, Type, Status string }

// Repository persists requests. Enqueue must deduplicate on (RideID, DriverID, Type) with a
// database unique constraint and return the committed row, which may carry an earlier ID.
type Repository interface {
	Enqueue(context.Context, Notification) (Notification, error)
	Get(context.Context, string) (Notification, error)
}

// New codes: the integrator maps NOTIFICATION_NOT_FOUND to 404 and VALIDATION_ERROR to 400.
// Repository failures use fault.Unavailable (503 DEPENDENCY_UNAVAILABLE).
var (
	ErrNotFound   = fault.New("NOTIFICATION_NOT_FOUND", "Notification not found")
	ErrValidation = fault.New("VALIDATION_ERROR", "Invalid notification request")
)

// Application is the real Notification implementation. It satisfies Service, so the
// existing router can serve it once the integrator wires it; the default runtime stays mock.
type Application struct {
	Repo      Repository
	DBTimeout time.Duration
	NewID     func() (string, error) // nil uses random notification_<hex>
}

func NewApplication(repo Repository) *Application { return &Application{Repo: repo} }

func (s *Application) timeout(ctx context.Context) (context.Context, context.CancelFunc) {
	d := s.DBTimeout
	if d <= 0 {
		d = 2 * time.Second
	}
	return context.WithTimeout(ctx, d)
}
func randomID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return "notification_" + hex.EncodeToString(id[:]), nil
}
func blank(s string) bool { return strings.TrimSpace(s) == "" }

// EnqueueRideRequest stores a QUEUED request and returns only after the write committed.
// Retries and concurrent calls with the same (rideID, driverID, type) return the same ID.
func (s *Application) EnqueueRideRequest(ctx context.Context, rideID, driverID, kind string) (Notification, error) {
	if blank(rideID) || blank(driverID) || kind != TypeRideRequest {
		return Notification{}, ErrValidation
	}
	newID := s.NewID
	if newID == nil {
		newID = randomID
	}
	id, err := newID()
	if err != nil || blank(id) {
		return Notification{}, fault.Unavailable
	}
	ctx, cancel := s.timeout(ctx)
	defer cancel()
	return s.Repo.Enqueue(ctx, Notification{ID: id, RideID: rideID, DriverID: driverID, Type: kind, Status: StatusQueued})
}

// GetNotification reads a stored request by ID without side effects.
func (s *Application) GetNotification(ctx context.Context, id string) (Notification, error) {
	if blank(id) {
		return Notification{}, ErrValidation
	}
	ctx, cancel := s.timeout(ctx)
	defer cancel()
	return s.Repo.Get(ctx, id)
}

// SendRideRequest adapts the existing Service interface to the real implementation.
func (s *Application) SendRideRequest(ctx context.Context, input contracts.NotificationRequest) (Response, error) {
	n, err := s.EnqueueRideRequest(ctx, input.RideID, input.DriverID, input.Type)
	if err != nil {
		return Response{}, err
	}
	return n.Response(), nil
}

// Response renders the five-field snapshot shared by POST 202 and the target GET 200.
func (n Notification) Response() Response {
	return Response{NotificationID: n.ID, RideID: n.RideID, DriverID: n.DriverID, Type: n.Type, Status: n.Status}
}
