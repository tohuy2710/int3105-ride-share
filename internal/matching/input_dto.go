package matching

import (
	"bytes"
	"encoding/json"

	"int3105-ride-share/internal/contracts"
)

// InputFields is an opt-in mapping helper for the future HTTP boundary. The
// current Request/DTO and mock endpoint deliberately do not use it yet.
type InputFields struct {
	RideID         string        `json:"rideId"`
	PickupLocation *PickupFields `json:"pickupLocation"`
	RadiusMeters   *float64      `json:"radiusMeters"`
}

type PickupFields struct {
	Lat  *float64 `json:"lat"`
	Long *float64 `json:"long"`
}

// UnmarshalJSON distinguishes an omitted radius (defaulted) from an explicit
// null (invalid). Coordinate pointers already distinguish missing from zero.
func (fields *InputFields) UnmarshalJSON(data []byte) error {
	var raw struct {
		RideID         string          `json:"rideId"`
		PickupLocation *PickupFields   `json:"pickupLocation"`
		RadiusMeters   json.RawMessage `json:"radiusMeters"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := InputFields{RideID: raw.RideID, PickupLocation: raw.PickupLocation}
	if len(raw.RadiusMeters) != 0 {
		if bytes.Equal(bytes.TrimSpace(raw.RadiusMeters), []byte("null")) {
			return ErrValidation
		}
		var radius float64
		if err := json.Unmarshal(raw.RadiusMeters, &radius); err != nil {
			return ErrValidation
		}
		out.RadiusMeters = &radius
	}
	*fields = out
	return nil
}

func (fields InputFields) Normalize() (Input, error) {
	if fields.PickupLocation == nil || fields.PickupLocation.Lat == nil || fields.PickupLocation.Long == nil {
		return Input{}, ErrValidation
	}
	in := Input{
		RideID: fields.RideID,
		PickupLocation: contracts.Coordinates{
			Lat: *fields.PickupLocation.Lat, Long: *fields.PickupLocation.Long,
		},
		RadiusMeters: 3000,
	}
	if fields.RadiusMeters != nil {
		in.RadiusMeters = *fields.RadiusMeters
	}
	if err := in.Validate(); err != nil {
		return Input{}, err
	}
	return in, nil
}
