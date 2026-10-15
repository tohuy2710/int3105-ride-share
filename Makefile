GO ?= go
SERVICES := ride location matching notification

.PHONY: build test vet compose-up compose-down $(addprefix run-,$(SERVICES))

build:
	@mkdir -p bin
	@set -e; for service in $(SERVICES); do $(GO) build -buildvcs=false -o bin/$$service ./cmd/$$service; done

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

$(addprefix run-,$(SERVICES)): run-%:
	$(GO) run ./cmd/$*

compose-up:
	docker compose up --build -d --wait

compose-down:
	docker compose down
