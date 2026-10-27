# Auth service ownership

Read the root AGENTS.md, docs/development.md and docs/openapi/auth.yaml.
Own internal/auth and cmd/auth; coordinate shared contract/platform/dependency edits with the integrator. Keep HTTP validation in DTO/handlers and infrastructure out of service.go.

Keep registration mock (persisted:false). Only pre-seeded bcrypt accounts can log in. JWT security is real; never accept unsigned tokens or expose password hashes.
