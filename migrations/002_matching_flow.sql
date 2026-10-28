-- Forward bootstrap for the integrated flow. Each service owns its own schema.


CREATE SCHEMA IF NOT EXISTS location;

CREATE TABLE IF NOT EXISTS location.drivers (
    driver_id text PRIMARY KEY CHECK (driver_id ~ '[^[:space:]]'),
    lat double precision,
    long double precision,
    available boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL,
    CONSTRAINT coordinates_together CHECK ((lat IS NULL) = (long IS NULL)),
    CONSTRAINT latitude_range CHECK (lat BETWEEN -90 AND 90),
    CONSTRAINT longitude_range CHECK (long BETWEEN -180 AND 180)
);


CREATE SCHEMA IF NOT EXISTS matching;

CREATE TABLE IF NOT EXISTS matching.requests (
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
CREATE SCHEMA IF NOT EXISTS notification;
CREATE TABLE IF NOT EXISTS notification.requests (
    id text PRIMARY KEY CHECK (id ~ '\S'),
    ride_id text NOT NULL CHECK (ride_id ~ '\S'),
    driver_id text NOT NULL CHECK (driver_id ~ '\S'),
    type text NOT NULL CHECK (type IN ('RIDE_REQUEST')),
    status text NOT NULL DEFAULT 'QUEUED' CHECK (status IN ('QUEUED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT requests_ride_driver_type_key UNIQUE (ride_id, driver_id, type)
);

CREATE TABLE IF NOT EXISTS ride.matching_dispatch (
    ride_id text PRIMARY KEY REFERENCES ride.rides(id) ON DELETE CASCADE,
    pickup_lat double precision NOT NULL CHECK (pickup_lat BETWEEN -90 AND 90),
    pickup_long double precision NOT NULL CHECK (pickup_long BETWEEN -180 AND 180),
    radius_meters double precision NOT NULL CHECK (radius_meters > 0 AND radius_meters < 'Infinity'::double precision),
    dispatched boolean NOT NULL DEFAULT false,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    attempts integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS matching_dispatch_pending ON ride.matching_dispatch(next_attempt_at) WHERE NOT dispatched;
