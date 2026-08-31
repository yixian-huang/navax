# nav.ax

[中文](README.md) | English

nav.ax is a personalized navigation-site service built with Go and React, for individuals, invited users, and self-hosting. A single Go process serves the REST API, public navigation pages, the admin UI, SQLite storage, and the embedded frontend.

## Quick start (Docker Compose)

Requires Docker 24+ with Compose v2.

```bash
cp .env.example .env
# For production, write two independent random secrets
sed -i.bak "s/^NAVAX_SETUP_TOKEN=$/NAVAX_SETUP_TOKEN=$(openssl rand -hex 32)/" .env
sed -i.bak "s|^NAVAX_MASTER_KEY=$|NAVAX_MASTER_KEY=$(openssl rand -base64 32)|" .env
docker compose up -d --build
docker compose logs -f navax
```

Open `http://localhost:8080/setup` and complete first-run setup with `NAVAX_SETUP_TOKEN` from `.env`. Before going live, set `PUBLIC_BASE_URL` to your real HTTPS address and `NAVAX_SECURE_COOKIES=true`.

Environment variables, reverse proxy, backups, updates, and systemd: [docs/deployment.md](docs/deployment.md) (Chinese). Official production CD: [deploy/README.md](deploy/README.md).

Health check: `GET /healthz` · database readiness: `GET /readyz` · build info: `GET /api/v1/version`.

## Local development and build

Requires Go 1.25, Node.js 22, and npm.

```bash
make check     # TypeScript, ESLint, mock contract, gofmt, go vet
make test      # all Go tests
make build     # frontend + embed + bin/navax
go run ./cmd/navax
```

Frontend-only: `cd web && npm run dev` with `VITE_ENABLE_API_MOCKS=true`. Merge gates and architecture boundaries: [CONTRIBUTING.md](CONTRIBUTING.md) (Chinese).

## Writing themes

A theme is a server-validated data package, not executable code. Author contract: [docs/theme-api.md](docs/theme-api.md) (Chinese). Minimal example: [examples/theme-starter](examples/theme-starter).

## Documentation

Index: [docs/README.md](docs/README.md). Product scope: [docs/requirements.md](docs/requirements.md). Architecture: [docs/architecture.md](docs/architecture.md).

## Contributing and license

Participation is governed by [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Report security issues privately per [SECURITY.md](SECURITY.md). The source is licensed under [AGPL-3.0-only](LICENSE); if you run a modified version as a network service, offer its source to your users (keep the footer source link). The "nav.ax" name and logo identify the official instance and are not covered by the code license.
