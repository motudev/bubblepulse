BINARY        := tmp/main
MIGRATION_DIR := internal/db/migrations
GOBIN         := $(shell go env GOPATH)/bin

export PATH := $(GOBIN):$(PATH)

.PHONY: install dev-backend dev-frontend \
        db-up db-down db-reset \
        migrate-up migrate-down migrate-status \
        test test-integration test-system build

# ── Setup ─────────────────────────────────────────────────────────────────────

install:
	go get github.com/sugarme/tokenizer@master
	go get golang.org/x/oauth2 github.com/coreos/go-oidc/v3/oidc github.com/joho/godotenv
	go mod tidy
	go mod download
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/air-verse/air@latest
	cd frontend && npm install

# ── Database (Docker) ─────────────────────────────────────────────────────────

db-up:
	docker compose up -d --wait

db-down:
	docker compose down

db-reset:
	docker compose down -v

# ── Development ───────────────────────────────────────────────────────────────

dev-backend:
	docker compose up -d nlp_service
	air -c .air.toml

dev-frontend:
	cd frontend && npm run dev

# ── Database Migrations ───────────────────────────────────────────────────────

migrate-up:
	goose -dir $(MIGRATION_DIR) postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir $(MIGRATION_DIR) postgres "$(DATABASE_URL)" down

migrate-status:
	goose -dir $(MIGRATION_DIR) postgres "$(DATABASE_URL)" status

# ── Testing ───────────────────────────────────────────────────────────────────

test:
	go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...
	cd frontend && npm run test

test-integration:
	go test -race -count=1 -tags=integration ./internal/db/...

test-system:
	go test -race -count=1 -tags=system ./test/...

# ── Build ─────────────────────────────────────────────────────────────────────

build:
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/bubblepulse
