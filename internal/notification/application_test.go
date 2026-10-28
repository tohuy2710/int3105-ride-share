package notification

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// memoryRepo is a test double only; the real deduplication lives in the PostgreSQL constraint.
type memoryRepo struct {
	mu    sync.Mutex
	byKey map[string]Notification
	err   error
	calls int
	dead  time.Duration // remaining deadline observed by the last call
}

func (m *memoryRepo) observe(ctx context.Context) {
	m.calls++
	if d, ok := ctx.Deadline(); ok {
		m.dead = time.Until(d)
	}
}
func (m *memoryRepo) Enqueue(ctx context.Context, n Notification) (Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observe(ctx)
	if m.err != nil {
		return Notification{}, m.err
	}
	key := n.RideID + "\x00" + n.DriverID + "\x00" + n.Type
	if old, ok := m.byKey[key]; ok {
		return old, nil
	}
	if m.byKey == nil {
		m.byKey = map[string]Notification{}
	}
	m.byKey[key] = n
	return n, nil
}
func (m *memoryRepo) Get(ctx context.Context, id string) (Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observe(ctx)
	if m.err != nil {
		return Notification{}, m.err
	}
	for _, n := range m.byKey {
		if n.ID == id {
			return n, nil
		}
	}
	return Notification{}, ErrNotFound
}

