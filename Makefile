local-up:
	docker compose -f deploy/compose/local.yml up --build

local-down:
	docker compose -f deploy/compose/local.yml down

dev-up:
	docker compose -f deploy/compose/dev.yml up --build

dev-down:
	docker compose -f deploy/compose/dev.yml down

prod-up:
	@if [ -z "$$IMAGE_TAG" ]; then \
		echo "ERROR: IMAGE_TAG is not set. Use: IMAGE_TAG=<sha> make prod-up"; \
		exit 1; \
	fi
	IMAGE_TAG=$$IMAGE_TAG docker compose -f deploy/compose/prod.yml pull
	IMAGE_TAG=$$IMAGE_TAG docker compose -f deploy/compose/prod.yml up -d

prod-down:
	docker compose -f deploy/compose/prod.yml down