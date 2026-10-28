package notification

import "int3105-ride-share/internal/contracts"

type Request struct {
	RideID   string `json:"rideId" binding:"required,notblank"`
	DriverID string `json:"driverId" binding:"required,notblank"`
	Type     string `json:"type" binding:"required,oneof=RIDE_REQUEST"`
}
type Response = contracts.NotificationResponse
