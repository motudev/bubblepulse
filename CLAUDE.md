# BubblePulse — Coding Standards for Claude

## Project Identity

- **Module:** `github.com/motudev/bubblepulse`
- **Go version:** 1.22 (use enhanced `ServeMux` method+path routing, `slices` package)
- **Frontend:** Vue 3 + Vite + TypeScript 5 + Pinia

---

## Go Standards

### Errors
- Never use `panic` outside `main` packages; always return `error`
- Define named sentinel errors (`var ErrX = errors.New(...)`) at the package level
- Collect all validation failures before returning — don't short-circuit on the first

### Dependencies & Interfaces
- No global mutable state; inject all dependencies through constructors
- Define interfaces **in the consumer package**, not in the implementation package (Effective Go)
- Keep the interface as small as possible — only what the consumer actually calls

### Database
- All SQL queries use parameterized inputs (`$1`, `$2`, …) — never string interpolation
- Use `pgx/v5` directly; do not wrap in `database/sql`

### Code Style
- Exported types and functions must have godoc comments (one-line minimum)
- Named constants for all magic numbers and string literals used more than once
- No `utils.go` catch-alls — one file per single responsibility
- Table-driven tests with `t.Run(tc.name, ...)` for all unit tests

### Test Separation (ISTQB)
- Unit tests: co-located `*_test.go`, no build tag, no external dependencies
- Integration tests: `//go:build integration`, require `TEST_DATABASE_URL`
- System tests: `//go:build system`, in `test/system/`, require full stack

---

## Vue / TypeScript Standards

### Components
- `<script setup lang="ts">` only — no Options API, no `defineComponent`
- `defineProps<T>()` with explicit TypeScript interfaces
- No `any` types anywhere

### State
- Pinia stores use the **composition style** (`defineStore('id', () => { ... })`)
- Derived state goes in `computed()`; side effects go in `watch()` or async actions

### Services
- All HTTP calls go through `src/services/api.ts` — no raw `fetch` in components
- API functions return typed promises; throw on non-ok responses

---

## Migration Conventions

- Use **goose** format: `-- +goose Up` / `-- +goose Down` sections
- File naming: `NNN_description.sql` (e.g., `001_create_teams.sql`)
- One table per migration file
- Every migration must be reversible (`Down` section required)
- Named constraints preferred over anonymous ones for clear error handling
