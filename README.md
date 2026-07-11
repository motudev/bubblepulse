# BubblePulse

An open-source, self-hosted asynchronous check-in platform. Eliminate unnecessary status meetings while keeping teams aligned through a visual dependency graph powered by daily Focus/Friction/Energy updates.

## Requirements

- Go 1.22+
- Node.js 20+
- Docker (for the database)

## Quick Start

```bash
# 1. Clone and configure environment
cp .env.example .env

# 2. Start the database
make db-up

# 3. Install all dependencies and tools
make install

# 4. Apply database migrations
make migrate-up

# 5. Start development servers (run in separate terminals)
make dev-backend
make dev-frontend
```

The backend runs on `http://localhost:8080` and the frontend on `http://localhost:5173`.

The default `.env.example` values are pre-configured to match the Docker Compose database — no edits required for local development.

## Project Structure

```
cmd/bubblepulse/     Entry point — composition root
internal/api/     HTTP handlers and middleware
internal/db/      Database connection and migrations
internal/domain/  Core domain types (added per feature)
internal/service/ Business logic (added per feature)
pkg/config/       Environment configuration
frontend/         Vue 3 + Vite SPA
test/system/      End-to-end system tests
```

## Available Commands

| Command | Description |
|---|---|
| `make install` | Install backend + frontend deps and CLI tools |
| `make db-up` | Start the PostgreSQL container (Docker) |
| `make db-down` | Stop the PostgreSQL container |
| `make db-reset` | Stop and delete all data (drops volumes) |
| `make dev-backend` | Run Go server with live reload (Air) |
| `make dev-frontend` | Run Vite dev server |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the last migration |
| `make migrate-status` | Show migration status |
| `make test` | Run full test suite with coverage |
| `make build` | Build production binary |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Project Vision

See [PROJECT_GOAL.md](PROJECT_GOAL.md).
