SHELL := /bin/sh
.PHONY: up down test api web
up:
	docker compose -f infra/compose.yaml up --build -d

down:
	docker compose -f infra/compose.yaml down

test:
	cd api && go test ./...
	cd web && npm run build
	cd mobile && flutter analyze && flutter test

api:
	cd api && go run ./cmd/server

web:
	cd web && npm run dev
