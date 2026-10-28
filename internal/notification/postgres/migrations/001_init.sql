-- Proposed Notification schema. Applied explicitly by the integrator's versioned runner
-- (or by the service integration test against notification_test), never at startup.
-- Touches only schema notification: no cross-service FK, outbox or delivery log.
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
