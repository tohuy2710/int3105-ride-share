package runtime

import (
	"context"
	"database/sql"
	"errors"
	"int3105-ride-share/internal/clients"
	"int3105-ride-share/internal/platform/config"
	"int3105-ride-share/internal/ride/postgres"
	"log/slog"
	"os"
	"time"
)

// StartDispatcher starts the durable matching outbox. Close cancels and joins it.
// Errors retain the row for retry, including ambiguous remote acknowledgements.
func (rt *Runtime) StartDispatcher() error {
	client, err := clients.NewHTTP(config.Env("MATCHING_URL", "http://localhost:8083"), os.Getenv("INTERNAL_TOKEN"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	rt.stopDispatch = cancel
	rt.dispatchDone = make(chan struct{})
	go func() {
		defer close(rt.dispatchDone)
		repo := &postgres.Repository{DB: rt.DB}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			// Drain a bounded batch; each claim has an independent HTTP deadline.
			for i := 0; i < 20 && ctx.Err() == nil; i++ {
				attempt, done := context.WithTimeout(ctx, 10*time.Second)
				in, err := repo.ClaimMatching(attempt)
				if errors.Is(err, sql.ErrNoRows) {
					done()
					break
				}
				if err == nil {
					current, readErr := repo.Get(attempt, in.RideID)
					err = readErr
					if err == nil && current.Status == "REQUESTED" {
						_, err = client.Request(attempt, in)
					}
					if err == nil {
						err = repo.CompleteMatching(attempt, in.RideID)
					}
				}
				done()
				if err != nil && ctx.Err() == nil {
					slog.Warn("matching dispatch deferred", "rideId", in.RideID, "error", err)
				}
				if err != nil && in.RideID == "" {
					break
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}
