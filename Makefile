local-up:
	docker compose -f deploy/compose/local.yml --env-file .env up --build

local-down:
	docker compose -f deploy/compose/local.yml down

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

# Тесты
test:
	go test ./... -v -race

test-short:
	go test ./... -short -race

build:
	go build ./cmd/bot
	go build ./cmd/scheduler
	go build ./cmd/worker