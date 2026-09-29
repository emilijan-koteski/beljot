.PHONY: dev build test test-client test-server lint lint-client lint-server migrate seed

-include .env
export

dev:
	docker compose up -d
	@echo "PostgreSQL on :5433, Garage S3 on :3900 (bucket web on :3902)"
	npx -y concurrently -k -n client,server -c blue,green \
		"cd client && npm run dev" \
		"cd server && go run -tags nodynamic ./cmd/api"

build:
	cd client && npm run build
	cd server && go build -tags nodynamic -o bin/api ./cmd/api

test: test-client test-server

test-client:
	cd client && npx vitest run

# nodynamic: exercise the same pure-Go WebP encoder the image ships (see
# server/Dockerfile), never a system libwebp the machine happens to have.
test-server:
	cd server && go test -tags nodynamic ./...

lint: lint-client lint-server

lint-client:
	cd client && npx tsc --noEmit && npx eslint . && npx prettier --check .

lint-server:
	cd server && golangci-lint run ./...

migrate:
	migrate -path server/migrations -database "$(BELJOT_DB_URL)" up

seed:
	@echo "Seed script not yet implemented"
