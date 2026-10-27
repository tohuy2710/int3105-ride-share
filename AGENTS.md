# Instructions for this repository

This repository is a phase-1 Go/Gin scaffold. Preserve the explicit real/mock boundary.

## Ownership and collaboration

When assigned one service, edit its `cmd/<service>` and `internal/<service>` plus its contract/tests. Shared platform, contracts, clients, go.mod/go.sum, Compose, CI, migrations policy and benchmark are owned by the integrator. The initial project-wide implementation task is integrator work. Use an isolated branch/worktree for each developer; do not overwrite another developer's work. Do not spawn additional agents unless the user explicitly requests delegation.

## Architecture and contracts

- Read `docs/development.md` and `docs/api-catalog.md` before changing behavior.
- OpenAPI in `docs/openapi` is the API source of truth. Update it before changing a wire contract, then update DTOs, client stubs, fixtures and contract tests together.
- `service.go` is application/business code. It must not import Gin, database/sql, SQL drivers, Redis or broker libraries. Use plain domain values and repository/client ports.
- HTTP binding belongs in DTO/handler files. SQL belongs in the PostgreSQL adapter. Infrastructure wiring belongs in runtime/entrypoint code.
- Preserve camelCase, nullable driverId, error envelope and X-Request-ID. Identity comes from verified JWT; internal APIs use X-Internal-Token. Never restore fixed identity middleware.
- Shared contract changes that break consumers require integrator review. Do not silently change fixture values.

## Real/mock boundary

Ride creation/read/accept/cancel/status persist in PostgreSQL. Redis is a best-effort contention gate; SQL conditional update guarantees one winner. Do not replace these with mock storage. No driver-wide active-ride constraint exists in this baseline.

Auth uses bcrypt fixture credentials and signed JWT; register is deliberately mock. Fare, Location, Matching and Notification remain deterministic mock services. RabbitMQ has topology only. Do not add automatic HTTP/AMQP wiring, outbox, extra database tables or CouchDB without task scope requiring them.

## Verification

Run `make fmt-check vet test-race contract build` after relevant Go changes. Integration: `make integration` with TEST_DATABASE_URL/TEST_REDIS_URL, or `python3 scripts/native_check.py` for owned native dependencies. Run a short benchmark suite after changing acceptance, metrics or load generation. Do not invent performance results or label local measurements as Kaggle results. Kaggle verification needs Save & Run All evidence.

Never commit .env, caches, binary/data directories, JWTs or logs. Preserve existing `work/` files. Run explicit migrations, never auto-migrate from each replica.
