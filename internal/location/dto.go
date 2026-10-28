package location

import (
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/api"
)

type LocationRequest = api.Coordinates
type LocationResponse struct {
	DriverID string  `json:"driverId"`
	Lat      float64 `json:"lat"`
	Long     float64 `json:"long"`
}
type AvailabilityRequest struct {
	Available *bool `json:"available" binding:"required"`
}
type AvailabilityResponse struct {
	DriverID  string `json:"driverId"`
	Available bool   `json:"available"`
}
type NearbyQuery struct {
	Lat          *float64 `form:"lat" binding:"required,gte=-90,lte=90"`
	Long         *float64 `form:"long" binding:"required,gte=-180,lte=180"`
	RadiusMeters float64  `form:"radiusMeters,default=3000" binding:"gt=0"`
}
type Driver = contracts.Driver
type NearbyResponse = contracts.NearbyResponse
