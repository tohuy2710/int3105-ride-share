GO ?= go
PYTHON ?= python3
-include .env
export
export GOCACHE := $(CURDIR)/.cache/go-build
export GOMODCACHE := $(CURDIR)/.cache/go-mod
SERVICES := ride location matching notification auth
COMMANDS := $(SERVICES) migrate seed benchmark

.PHONY: build test test-race integration vet fmt fmt-check contract compose-up compose-down infra-up migrate seed smoke benchmark benchmark-suite $(addprefix run-,$(SERVICES))
build:
	@mkdir -p bin
	@set -e; for service in $(COMMANDS); do $(GO) build -buildvcs=false -o bin/$$service ./cmd/$$service; done

test:
	$(GO) test ./...
test-race:
	$(GO) test -race ./...
integration:
	$(GO) test -race -tags=integration ./internal/integration -count=1
vet:
	$(GO) vet ./...
fmt:
	$(GO) fmt ./...
fmt-check:
	$(PYTHON) scripts/check_format.py
contract:
	$(GO) test ./internal/contracttest -count=1
$(addprefix run-,$(SERVICES)): run-%:
	$(GO) run ./cmd/$*
infra-up:
	docker compose --profile infra up -d --wait
compose-up:
	docker compose --profile full up --build -d --wait
compose-down:
	docker compose --profile infra --profile full down
migrate:
	$(GO) run ./cmd/migrate
seed:
	$(GO) run ./cmd/seed
smoke:
	$(PYTHON) scripts/smoke.py
benchmark:
	$(GO) run ./cmd/benchmark
benchmark-suite: build
	$(PYTHON) scripts/benchmark_suite.py
