package ride

import (
	"context"
	"errors"
	"int3105-ride-share/internal/platform/fault"
	"int3105-ride-share/internal/platform/metrics"
	"testing"
)

type gateStub struct {
	ok  bool
	err error
}

func (g gateStub) Acquire(context.Context, string) (func(), bool, error) {
	return func() {}, g.ok, g.err
}
func TestGateRejectionNeverCallsRepository(t *testing.T) {
	counters := &metrics.Counters{}
	s := &Application{Gate: gateStub{}, Metrics: counters} // nil repository makes accidental access fail
	_, err := s.RespondToRide(context.Background(), "ride", "driver", "ACCEPT")
	if !errors.Is(err, fault.InProgress) || counters.Snapshot().SQLAttempts != 0 || counters.Snapshot().RedisRejected != 1 {
		t.Fatalf("%v %+v", err, counters.Snapshot())
	}
}
