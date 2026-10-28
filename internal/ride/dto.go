package ride

import (
	"bytes"
	"encoding/json"
	"errors"
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
	FareID         string           `json:"fareId" binding:"required,notblank"`
	PickupLocation *api.Coordinates `json:"pickupLocation" binding:"required"`
	RadiusMeters   *float64         `json:"radiusMeters" binding:"omitempty,gt=0"`
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

func (r *CreateRequest) UnmarshalJSON(data []byte) error {
	type fields CreateRequest
	var decoded fields
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if radius, ok := raw["radiusMeters"]; ok && bytes.Equal(bytes.TrimSpace(radius), []byte("null")) {
		return errors.New("radiusMeters cannot be null")
	}
	*r = CreateRequest(decoded)
	return nil
}
