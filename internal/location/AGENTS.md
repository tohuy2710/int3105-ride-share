# Location service — Person 1

Read the [root instructions](../../AGENTS.md), [team ownership](../../docs/team/README.md), [requirements](../../docs/team/location-requirements.md) and [copyable agent prompt](../../docs/team/location-agent.md) before working.

Your delivery is a real application + PostgreSQL repository + private SQL + tests, not global runtime/transport integration. Own new implementation files inside `internal/location/` and your [handoff](../../docs/team/handoffs/location.md). Do not edit instruction/requirements documents. Preserve the current Service/MockService interfaces and router/handler/DTO behavior so baseline fixtures still work; add independent real models/ports instead of breaking shared callers.

Implement real latest-location/availability persistence and Haversine nearby queries in schema location. Preserve zero coordinates, false availability and independent column updates. No TTL/PostGIS, Redis cache or calls to other services.

`cmd/location`, shared contracts/clients/platform/OpenAPI/tests, root migrations, dependency/environment files, Ride/Auth and benchmark belong to the integrator. Do not switch the default runtime or edit those paths. Put proposed SQL in `internal/location/postgres/migrations/001_init.sql` and test it in a dedicated service test DB; do not auto-migrate on application startup.

Follow the required unit/PostgreSQL/race checks and fill the handoff with constructor/port/SQL/contract delta and actual test evidence. The [integrator](../../docs/team/integration.md) publishes shared contracts and wires the application after review. Do not modify the race baseline or weaken tests; do not spawn additional agents.
