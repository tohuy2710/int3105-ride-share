# Matching service ownership

Read the root AGENTS.md, docs/development.md and docs/openapi/matching.yaml.
Own internal/matching and cmd/matching; coordinate shared contract/platform/dependency edits with the integrator. Keep HTTP validation in DTO/handlers and infrastructure out of service.go.

Keep MATCHING acknowledgement/read fixtures independent from Ride state. No worker, broker consumer or inter-service calls in phase 1.
