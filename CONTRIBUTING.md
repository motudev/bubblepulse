# Contributing to BubblePulse

Thank you for your interest in contributing. BubblePulse is an open-source project and contributions of all kinds are welcome.

## Getting Started

1. Fork the repository
2. Create a feature branch from `main`: `git checkout -b feat/your-feature`
3. Make your changes, following the coding standards in [CLAUDE.md](CLAUDE.md)
4. Ensure all tests pass: `make test`
5. Open a Pull Request against `main`

## Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
type(scope): short description

Optional longer body explaining WHY, not what.
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`

Examples:
```
feat(api): add POST /api/v1/updates endpoint
fix(db): handle unique constraint violation on team registration
docs(readme): add docker-compose quick-start
```

## Running Tests

```bash
# Unit tests only (no database required)
make test

# Integration and system tests require the database to be running:
make db-up
make test-integration   # requires TEST_DATABASE_URL in .env
make test-system        # requires full stack
```

## Database Migrations

When adding a new migration:

1. Create a file in `internal/db/migrations/` with the next sequence number
2. File name format: `NNN_description.sql` (e.g., `006_add_reactions_table.sql`)
3. Always include both `-- +goose Up` and `-- +goose Down` sections
4. Test both `make migrate-up` and `make migrate-down` before opening a PR

## Code of Conduct

Be respectful and constructive. We follow the [Contributor Covenant](https://www.contributor-covenant.org/) Code of Conduct.
