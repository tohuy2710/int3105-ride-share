CREATE SCHEMA IF NOT EXISTS matching;

CREATE TABLE matching.requests (
    ride_id text PRIMARY KEY CHECK (ride_id ~ '[^[:space:]]'),
    pickup_lat double precision NOT NULL CHECK (pickup_lat BETWEEN -90 AND 90),
    pickup_long double precision NOT NULL CHECK (pickup_long BETWEEN -180 AND 180),
    radius_meters double precision NOT NULL CHECK (radius_meters > 0 AND radius_meters < 'Infinity'::double precision),
    status text NOT NULL CHECK (status IN ('MATCHING', 'NO_DRIVERS', 'MATCHED', 'CANCELLED')),
    candidate_driver_ids jsonb NOT NULL CHECK (jsonb_typeof(candidate_driver_ids) = 'array'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (status <> 'MATCHING' OR jsonb_array_length(candidate_driver_ids) > 0),
    CHECK (status <> 'NO_DRIVERS' OR jsonb_array_length(candidate_driver_ids) = 0)
);
