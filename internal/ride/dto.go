package ride

import (
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/api"
)

type FareRequest struct {
	PickupLocation *api.Coordinates `json:"pickupLocation" binding:"required"`
	Destination    *api.Coordinates `json:"destination" binding:"required"`
}
type FareResponse struct {
	FareID        string `json:"fareId"`
	EstimatedFare int    `json:"estimatedFare"`
	ETAMinutes    int    `json:"etaMinutes"`
}
type CreateRequest struct {
	FareID string `json:"fareId" binding:"required,notblank"`
}
type RideResponse struct {
	RideID string `json:"rideId"`
	Status string `json:"status"`
}
type DetailsResponse = contracts.Ride
type ActionRequest struct {
	Action string `json:"action" binding:"required,oneof=ACCEPT DENY"`
}
type ActionResponse struct {
	RideID   string  `json:"rideId"`
	Action   string  `json:"action"`
	DriverID *string `json:"driverId"`
	Status   string  `json:"status"`
}
type StatusRequest struct {
	Status string `json:"status" binding:"required,oneof=ARRIVED IN_PROGRESS COMPLETED"`
}