func TestEnqueueValidationNeverCallsRepository(t *testing.T) {
	repo := &memoryRepo{}
	s := NewApplication(repo)
	for _, tc := range [][3]string{{"", "driver", TypeRideRequest}, {" \t", "driver", TypeRideRequest}, {"ride", "", TypeRideRequest}, {"ride", "  ", TypeRideRequest}, {"ride", "driver", ""}, {"ride", "driver", "ride_request"}, {"ride", "driver", "RIDE_ACCEPTED"}} {
		if _, err := s.EnqueueRideRequest(context.Background(), tc[0], tc[1], tc[2]); !errors.Is(err, ErrValidation) || fault.Code(err) != "VALIDATION_ERROR" {
			t.Fatalf("%q: %v", tc, err)
		}
	}
	for _, id := range []string{"", "   "} {
		if _, err := s.GetNotification(context.Background(), id); !errors.Is(err, ErrValidation) {
			t.Fatalf("%q: %v", id, err)
		}
	}
	if repo.calls != 0 {
		t.Fatalf("repository called %d times", repo.calls)
	}
}
func TestEnqueueGeneratesRealIDsAndDeduplicates(t *testing.T) {
	s := NewApplication(&memoryRepo{})
	ctx := context.Background()
	a, err := s.EnqueueRideRequest(ctx, "ride_1", "driver_1", TypeRideRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a.ID, "notification_") || len(a.ID) != len("notification_")+32 || a.ID == "notification_123" {
		t.Fatalf("id %q", a.ID)
	}
	if a != (Notification{a.ID, "ride_1", "driver_1", TypeRideRequest, StatusQueued}) {
		t.Fatalf("%+v", a)
	}
	again, err := s.EnqueueRideRequest(ctx, "ride_1", "driver_1", TypeRideRequest)
	if err != nil || again != a {
		t.Fatalf("retry %+v %v", again, err)
	}
	other, _ := s.EnqueueRideRequest(ctx, "ride_1", "driver_2", TypeRideRequest)
	otherRide, _ := s.EnqueueRideRequest(ctx, "ride_2", "driver_1", TypeRideRequest)
	if other.ID == a.ID || otherRide.ID == a.ID || other.ID == otherRide.ID {
		t.Fatalf("ids %q %q %q", a.ID, other.ID, otherRide.ID)
	}
	got, err := s.GetNotification(ctx, a.ID)
	if err != nil || got != a {
		t.Fatalf("get %+v %v", got, err)
	}
	if _, err = s.GetNotification(ctx, "notification_missing"); !errors.Is(err, ErrNotFound) || fault.Code(err) != "NOTIFICATION_NOT_FOUND" {
		t.Fatal(err)
	}
}
func TestRepositoryFailureIsNeverSuccess(t *testing.T) {
	for _, cause := range []error{fault.Unavailable, context.DeadlineExceeded} {
		s := NewApplication(&memoryRepo{err: cause})
		if n, err := s.EnqueueRideRequest(context.Background(), "ride", "driver", TypeRideRequest); !errors.Is(err, cause) || n != (Notification{}) {
			t.Fatalf("enqueue %+v %v", n, err)
		}
		if r, err := s.SendRideRequest(context.Background(), contracts.NotificationRequest{RideID: "ride", DriverID: "driver", Type: TypeRideRequest}); err == nil || r != (Response{}) {
			t.Fatalf("send %+v %v", r, err)
		}
		if n, err := s.GetNotification(context.Background(), "id"); !errors.Is(err, cause) || n != (Notification{}) {
			t.Fatalf("get %+v %v", n, err)
		}
	}
}
func TestIDGeneratorFailureIsUnavailable(t *testing.T) {
	repo := &memoryRepo{}
	for _, gen := range []func() (string, error){func() (string, error) { return "", errors.New("entropy") }, func() (string, error) { return " ", nil }} {
		s := &Application{Repo: repo, NewID: gen}
		if _, err := s.EnqueueRideRequest(context.Background(), "ride", "driver", TypeRideRequest); !errors.Is(err, fault.Unavailable) {
			t.Fatal(err)
		}
	}
	if repo.calls != 0 {
		t.Fatal("repository called without an ID")
	}
}
func TestRepositoryCallsAreBounded(t *testing.T) {
	repo := &memoryRepo{}
	s := NewApplication(repo)
	s.EnqueueRideRequest(context.Background(), "ride", "driver", TypeRideRequest)
	if repo.dead <= 0 || repo.dead > 2*time.Second {
		t.Fatalf("default deadline %v", repo.dead)
	}
	s.DBTimeout = 50 * time.Millisecond
	s.GetNotification(context.Background(), "id")
	if repo.dead <= 0 || repo.dead > 50*time.Millisecond {
		t.Fatalf("configured deadline %v", repo.dead)
	}
}
func TestSendRideRequestImplementsService(t *testing.T) {
	var s Service = NewApplication(&memoryRepo{})
	r, err := s.SendRideRequest(context.Background(), contracts.NotificationRequest{RideID: "ride_9", DriverID: "driver_9", Type: TypeRideRequest})
	if err != nil || r.NotificationID == "" || r.NotificationID == "notification_123" || r.RideID != "ride_9" || r.DriverID != "driver_9" || r.Type != TypeRideRequest || r.Status != StatusQueued {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err = s.SendRideRequest(context.Background(), contracts.NotificationRequest{RideID: "ride_9", Type: TypeRideRequest}); !errors.Is(err, ErrValidation) {
		t.Fatal(err)
	}
}
func TestMockFixtureUnchanged(t *testing.T) {
	r, err := MockService{}.SendRideRequest(context.Background(), contracts.NotificationRequest{RideID: "custom", DriverID: "recipient", Type: TypeRideRequest})
	if err != nil || r != (Response{NotificationID: "notification_123", RideID: "custom", DriverID: "recipient", Type: TypeRideRequest, Status: "QUEUED"}) {
		t.Fatalf("%+v %v", r, err)
	}
}

// The real implementation must stay free of transport, provider and framework I/O.
func TestImplementationImports(t *testing.T) {
	forbidden := []string{"net/http", "net/rpc", "net/smtp", "github.com/gin-gonic", "github.com/rabbitmq", "github.com/streadway/amqp", "github.com/redis", "int3105-ride-share/internal/clients", "github.com/gorilla/websocket"}
	files := map[string][]string{
		"application.go": append(forbidden, "database/sql", "github.com/jackc"),
		"service.go":     append(forbidden, "database/sql", "github.com/jackc"),
		filepath.Join("postgres", "repository.go"): forbidden,
	}
	for name, deny := range files {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range f.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			for _, d := range deny {
				if strings.HasPrefix(path, d) {
					t.Errorf("%s imports %s", name, path)
				}
			}
		}
	}
}
