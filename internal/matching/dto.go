package matching

type Request struct {
	RideID string `json:"rideId" binding:"required,notblank"`
}
type Response struct {
	RideID string `json:"rideId"`
	Status string `json:"status"`
}
