# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Language

All user-facing responses must be written in Chinese (see AGENTS.md). Code identifiers and Conventional Commit subjects stay in English, matching existing history.

## Commands

Run from the repository root:

- `make check` — frontend type-check + ESLint + mock contract guard, `gofmt` verification, `go vet ./...`
- `make test` — all Go tests (`go test ./...`)
- `make test-mock` — mock contract guard: validates dev-mock responses against `api/openapi.yaml` via Vitest/ajv (`web/tests/mock-contract.test.ts`); guards against mock/contract drift (opaque-`Id` length is intentionally out of scope)
- `go test -race ./...` — required to pass before merging
- `go test ./internal/<pkg> -run TestName` — run a single Go test
- `make test-contract` — API contract tests: boots the real binary and validates every request/response against `api/openapi.yaml` (`tests/contract/`, self-building)
- `make e2e` — Playwright E2E over the embedded-frontend binary (`tests/e2e/`); builds first, then runs guest/user/admin key paths. `make e2e-install` provisions Playwright + Chromium once
- `make frontend` — `npm ci` + Vite production build to `web/out/`
- `make embed` — copy `web/out/` into `internal/webui/dist/` for Go embedding
- `make build` — frontend + embed + static `bin/navax` binary (CGO disabled)
- `go run ./cmd/navax` — run the service locally (env vars per `.env.example`; data dir defaults to `./data`)
- `docker compose up --build` — production-style container; first-run setup at `/setup` with `NAVAX_SETUP_TOKEN`

Frontend-only development: `cd web && npm run dev` (Vite, port 3000). There is no dev proxy to the Go backend — set `VITE_ENABLE_API_MOCKS=true` to install the fetch-intercepting mock API (`web/src/api/mock-handlers.ts`, dev-only). Production code must never depend on `web/src/mocks/`. The mock projects its internal store to the same contract shape as the real backend; when adding or changing mock responses, keep them passing `make test-mock`.

Every change must pass `make check`, `go test -race ./...`, and `make build`. UI changes also require a browser smoke test of loading, empty, error, mobile, keyboard, and dark-theme states.

## Architecture

nav.ax is a personalized navigation-site service: a single Go 1.25 process serves the REST API, public navigation pages, admin UI, SQLite storage, and the embedded React SPA. Design docs (Chinese): `docs/architecture.md`, `docs/requirements.md`.

### Backend (Go modular monolith)

- HTTP: stdlib `net/http` + chi. `internal/httpapi/` owns routing, DTOs, auth/permission middleware, and serialization **only** — business logic and transaction boundaries belong to the domain packages under `internal/` (auth, navigation, subdomains, analytics, catalog, linkcheck, maintenance, ...). `internal/app/` wires everything together.
- Database: `database/sql` + `modernc.org/sqlite` (pure Go, no CGO). WAL mode, foreign keys, short write transactions. `internal/database/` must not leak SQLite-specific types upward. Migrations in `migrations/` are embedded, sequential, append-only SQL, applied at startup under a process lock.
- API contract: `api/openapi.yaml` is the single source of truth. Update it whenever an endpoint contract changes; Go DTOs and frontend types must conform to it. `tests/contract/` enforces this at CI time (see below), so a contract change that isn't reflected in the spec fails the build.
- Publish model: edits produce drafts; publishing writes an immutable JSON snapshot and flips the current pointer in one transaction. Public requests never read draft tables and are served with ETag/Cache-Control.
- Subdomains: 4+ character names auto-approve; 1–3 character names go to `pending` for admin review. Reserved-word and uniqueness checks come first.
- Deliberately absent: ORM, DI framework, event bus, Redis, queues, PostgreSQL. Do not introduce them.

### Security invariants (preserve these)

- Sessions use Host-only, HttpOnly, SameSite=Lax cookies; `Origin` is validated on all non-safe methods; login, invites, events, and link checks are rate-limited separately.
- Session/invite/recovery tokens are stored only as hashes; third-party secrets are encrypted with `NAVAX_MASTER_KEY` and never returned to clients.
- SSRF: server-side URL fetching rejects loopback, private, link-local, reserved, and cloud-metadata addresses on every DNS resolution and redirect.
- Uploads restrict MIME type and size; SVG is rejected by default. Full IPs are never persisted; visitor IDs use a daily-rotating HMAC.

### Frontend (`web/`, React 19 + Vite + TypeScript)

- Tailwind, Radix UI, TanStack Query, react-router v7, i18next, Recharts; two-space indentation; `@/` aliases `web/src/`; Vite outputs to `web/out/` (not `dist/`).
- Auto-imports: React hooks, react-router-dom hooks/components, and `useTranslation`/`Trans` are auto-imported via `unplugin-auto-import` (see `auto-imports.d.ts`) — do not add manual imports for them.
- All HTTP calls go through `web/src/api/` (shared client in `client.ts` plus per-domain typed modules). Do not bypass the OpenAPI contract.
- Themes are TypeScript packages under `web/src/themes/packages/`, wired through `registry.ts`/`manifest.ts`.
- Pages live under `web/src/pages/` grouped by surface (public home, app, admin, discover, setup, login, invite); routes are configured in `web/src/router/config.tsx` (a custom ESLint rule constrains route `element` JSX).

### Tests (`tests/`)

- `tests/contract/` (Go): compiles and boots the binary once, drives a bootstrap→auth→edit→publish→public-read→admin flow, and validates each request+response against `api/openapi.yaml` via `libopenapi-validator`. Steps share state and run in order. `-short` skips it. This is where "does the implementation match the spec" is enforced — extend it, and the spec, together.
- `tests/e2e/` (Playwright, separate npm project — not part of the `web/` build): a Go-less Node project. `global.setup.ts` seeds accounts and the published system page via the API and saves per-role storage states; specs (`guest`/`user`/`admin`) exercise the UI against the real binary launched by `server.mjs` in a fresh temp data dir. Test files may not import each other — shared constants live in `specs/accounts.ts`.

## Conventions

- Conventional Commit subjects, e.g. `feat: add signed instance backups`.
- Go: gofmt-formatted, small domain-focused packages; table-driven tests named `TestFeatureCondition` placed beside source as `*_test.go`; prefer SQLite integration tests for persistence or authorization behavior; add regression tests with bug fixes.

## Agent notes (from AGENTS.md)

- "KB" means the MCP `omni` knowledge base, never a repo `docs/wiki` mirror. Before writing to it: call `whoami`, consult current principles, preserve provenance, and attach non-hub pages to their hub.
