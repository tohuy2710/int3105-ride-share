# Location service ownership

Read the root AGENTS.md, docs/development.md and docs/openapi/location.yaml.
Own internal/location and cmd/location; coordinate shared contract/platform/dependency edits with the integrator. Keep HTTP validation in DTO/handlers and infrastructure out of service.go.

Keep coordinate/availability echo and deterministic nearby fixture. Accept zero coordinates and false availability. Do not add persistence or geospatial matching.
