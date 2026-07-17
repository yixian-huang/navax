# nav.ax

[中文](README.md) | English

nav.ax is a personalized navigation-site (start page) service built with Go and
React, designed for individuals, invited users, and self-hosting. A single Go
process serves the REST API, public navigation pages, the admin UI, SQLite
storage, and the embedded frontend.

## Quick start (Docker Compose)

Requires Docker 24+ with Compose v2.

```bash
cp .env.example .env
# Generate two independent random secrets for production
sed -i.bak "s/^NAVAX_SETUP_TOKEN=$/NAVAX_SETUP_TOKEN=$(openssl rand -hex 32)/" .env
sed -i.bak "s|^NAVAX_MASTER_KEY=$|NAVAX_MASTER_KEY=$(openssl rand -base64 32)|" .env
docker compose up -d --build
docker compose logs -f navax
```

Open `http://localhost:8080/setup` and complete first-run setup with the
`NAVAX_SETUP_TOKEN` from `.env`. Before going live, set `PUBLIC_BASE_URL` to
your real HTTPS address and `NAVAX_SECURE_COOKIES=true`. Your reverse proxy
must preserve the original `Host`; personal subdomains additionally require
`ROOT_DOMAIN`, wildcard DNS, and TLS.

Health check: `GET /healthz` · readiness: `GET /readyz` · build info:
`GET /api/v1/version`.

## Build from source

Requires Go 1.25, Node.js 22, and npm.

```bash
make frontend  # npm ci + Vite production build to web/out
make check     # TypeScript, ESLint, gofmt, go vet
make test      # all Go tests
make build     # embedded-frontend static binary at bin/navax
```

See the [Chinese README](README.md) for the full configuration reference,
backup/restore, and update procedures — it is the authoritative version.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) (Chinese). Every change must pass
`make check`, `go test -race ./...`, and `make build`. Report security issues
privately per [SECURITY.md](SECURITY.md).

## License and trademarks

The source code is licensed under [AGPL-3.0-only](LICENSE). If you run a
modified version as a network service, the AGPL requires you to offer its
source to your users — the built-in footer source link satisfies this; keep an
equivalent link in your fork.

The "nav.ax" name and logo identify the official instance and are not covered
by the code license. Please run modified deployments under your own name and
branding.
