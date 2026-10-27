# Notification service ownership

Read the root AGENTS.md, docs/development.md and docs/openapi/notification.yaml.
Own internal/notification and cmd/notification; coordinate shared contract/platform/dependency edits with the integrator. Keep HTTP validation in DTO/handlers and infrastructure out of service.go.

Keep notification_123 and QUEUED acknowledgement fixture; echo rideId/driverId/type. No real push or broker consumer in phase 1.
