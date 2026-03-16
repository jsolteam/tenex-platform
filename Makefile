local-up:
	docker compose -f deploy/compose/local.yml --env-file .env up --build

local-down:
	docker compose -f deploy/compose/local.yml down

local-infra:
	docker compose -f deploy/compose/local.yml --env-file .env up --build postgres redis minio loki tempo otel-collector

dev-up:
	@cp -n .env.dev .env 2>/dev/null || true
	docker compose -f deploy/compose/dev.yml --env-file .env up --build

dev-down:
	docker compose -f deploy/compose/dev.yml down

dev-logs:
	docker compose -f deploy/compose/dev.yml logs -f bot scheduler worker

prod-up:
	@if [ -z "$$IMAGE_TAG" ]; then \
		echo "ERROR: IMAGE_TAG is not set. Use: IMAGE_TAG=<sha> make prod-up"; \
		exit 1; \
	fi
	IMAGE_TAG=$$IMAGE_TAG docker compose -f deploy/compose/prod.yml pull
	IMAGE_TAG=$$IMAGE_TAG docker compose -f deploy/compose/prod.yml up -d

prod-down:
	docker compose -f deploy/compose/prod.yml down

# ── Migrations ────────────────────────────────────────────────────────────────

# Run migrations inside Docker (postgres must be running via local-infra or local-up)
migrate-docker-up:
	docker compose -f deploy/compose/local.yml --env-file .env run --rm migrate up

migrate-docker-status:
	docker compose -f deploy/compose/local.yml --env-file .env run --rm migrate status

# Run migrations from host machine (reads .env.local which uses localhost)
# Requires: make local-infra (postgres must be up and port-forwarded)
migrate-up:
	go run ./cmd/migrate -env-file .env.local up

migrate-status:
	go run ./cmd/migrate -env-file .env.local status

migrate-validate:
	go run ./cmd/migrate -env-file .env.local validate

# Create a new migration file: make migration NAME=add_something
migration:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migration NAME=<name>"; exit 1; fi
	@NEXT=$$(printf "%04d" $$(ls internal/infrastructure/database/migrations/*.sql 2>/dev/null | wc -l | tr -d ' \t')); \
	FILE="internal/infrastructure/database/migrations/$${NEXT}_$(NAME).sql"; \
	printf "-- Migration %s: %s\n\n" "$$NEXT" "$(NAME)" > "$$FILE"; \
	echo "Created $$FILE"

# ── Tests ─────────────────────────────────────────────────────────────────────

test:
	go test ./... -v -race

test-short:
	go test ./... -short -race

# ── Build ─────────────────────────────────────────────────────────────────────

build:
	go build ./cmd/bot
	go build ./cmd/scheduler
	go build ./cmd/worker
	go build ./cmd/migrate