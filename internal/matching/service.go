package matching

import (
	"context"
	"int3105-ride-share/internal/contracts"
)

// Service is the extension point for a future business implementation.
type Service interface {
	RequestMatching(ctx context.Context, input contracts.MatchingRequest) (Response, error)
	GetMatching(ctx context.Context, id string) (Response, error)
}

// MockService is stateless: no persistence, transitions or inter-service calls.
type MockService struct{}

func (MockService) RequestMatching(ctx context.Context, input contracts.MatchingRequest) (Response, error) {
	return Response{RideID: input.RideID, Status: "MATCHING"}, nil
}
func (MockService) GetMatching(ctx context.Context, id string) (Response, error) {
	return Response{RideID: id, Status: "MATCHING"}, nil
}
