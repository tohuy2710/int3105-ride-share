-- Idempotent bootstrap; later changes must use new, forward migrations.
CREATE SCHEMA IF NOT EXISTS ride;
CREATE TABLE IF NOT EXISTS ride.rides (
    id text PRIMARY KEY CHECK (length(trim(id)) > 0),
    fare_id text NOT NULL CHECK (length(trim(fare_id)) > 0),
    rider_id text NOT NULL CHECK (length(trim(rider_id)) > 0),
    driver_id text CHECK (driver_id IS NULL OR length(trim(driver_id)) > 0),
    status text NOT NULL DEFAULT 'REQUESTED'
        CHECK (status IN ('REQUESTED','ACCEPTED','ARRIVED','IN_PROGRESS','COMPLETED','CANCELLED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((status IN ('REQUESTED','CANCELLED') AND driver_id IS NULL)
        OR (status IN ('ACCEPTED','ARRIVED','IN_PROGRESS','COMPLETED') AND driver_id IS NOT NULL))
);
