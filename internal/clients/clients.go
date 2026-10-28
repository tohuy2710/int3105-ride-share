// Package clients defines service ports and deterministic development stubs.
// No method performs HTTP or AMQP I/O in phase 1.
package clients

import (
	"context"
	"int3105-ride-share/internal/contracts"
)

type Ride interface {
	GetRide(context.Context, string) (contracts.Ride, error)
}
type Location interface {
	Nearby(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error)
}
type Matching interface {
	Request(context.Context, contracts.MatchingRequest) (contracts.MatchingResponse, error)
	Get(context.Context, string) (contracts.MatchingResponse, error)
}
type Notification interface {
	Send(context.Context, contracts.NotificationRequest) (contracts.NotificationResponse, error)
}
type RideStub struct{}

func (RideStub) GetRide(_ context.Context, id string) (contracts.Ride, error) {
	return contracts.Ride{RideID: id, FareID: "fare_123", RiderID: "rider_123", Status: "REQUESTED"}, nil
}

type LocationStub struct{}

func (LocationStub) Nearby(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error) {
	return contracts.NearbyResponse{Drivers: []contracts.Driver{{DriverID: "driver_456", Location: contracts.Coordinates{Lat: 21.0285, Long: 105.8542}, DistanceMeters: 500}}}, nil
}

type MatchingStub struct{}

func (MatchingStub) Request(_ context.Context, in contracts.MatchingRequest) (contracts.MatchingResponse, error) {
	return contracts.MatchingResponse{RideID: in.RideID, Status: "MATCHING"}, nil
}
func (MatchingStub) Get(_ context.Context, id string) (contracts.MatchingResponse, error) {
	return contracts.MatchingResponse{RideID: id, Status: "MATCHING"}, nil
}

type NotificationStub struct{}

func (NotificationStub) Send(_ context.Context, in contracts.NotificationRequest) (contracts.NotificationResponse, error) {
	return contracts.NotificationResponse{NotificationID: "notification_123", RideID: in.RideID, DriverID: in.DriverID, Type: in.Type, Status: "QUEUED"}, nil
}

var _ Ride = RideStub{}
var _ Location = LocationStub{}
var _ Matching = MatchingStub{}
var _ Notification = NotificationStub{}
