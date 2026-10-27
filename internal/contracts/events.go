package contracts

import "time"

type Event[T any] struct {
	EventID    string    `json:"eventId"`
	EventType  string    `json:"eventType"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurredAt"`
	RequestID  string    `json:"requestId"`
	Data       T         `json:"data"`
}
type RideRequested struct {
	RideID  string `json:"rideId"`
	RiderID string `json:"riderId"`
	FareID  string `json:"fareId"`
}
type RideAccepted struct {
	RideID   string `json:"rideId"`
	DriverID string `json:"driverId"`
}
type RideCancelled struct {
	RideID  string `json:"rideId"`
	RiderID string `json:"riderId"`
}
