.PHONY: fmt test backend-test frontend-test frontend-install run frontend-run frontend-build migrate-up migrate-down

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
