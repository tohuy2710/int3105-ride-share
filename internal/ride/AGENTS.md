# Ride service ownership

Read the root AGENTS.md, docs/development.md and docs/openapi/ride.yaml.
Own internal/ride and cmd/ride; coordinate shared contract/platform/dependency edits with the integrator. Keep HTTP validation in DTO/handlers and infrastructure out of service.go.

Keep PostgreSQL as final authority. ACCEPT must use conditional UPDATE and commit before 200; Redis expiry/failure cannot create a second winner. Preserve real create/read/cancel/status and one-ride/one-winner scope.

## Team integration ownership

This service stays on the current machine with the integrator. The three delegated service tasks must not edit it. See [team boundaries](../../docs/team/README.md) and the [integration/regression procedure](../../docs/team/integration.md). The documentation assignment itself must leave implementation and tests unchanged.
