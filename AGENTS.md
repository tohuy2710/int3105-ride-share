# Instructions for this repository

This repository has a working phase-1 Go/Gin baseline. Preserve the race-condition implementation and distinguish current mock runtime from the new service-development assignments.

## Team assignments — read before doing service work

Read [team ownership and boundaries](docs/team/README.md), [development](docs/development.md) and the [current API catalog](docs/api-catalog.md). Use exactly the instruction for your assigned service:

| Assignee | Requirements | Copyable agent instruction | Handoff |
| --- | --- | --- | --- |
| Person 1 — Location | [Location requirements](docs/team/location-requirements.md) | [Location agent prompt](docs/team/location-agent.md) | [Location handoff](docs/team/handoffs/location.md) |
| Person 2 — Matching | [Matching requirements](docs/team/matching-requirements.md) | [Matching agent prompt](docs/team/matching-agent.md) | [Matching handoff](docs/team/handoffs/matching.md) |
| Person 3 — Notification | [Notification requirements](docs/team/notification-requirements.md) | [Notification agent prompt](docs/team/notification-agent.md) | [Notification handoff](docs/team/handoffs/notification.md) |
| Current machine — integrator | Ride/race, Auth, existing shared infrastructure, contracts, runtime wiring, benchmark/Kaggle | [Integration procedure](docs/team/integration.md) | Review and complete each service handoff |

Baseline race commit: `8dd4bd0cadefe73c82b92ee5fd06c87b06cf7cd9`. Start service branches from the integrator's published common commit containing these instructions, which may be newer than that baseline. Record the actual base SHA. Each developer uses a separate branch/worktree; do not overwrite other people's work. Do not spawn additional agents unless the user explicitly requests delegation.

## Ownership and staged delivery

Service assignees may add real application/domain/port code, PostgreSQL adapters, private SQL and tests inside `internal/<assigned-service>/`, and update only their allocated handoff document. Do not edit instruction/requirements documents to widen your own scope. Preserve current Service/MockService interfaces and the behavior of existing router/handler/DTO files; add independent real implementations/models as necessary. Default runtime remains mock until the integrator switches it deliberately.

All `cmd/` runtime wiring, Ride/Auth/platform, shared contracts/clients/tests, published OpenAPI, root migrations, dependencies, Docker/Compose/configuration, CI, scripts, fixtures, notebooks and benchmark belong to the integrator. Do not change these from a service branch. Propose needed changes in the handoff and continue independent application/repository work. The initial project-wide baseline and the current documentation task are integrator work.

The integrator publishes contract deltas, applies reviewed/versioned migrations, maps handlers and wires HTTP/RabbitMQ after receiving the service code. See [integration procedure](docs/team/integration.md). A service application delivery is not evidence that its real HTTP endpoint or transport has been enabled.

## Architecture and contracts

- `service.go` and all new application/business files must not import Gin, database/sql, SQL drivers, Redis or broker libraries. Use plain domain values and repository/client ports.
- HTTP binding belongs in DTO/handler files. SQL belongs in a PostgreSQL adapter. Infrastructure wiring belongs in runtime/entrypoint code owned by the integrator.
- Published OpenAPI in `docs/openapi` is the current API source of truth. Team requirements describe the future real contract; only the integrator updates shared OpenAPI/models/client stubs/error mapping/callers/tests together before activating it.
- Preserve camelCase, nullable driverId, error envelope and X-Request-ID. Identity comes from verified JWT; internal APIs use X-Internal-Token. Never restore fixed identity middleware.
- Matching's future pickup input and statuses, Notification's future GET/error, and dynamic Location/Notification outputs are deliberate changes for integration. Do not fake coordinates, silently change fixtures or relax shared tests to bypass the transition.

## Protected baseline and permitted new logic

Ride creation/read/accept/cancel/status persist in PostgreSQL. Redis is only a contention gate; the SQL conditional transaction guarantees one winner, including lease expiry and Redis failure. Preserve commit-before-success, ownership, retries, cancellation/status, both acceptance modes, race/fault tests and benchmark semantics. No driver-wide active-ride constraint exists in this baseline. Matching must never assign a winner or call ACCEPT. Do not introduce synchronous remote work into the ACCEPT transaction.

Auth uses bcrypt fixture credentials and signed JWT; register and fare remain mock. Location/Matching/Notification currently run deterministic MockService implementations. Their assigned new logic may use only their own PostgreSQL schemas (`location`, `matching`, `notification`) according to team requirements; no cross-service SQL/FK, Ride schema edits, startup auto-migration, CouchDB or outbox in those service deliveries.

RabbitMQ currently has topology only. Service assignees must not add inter-service HTTP/AMQP adapters, workers or push providers. Fake remote ports are for tests, not runtime fallback. Transport and any future Ride publishing changes are separate integrator work with regression checks.

## Verification and handoff

For Go implementation changes run `make fmt-check vet test-race contract build` plus the service's PostgreSQL integration tests. Format only files within your ownership. Integration tests require real PostgreSQL and an explicit service test DSN; do not silently skip or fall back to RAM when the integration tag is selected.

The integrator also runs baseline integration/fault/smoke and a short two-replica benchmark before final integration; see the [checklist](docs/team/integration.md). Do not weaken tests or invent performance/Kaggle results. Kaggle verification needs Save & Run All evidence. Documentation-only work should validate links, consistency and that protected code is unchanged; it does not require modifying code or running the load suite.

Complete the assigned [handoff template](docs/team/handoff-template.md) with base/head SHA, constructors/ports, SQL, contract/config deltas, actual tests and limitations. Do not push main or merge service branches yourself. Commit/push a service branch only when the task owner requests it in that session.

Never commit .env, caches, binary/data directories, JWTs or logs. Preserve existing `work/` files. Run explicit reviewed migrations, never auto-migrate from each replica.
