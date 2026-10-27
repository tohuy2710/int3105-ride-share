# Notification service — Person 3

Read the [root instructions](../../AGENTS.md), [team ownership](../../docs/team/README.md), [requirements](../../docs/team/notification-requirements.md) and [copyable agent prompt](../../docs/team/notification-agent.md) before working.

Your delivery is a real application + PostgreSQL repository + private SQL + tests, not global runtime/transport integration. Own new implementation files inside `internal/notification/` and your [handoff](../../docs/team/handoffs/notification.md). Do not edit instruction/requirements documents. Preserve the current Service/MockService interfaces and router/handler/DTO behavior so baseline fixtures still work; add independent real models/ports instead of breaking shared callers.

Implement durable QUEUED requests in schema notification, unique by (rideId,driverId,type), with an application read operation by ID. No push provider, delivery worker, broker adapter or SENT/DELIVERED status. Preserve notification_123 only in the existing mock.

`cmd/notification`, shared contracts/clients/platform/OpenAPI/tests, root migrations, dependency/environment files, Ride/Auth and benchmark belong to the integrator. Do not switch the default runtime or edit those paths. Put proposed SQL in `internal/notification/postgres/migrations/001_init.sql` and test it in a dedicated service test DB; do not auto-migrate on application startup.

Follow the required unit/PostgreSQL/race checks and fill the handoff with constructor/port/SQL/contract delta and actual test evidence. The [integrator](../../docs/team/integration.md) publishes shared contracts and wires the application after review. Do not modify the race baseline or weaken tests; do not spawn additional agents.
