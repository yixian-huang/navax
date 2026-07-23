# nav.ax

[中文](README.md) | English

nav.ax is a personalized navigation-site (start page) service built with Go and
React, designed for individuals, invited users, and self-hosting. A single Go
process serves the REST API, public navigation pages, the admin UI, SQLite
storage, and the embedded frontend assets.

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

Open `http://localhost:8080/setup` and complete first-run setup with the
`NAVAX_SETUP_TOKEN` from `.env`. Before going live, set `PUBLIC_BASE_URL` in
`.env` to your real HTTPS address and set `NAVAX_SECURE_COOKIES=true`. Your
reverse proxy must preserve the original `Host`. To enable personal subdomains,
set the root domain under Admin → System settings → Domain, and provide wildcard
DNS and TLS.

On the official `nav.ax` instance, available subdomains of 4+ characters are
enabled automatically; scarce 1–3 character names go to admin review. Paid
subscriptions are not part of v1; future commercialization focuses on higher
link quotas, short subdomains, and white-label capability.

Health check: `GET /healthz` · database readiness: `GET /readyz` · build info:
`GET /api/v1/version`.

## Local development and build

Requires Go 1.25, Node.js 22, and npm.

```bash
make frontend  # npm ci + Vite production build to web/out
make check     # TypeScript, ESLint, gofmt, go vet
make test      # all Go tests
make embed     # copy the frontend bundle into internal/webui/dist
make build     # embedded-frontend static binary at bin/navax
```

To run the native binary, export the environment first:

```bash
set -a; . ./.env; set +a
NAVAX_DATA_DIR=./data ./bin/navax
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `NAVAX_ADDR` | `:8080` | HTTP listen address |
| `NAVAX_DATA_DIR` | `./data` | SQLite, uploads, backups, and key directory; `/data` inside containers |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Public absolute URL, no trailing `/` |
| `INSTANCE_NAME` | `nav.ax` | Instance name |
| `NAVAX_SETUP_TOKEN` | random at startup | First-run setup token, at least 32 characters |
| `NAVAX_MASTER_KEY` | empty | Base64 32-byte key encrypting third-party credentials; do not rotate casually once set |
| `NAVAX_SECURE_COOKIES` | auto-enabled with HTTPS | Send session cookies over HTTPS only |
| `NAVAX_SESSION_TTL` | `720h` | Session lifetime |
| `NAVAX_SHUTDOWN_TIMEOUT` | `15s` | Graceful shutdown deadline |
| `NAVAX_UPDATE_MANIFEST_URL` | empty | Optional signed update-manifest URL; official value under [Updates](#updates) |
| `NAVAX_UPDATE_PUBLIC_KEY` | empty | Base64 Ed25519 public key verifying updates; official value under [Updates](#updates) |

## Data, backup, and restore

All persistent data lives in `NAVAX_DATA_DIR`. Compose uses the fixed named
volume `navax-data`; removing the container does not delete the volume. Prefer
creating, downloading, and restoring `.navbak` full-instance archives from the
admin UI — they contain a SQLite snapshot, locally uploaded assets, and
instance-generated keys. A restore exits the process cleanly; Compose's restart
policy validates and atomically applies the archive on the next start.

For whole-volume offline backups, stop the service first to avoid copying WAL
intermediate state:

```bash
docker compose stop navax
docker run --rm -v navax-data:/data:ro -v "$PWD":/backup alpine \
  tar czf /backup/navax-data-$(date +%F).tar.gz -C /data .
docker compose start navax
```

If `NAVAX_MASTER_KEY` is set explicitly in `.env`, it is deployment
configuration and is not written into archives — store it in a controlled
location alongside your backups, or encrypted third-party credentials cannot
be decrypted after a restore.

## Updates

Container deployments do not replace themselves in place. Create a backup
first, then let Compose pull and recreate:

```bash
docker compose pull
docker compose up -d
docker compose ps
```

When building from local source, run
`docker compose build --pull && docker compose up -d` instead. Native binaries
can be downloaded per platform from GitHub Releases and verified with
`SHA256SUMS`; with a signed update manifest configured, the admin UI can also
perform atomic updates with backup and verification built in.

The release workflow generates `update-manifest.json` when the repository
secret `NAVAX_UPDATE_SIGNING_KEY_DER` exists. The secret is the Base64 of an
Ed25519 PKCS#8 DER private key; on the instance side, `NAVAX_UPDATE_PUBLIC_KEY`
is the Base64 of the corresponding 32-byte raw public key. The private key is
used only for signing in GitHub Actions and must never be deployed to
instances.

### Enabling one-click updates (self-hosting)

Official releases ship an `update-manifest.json` signed with the official key.
Set both variables below, and the admin UI can perform atomic updates with
backup and verification built in:

```bash
NAVAX_UPDATE_MANIFEST_URL=https://github.com/yixian-huang/navax/releases/latest/download/update-manifest.json
NAVAX_UPDATE_PUBLIC_KEY=P0yCGX0jV+TAx/BfmY7tvGKFeRQmtjq/y9/pMl8ciDA=
```

Preconditions:

- **Requires the first stable release.** Hyphenated tags (such as
  `v0.1.0-rc.1`) are uploaded as pre-releases, and `latest/download` only
  resolves stable releases — so the URL returns 404 until `v0.1.0` ships. You
  may also point it at a specific tag's asset URL instead.
- **Native binary deployments only.** Container deployments are rejected; use
  `docker compose pull` there.
- After an update the process **exits gracefully but does not restart itself**,
  so a systemd unit with `Restart=always` is required (see
  [docs/deployment.md](docs/deployment.md) §7, Chinese).
- Updates are refused — leaving the old version in place — when checksum
  verification fails, the manifest signature does not match, or the offered
  version is not newer than the running one.

## Project structure

- `cmd/navax/` — entry point and build info
- `internal/` — domain logic, HTTP, SQLite, operations, and embedded web UI
- `migrations/` — SQLite migrations applied automatically at startup
- `web/` — React/Vite frontend
- `api/openapi.yaml` — the HTTP API contract (single source of truth for endpoints)
- `docs/` — requirements, architecture, and deployment notes (Chinese; see below)
- `deploy/` — native binary install and official production CD notes

## Documentation

| Doc | Contents |
| --- | --- |
| [docs/requirements.md](docs/requirements.md) | Product scope and acceptance (authoritative; Chinese) |
| [docs/architecture.md](docs/architecture.md) | Module boundaries, data, and security invariants |
| [docs/deployment.md](docs/deployment.md) | Self-hosting: DNS, TLS, reverse proxy, env vars |
| [deploy/README.md](deploy/README.md) | systemd install, upgrades, official CI→NoPanel CD |
| [docs/design-background-media-library.md](docs/design-background-media-library.md) | Background media library design note |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Dev commands, merge gates, architecture boundaries (Chinese) |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) (Chinese) for the workflow, merge
gates, and architecture boundaries; participation is governed by
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Every change must pass `make check`,
`go test -race ./...`, and `make build`. Report security issues privately per
[SECURITY.md](SECURITY.md) — never in a public issue.

## License and trademarks

The source code is licensed under [AGPL-3.0-only](LICENSE). Under AGPL
section 13, if you run a modified version as a network service you must offer
its source to your users — the built-in footer source link exists to satisfy
this; keep an equivalent link in your fork.

The "nav.ax" name and logo identify the official instance and are not covered
by the code license. Please run modified deployments under your own name and
branding to avoid confusion with the official instance.
