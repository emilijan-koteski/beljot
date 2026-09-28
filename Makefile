.PHONY: dev build test test-client test-server lint lint-client lint-server migrate seed

-include .env
export

dev:
	docker compose up -d
	@echo "PostgreSQL on :5433, Garage S3 on :3900 (bucket web on :3902)"
	npx -y concurrently -k -n client,server -c blue,green \
		"cd client && npm run dev" \
		"cd server && go run ./cmd/api"

build:
	cd client && npm run build
	cd server && go build -o bin/api cmd/api/main.go

test: test-client test-server

test-client:
	cd client && npx vitest run

test-server:
	cd server && go test ./...

lint: lint-client lint-server

lint-client:
	cd client && npx tsc --noEmit && npx eslint . && npx prettier --check .

lint-server:
	cd server && golangci-lint run ./...

migrate:
	migrate -path server/migrations -database "$(BELJOT_DB_URL)" up

seed:
	@echo "Seed script not yet implemented"
