# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) or opencode when working with code in this repository.

## Commands

```bash
# Build
go build -trimpath -ldflags="-s -w" -o crowdsec-capi .

# Run (requires a config file or env vars)
./crowdsec-capi serve -c config.yaml

# Test
go test ./... -count=1

# Single package test
go test ./internal/service/... -count=1 -v

# Lint
go vet ./...

# Docker (PostgreSQL + server)
make docker-up    # docker compose up --build -d
make docker-down
```

All config keys map to env vars with `CAPI_` prefix and `_` replacing `.` — e.g. `CAPI_DATABASE_DSN`, `CAPI_ADMIN_PASSWORD`, `CAPI_ALLOWLISTS_FILE`.

## Architecture

Self-hosted replacement for `api.crowdsec.net` (CrowdSec CAPI). CrowdSec agents point their `online_api_credentials.yaml` at this server instead of the upstream SaaS.

### Request flow
1. Agent registers/logs in via `POST /v2/watchers` → `POST /v2/watchers/login` → receives JWT
2. Agent pushes signals via `POST /v2/signals` → `internal/service/signal.go` deduplicates, checks allowlists, creates decisions in DB
3. Agent pulls decisions via `GET /v2/decisions/stream?startup=true/false` → delta stream using per-machine cursor in `machine_decision_cursors` table

### Key packages

| Package | Role |
|---|---|
| `internal/config` | Viper config loader; all config structs live here |
| `internal/db` | pgx pool init + golang-migrate runner (migrations embedded via `go:embed`) |
| `internal/db/queries` | Hand-written pgx query functions (no ORM) |
| `internal/db/migrations` | Numbered SQL migration files (`001_`…`006_`) |
| `internal/auth` | HS256 JWT (sign/verify), bcrypt password, chi Bearer middleware, JWT session manager for OIDC |
| `internal/api/router.go` | Assembles all routes; admin auth middleware (session cookie → Bearer key → Basic Auth) |
| `internal/api/v2` | CAPI-compatible agent-facing endpoints |
| `internal/api/admin` | Admin CRUD endpoints (machines, decisions, allowlists, enrollment keys, stats) |
| `internal/api/authapi` | OIDC browser flow handlers: `/auth/config`, `/auth/login`, `/auth/callback`, `/auth/logout` |
| `internal/oidcauth` | OIDC provider wrapper (discovery via `coreos/go-oidc/v3`, token exchange, email/domain allow-list) |
| `internal/service/signal.go` | Signal batch processing: allowlist check → dedup → decision creation |
| `internal/upstream` | Background goroutine syncing decisions from `api.crowdsec.net`; uses pg advisory lock `12345678` for single-runner in multi-replica |
| `internal/allowlists` | YAML file loader (`allowlists.yaml`) — upserts managed allowlists at startup |
| `internal/web` | Embedded vanilla JS dashboard (`go:embed static/*`) served at `/ui` |

### Database conventions
- Soft deletes: `is_deleted = TRUE` + `deleted_at` timestamp (never hard-delete decisions — agents need delta of removals)
- Decision expiry: background goroutine every 5 min calls `SoftDeleteExpiredDecisions` so expirations propagate to agents as "deleted" entries in the stream
- Cursor tracking: `machine_decision_cursors` stores `last_pulled_at` per machine; `startup=true` bypasses cursor and returns all active decisions

### Admin auth
Three accepted methods (checked in order):
1. `capi_session` HttpOnly cookie — issued after OIDC browser login via `/auth/login` → `/auth/callback`
2. `Authorization: Bearer <cfg.Admin.APIKey>` (if api_key is set)
3. HTTP Basic Auth with `cfg.Admin.Username` / `cfg.Admin.Password`

Password auto-generated at startup if empty (logged at WARN level). Basic Auth and Bearer key always work even when OIDC is enabled.

### OIDC
Configured under `auth.oidc`. Provider is initialised at startup via OIDC discovery (`coreos/go-oidc/v3`); startup fails if `enabled: true` but the provider is unreachable. Session TTL follows `server.jwt_ttl`. State cookie (`capi_state`) provides CSRF protection during the redirect flow. The UI detects the mode via `GET /auth/config` (public endpoint) and shows either a username/password form or an SSO button.

### Web UI
`index.html` references `echarts.min.js` and `world.json` as local files embedded in the binary via `go:embed`. Both files are committed to `internal/web/static/`.

### Allowlists as code
Set `allowlists.file: ./allowlists.yaml` (or `CAPI_ALLOWLISTS_FILE`). File is processed at every startup; managed allowlists cannot be deleted via API/UI. See `allowlists.example.yaml` for format.
