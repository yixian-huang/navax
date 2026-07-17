# Repository Guidelines

## Project Structure & Module Organization

nav.ax is a Go service with an embedded React SPA. `cmd/navax/` contains the executable entry point. Backend features are grouped by domain under `internal/`; HTTP handlers live in `internal/httpapi/`, database migrations in `migrations/`, and the API contract in `api/openapi.yaml`. Frontend routes, components, hooks, and API adapters live under `web/src/`. Vite outputs to `web/out/`; `make embed` copies that bundle into `internal/webui/dist/` for Go embedding. Product decisions are documented in `docs/requirements.md` and `docs/architecture.md`.

## Build, Test, and Development Commands

Run from the repository root:

- `make check` runs TypeScript checks, ESLint, `gofmt` verification, and `go vet`.
- `make test` runs all Go tests.
- `make frontend` creates the production SPA bundle.
- `make build` builds the frontend, embeds it, and writes `bin/navax`.
- `go run ./cmd/navax` starts the service with local environment settings.
- `docker compose up --build` starts the production-style container.

For frontend-only work, use `cd web && npm run dev`.

## Coding Style & Naming Conventions

Format Go with `gofmt`; keep packages small and domain-focused. Exported Go names use `PascalCase`, internal names use `camelCase`, and tests use `TestFeatureCondition`. React code uses TypeScript, functional components, two-space indentation, `PascalCase` component files, `useXxx` hooks, and the `@/` import alias. Keep API calls in `web/src/api/`; do not bypass the OpenAPI contract or add production dependencies on `web/src/mocks/`.

## Testing Guidelines

Place Go tests beside source as `*_test.go`; prefer table-driven unit tests and SQLite integration tests for persistence or authorization behavior. Every change must pass `make check`, `go test -race ./...`, and `make build`. Endpoint contract changes must also pass `make test-contract` (boots the real binary and validates against `api/openapi.yaml`), and UI/flow changes `make e2e` (Playwright over the embedded-frontend binary; see `tests/e2e/`). UI changes also require a browser smoke test of loading, empty, error, mobile, keyboard, and dark-theme states. Add regression tests for bug fixes.

## Commit & Pull Request Guidelines

Use focused Conventional Commit subjects, for example `feat: add signed instance backups` or `fix: reject private link-check targets`. Pull requests must describe user-visible behavior, linked issues, migrations or configuration changes, and verification performed. Include screenshots for UI changes and update `api/openapi.yaml` when an endpoint contract changes.

## Security & Agent Instructions

Keep secrets out of source and browser storage; sessions use HttpOnly cookies. Validate authorization server-side and preserve SSRF, upload, origin, and rate-limit protections. All user-facing agent responses must be written in Chinese.
