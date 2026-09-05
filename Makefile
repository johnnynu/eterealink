ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: fmt test backend-test frontend-test frontend-install run frontend-run frontend-build migrate-up migrate-down containers-build containers-up containers-down phase8-deploy phase8-verify

fmt:
	cd backend && gofmt -w $$(find . -name '*.go' -type f)

test: backend-test frontend-test

backend-test:
	cd backend && go test ./...

frontend-test:
	cd frontend && npm run lint && npm test

frontend-install:
	cd frontend && npm install

run:
	cd backend && go run ./cmd/api

frontend-run:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

migrate-up:
	cd backend && go run ./cmd/migrate up

migrate-down:
	cd backend && go run ./cmd/migrate down

containers-build:
	docker compose build

containers-up:
	docker compose up --build

containers-down:
	docker compose down

phase8-deploy:
	./scripts/deploy-phase8.sh

phase8-verify:
	./scripts/verify-phase8.sh
