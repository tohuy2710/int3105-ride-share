package notification

type Request struct {
	RideID   string `json:"rideId" binding:"required,notblank"`
	DriverID string `json:"driverId" binding:"required,notblank"`
	Type     string `json:"type" binding:"required,oneof=RIDE_REQUEST"`
}
type Response struct {
	NotificationID string `json:"notificationId"`
	RideID         string `json:"rideId"`
	DriverID       string `json:"driverId"`
	Type           string `json:"type"`
	Status         string `json:"status"`
}
