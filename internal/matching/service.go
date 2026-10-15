package matching

import "context"

// Service is the extension point for a future business implementation.
type Service interface {
	RequestMatching(ctx context.Context, input Request) (Response, error)
	GetMatching(ctx context.Context, id string) (Response, error)
}

// MockService is stateless: no persistence, transitions or inter-service calls.
type MockService struct{}

func (MockService) RequestMatching(ctx context.Context, input Request) (Response, error) {
	return Response{input.RideID, "MATCHING"}, nil
}
func (MockService) GetMatching(ctx context.Context, id string) (Response, error) {
	return Response{id, "MATCHING"}, nil
}
