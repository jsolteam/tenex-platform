local-up:
	docker compose -f deploy/compose/local.yml up --build

local-down:
	docker compose -f deploy/compose/local.yml down

dev-up:
	docker compose -f deploy/compose/dev.yml up --build

prod-up:
	docker compose -f deploy/compose/prod.yml up -d