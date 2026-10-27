# Ride service ownership

Read the root AGENTS.md, docs/development.md and docs/openapi/ride.yaml.
Own internal/ride and cmd/ride; coordinate shared contract/platform/dependency edits with the integrator. Keep HTTP validation in DTO/handlers and infrastructure out of service.go.

Keep PostgreSQL as final authority. ACCEPT must use conditional UPDATE and commit before 200; Redis expiry/failure cannot create a second winner. Preserve real create/read/cancel/status and one-ride/one-winner scope.
