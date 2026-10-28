---
name: run-ride-share
description: Start, stop, and verify the local int3105-ride-share Go/Gin project, including Docker Compose, host development services, Swagger UI, and isolated runtime checks. Use for running or troubleshooting this project locally, not production deployment or enabling unfinished service implementations.
---

# Run Ride Share

Run the project and verify the requested result. This repository skill is stored in `.codex/skills/run-ride-share`. Starting or stopping the project does not require adding workflow transcripts or changing Git history.

## Locate and inspect

Use the current checkout when its `go.mod` declares `module int3105-ride-share`. Otherwise locate the repository containing this skill by walking up from its directory to the matching `go.mod`; ask for the checkout location only if neither matches.

Read the checkout's `README.md`, `Makefile`, `compose.yaml`, `.env.example`, and `docs/development.md` as needed. Their current configuration takes precedence over the defaults below. Check Git status and preserve existing changes and `work/`; starting the project needs no commit, push, reset, or branch change.

Default to Docker Compose for an ordinary request to run the project. Use host Go processes when the user wants Go debugging, and the isolated native runner when they want verification rather than a stack left running. Do not run every mode automatically.

## Current runtime

| Service | Default host port | Runtime |
| --- | --- | --- |
| Ride | 8081 | PostgreSQL rides; Redis contention gate; fare mock |
| Location | 8082 | Mock HTTP endpoints |
| Matching | 8083 | Mock HTTP endpoints |
| Notification | 8084 | Mock HTTP endpoints |
| Auth | 8085 | Fixture credentials and signed JWT; register mock |

Location/Matching/Notification also contain real application/repository code, but their default entrypoints do not use it. RabbitMQ has topology only. Do not enable new wiring, run private service SQL against the development DB, change API contracts, or invent pickup coordinates merely to start the project. ACCEPT must retain its PostgreSQL conditional transaction; Redis is only a contention gate.

## Start with Docker Compose

1. Check `docker info` and `docker compose version`. If the daemon is unavailable, report that specific blocker or use an appropriate installed host workflow; do not describe a completed one-shot check as a running stack.
2. Create `.env` from `.env.example` only if `.env` is absent. Preserve existing configuration. Avoid printing secrets or dumping resolved Compose environment.
3. Check whether this Compose project already has running containers (`docker compose --profile full ps`). Reuse them or rebuild this project's services as needed. If a port belongs to an unrelated process, choose a free host port through configuration rather than stopping that process.
4. From the repository root run:

   ```bash
   make compose-up
   ```

   This builds five apps, starts PostgreSQL/Redis/RabbitMQ, and runs the explicit Ride migration job before Ride starts. It is not necessary to run `make migrate` again for this mode. Wait for health status; if startup fails, inspect only the relevant service's bounded logs and fix the demonstrated local startup issue.
5. Verify the stack as below and leave it running when the user asked to run it.

Use bounded tool waits and report meaningful startup progress during image builds. Do not install or upgrade dependencies, reset volumes, or change application behavior as an automatic startup repair. After a repair attempt, recheck health; if the same blocker remains, report its concrete cause instead of repeatedly restarting the stack.

## Host Go development

Use the version in `.go-version` (currently Go 1.25.9), Make, Python 3, and a compiler when running race tests. Keep Go caches under the repository's `.cache` as configured by Make.

Create `.env` only if missing, then run `make infra-up` and `make migrate` explicitly against the configured local development database. Start each required application in its own managed terminal/process, from the repository root:

```bash
PORT=8081 make run-ride
PORT=8082 make run-location
PORT=8083 make run-matching
PORT=8084 make run-notification
PORT=8085 make run-auth
```

Adapt ports to the actual configuration. Make loads/exports `.env`; `go run` and `bin/<service>` by themselves do not. Host `DATABASE_URL`/`REDIS_URL` must use published host ports; Compose's app containers use internal hostnames. Record the processes you launch so you can stop only those processes later. Keep the service processes alive after verification when a running project was requested.

## Verify startup

- On every requested service, check `GET /healthz` and `GET /readyz`, HTTP status and response body. Ride may report degraded readiness when Redis is unavailable; report that state instead of calling it fully healthy.
- Check `/swagger/index.html`, `/swagger/openapi.json`, and the bundled UI assets. Swagger's JSON `servers` should use `/` so Try it out targets the serving host and port. Open the UI in the browser when visual verification or showing it is useful.
- For an ordinary local development stack, run `make smoke` to exercise authenticated Ride/Auth and the current mock contracts. This creates and mutates fixture-owned test rides; use only a local development environment. For a user-supplied database not clearly intended for tests, verify read-only endpoints first and clarify before running mutating smoke.
- The smoke script reads service URLs, not `*_PORT`. With custom host ports, pass matching `RIDE_URL`, `LOCATION_URL`, `MATCHING_URL`, `NOTIFICATION_URL`, and `AUTH_URL` as Make command-line variables, for example `make smoke RIDE_URL=http://127.0.0.1:9081` when only Ride's host port changed. Use the configured internal token/JWT setup; do not bypass authentication.

Fixture login: `rider@example.com`, `driver1@example.com`, or `driver2@example.com`; the repository's default fixture password is `RideShare123!`. Login returns the bearer token for Swagger's Authorize control. Read fixture/config sources again if credentials were customized; avoid echoing tokens or secrets.

Report the actual service and Swagger URLs, readiness/smoke results, any remaining blocker, and whether the stack is still running. Do not claim real matching, push delivery, or service persistence from mock HTTP responses.

## Requested test or regression work

For code verification use `GOTOOLCHAIN=go1.25.9 make fmt-check vet test-race contract build`, adjusting the version if `.go-version` changes.

For isolated runtime verification, use:

```bash
GOTOOLCHAIN=go1.25.9 python3 scripts/native_check.py
```

The native runner requires matching PostgreSQL tools (`postgres`, `initdb`, `pg_ctl`, `createdb`) plus Redis tools on PATH. It builds apps, owns fresh PostgreSQL/Redis instances, creates service test DBs and explicit DSNs, runs race/integration/fault suites, checks Swagger and HTTP smoke, and stops its processes when finished. Logs/data remain under ignored `.cache/native-check/`. It does not start RabbitMQ or leave a development stack running. Default ports are 15432 (PostgreSQL), 16379 (Redis), and 18081–18085 (apps). Inspect port availability first; infrastructure ports can be changed with `NATIVE_POSTGRES_PORT`/`NATIVE_REDIS_PORT`, but app ports in the current runner are fixed.

For an explicitly requested short acceptance benchmark:

```bash
GOTOOLCHAIN=go1.25.9 python3 scripts/benchmark_suite.py \
  --concurrency 2,10,50,100 --groups 10 --warmup 2 --repeats 2 --no-plots
```

The benchmark owns isolated dependencies and two Ride replicas per mode. It also uses ports 18081/19081/18085, so do not run it concurrently with the native runner. Report measured correctness/errors from its artifacts; do not invent performance improvements or Kaggle evidence.

## Stop

For a stack this task started, use `make compose-down` when asked to stop it. This preserves named volumes. Do not add `-v`, prune Docker resources, or delete existing database directories as ordinary cleanup. Stop only owned host processes; do not use broad process-name kills. Leave previously running services alone unless the user asks to stop them.
