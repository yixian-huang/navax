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

`main` is protected: direct pushes are rejected for everyone, admins included. Push your work to a branch, open a PR (`gh pr create`), and enable auto-merge (`gh pr merge --auto --rebase`); it merges once the `verify`, `e2e`, and `container` checks pass with the branch up to date. Run the merge gates locally before pushing.

## Agent shipping workflow (commit / PR / production)

Agents **should not** invent commits or open PRs for unfinished work. Once the requested change is complete, verified, and the user asks to ship (or uses phrases like 提交、合并、上线、发布、部署生产、ship、merge、deploy), do the full path without asking again for each step:

1. **Branch** — If on `main` with local changes, create a focused branch (`fix/…`, `feat/…`). Never commit directly on `main`.
2. **Verify** — Run the merge gates that apply (`make check`, `go test -race ./...`; contract/e2e/build when the change touches those surfaces). Fix failures before committing.
3. **Commit** — Stage only related files; Conventional Commit subject in English; body in complete sentences when useful. Do not amend published commits unless explicitly asked.
4. **PR + auto-merge** — `git push -u origin HEAD`, `gh pr create` (summary + test plan), then `gh pr merge --auto --rebase`. Wait for CI green and merge; do not force-push `main`.
5. **Production deploy** — Official `nav.ax` CD is automatic: after merge to `main`, the `deploy-production` job runs when `verify` / `e2e` / `container` are green (see `deploy/README.md`). No separate manual deploy step unless CI/CD failed or the user asks for an out-of-band release (`npc deploy navax production --ref main --wait`).
6. **Report** — Reply with the PR URL, merge status, and whether production CD was triggered or needs attention.

Still require an **explicit user request** before:

- Force-push, hard reset, or rewriting shared history
- Deleting branches/tags or destructive remote ops beyond the normal PR head cleanup
- Changing GitHub secrets, protected-branch rules, or production env vars
- Manual production deploy when auto-CD was not requested and the user only asked for a local fix
- Committing secrets or `.env` files

If the user only asks to implement/fix something and does **not** mention shipping, leave changes uncommitted (or commit only if they later ask). Prefer one ship path end-to-end over stopping after “code done” when they already said 提交合并/部署.

## Security & Agent Instructions

Keep secrets out of source and browser storage; sessions use HttpOnly cookies. Validate authorization server-side and preserve SSRF, upload, origin, and rate-limit protections. All user-facing agent responses must be written in Chinese.
