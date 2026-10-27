package matching

import "int3105-ride-share/internal/contracts"

type Request struct {
	RideID string `json:"rideId" binding:"required,notblank"`
}
type Response = contracts.MatchingResponse
