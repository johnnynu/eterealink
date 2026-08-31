.PHONY: fmt test run migrate-up migrate-down

fmt:
	cd backend && gofmt -w $$(find . -name '*.go' -type f)

test:
	cd backend && go test ./...

run:
	cd backend && go run ./cmd/api

migrate-up:
	cd backend && go run ./cmd/migrate up

migrate-down:
	cd backend && go run ./cmd/migrate down

