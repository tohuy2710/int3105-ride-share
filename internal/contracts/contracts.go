// Package contracts contains transport-neutral, versioned service messages.
// Business packages may import this package; it must not import frameworks.
package contracts

type User struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

type Coordinates struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

type Ride struct {
	RideID   string  `json:"rideId"`
	FareID   string  `json:"fareId"`
	RiderID  string  `json:"riderId"`
	DriverID *string `json:"driverId"`
	Status   string  `json:"status"`
}

type MatchingRequest struct {
	RideID string `json:"rideId"`
}
type MatchingResponse struct {
	RideID string `json:"rideId"`
	Status string `json:"status"`
}
type Driver struct {
	DriverID       string      `json:"driverId"`
	Location       Coordinates `json:"location"`
	DistanceMeters int         `json:"distanceMeters"`
}
type NearbyResponse struct {
	Drivers []Driver `json:"drivers"`
}
type NearbyInput struct{ Lat, Long, RadiusMeters float64 }
type NotificationRequest struct {
	RideID   string `json:"rideId"`
	DriverID string `json:"driverId"`
	Type     string `json:"type"`
}
type NotificationResponse struct {
	NotificationID string `json:"notificationId"`
	RideID         string `json:"rideId"`
	DriverID       string `json:"driverId"`
	Type           string `json:"type"`
	Status         string `json:"status"`
}
