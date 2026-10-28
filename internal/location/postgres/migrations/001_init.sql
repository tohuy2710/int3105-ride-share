-- Proposed Location-only migration; apply explicitly to a dedicated test DB.
-- The integrator must version/checksum this before enabling the real runtime.
BEGIN;

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

COMMIT;
