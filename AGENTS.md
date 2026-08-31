# Repository Guidelines

Architecture and security invariants: `docs/architecture.md`. Product scope: `docs/requirements.md`.

nav.ax is a Go service with an embedded React SPA. `internal/httpapi/` owns routing, DTOs, middleware, and serialization only — business logic stays in domain packages. `api/openapi.yaml` is the contract. Vite writes `web/out/`; `make embed` copies it to `internal/webui/dist/`. Do not introduce ORM, DI, event bus, Redis, queues, or PostgreSQL.

## Commands

Merge gates: `make check`, `go test -race ./...`, `make build`. Also run `make test-contract` and `make test-mock` for contract or mock changes, and `make e2e` plus a browser smoke test (loading, empty, error, mobile, keyboard, dark theme) for UI. Local binary: `go run ./cmd/navax`.

Frontend-only: `cd web && npm run dev` with `VITE_ENABLE_API_MOCKS=true` (no Go proxy). Production code must never depend on `web/src/mocks/`.

## Style

Go: `gofmt`; table-driven `*_test.go` next to source; SQLite integration tests for persistence and auth; regression tests with bug fixes. React: auto-import hooks, react-router, and `useTranslation`/`Trans` (`web/auto-imports.d.ts`) — do not add those imports by hand. API calls go through `web/src/api/`. Routes live in `web/src/router/config.tsx`. Themes are server-compiled (`internal/themes`); do not put theme CSS strings in the SPA.

## Shipping

Do not invent commits or PRs. If the user only asked to implement or fix, leave the work uncommitted. When they ask to 提交 / 合并 / 上线 / 发布 / 部署生产 / ship / merge / deploy, complete the path without reconfirming each step:

1. Branch (`fix/…`, `feat/…`) — never commit on `main`.
2. Verify the gates that apply; fix failures first.
3. Commit related files only. Conventional Commit subject in English.
4. `git push -u origin HEAD`, `gh pr create`, `gh pr merge --auto --rebase`. Wait for `verify` / `e2e` / `container`. Do not force-push `main`.
5. Official `nav.ax` CD runs after `main` is green (`deploy/README.md`). Use `npc deploy navax production --ref main --wait` only if CD failed or the user asked for an out-of-band release.
6. Report the PR URL, merge status, and whether production CD ran.

Need an explicit ask before: force-push or history rewrite; deleting remotes beyond normal PR head cleanup; changing GitHub secrets, branch rules, or production env vars; out-of-band production deploy; committing secrets or `.env` files.

## Language and security

User-facing replies in Chinese. Identifiers and commit subjects in English. Keep secrets out of source and browser storage. Preserve SSRF, upload, origin, and rate-limit protections.
