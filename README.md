# CrowdSec Central API

A self-hosted replacement for `api.crowdsec.net`. Point your CrowdSec agents at this server to:

- Aggregate signals from all local agents and redistribute them as a private community blocklist
- Proxy and cache the real upstream CAPI blocklist under a single upstream account
- Manage decisions, allowlists, and enrollment keys without touching the SaaS

## Quick Start

```bash
# Copy and edit config
cp config.example.yaml config.yaml

# Start with Docker Compose (PostgreSQL + server)
docker compose up --build
```

The server starts on `http://localhost:8080`. The admin password is auto-generated and printed to the logs on first start if not configured.

### Point a CrowdSec agent at this server

Because `cscli capi register` is hardcoded to `api.crowdsec.net`, this server ships a `register` command that handles the full registration and enrollment flow and writes `/etc/crowdsec/online_api_credentials.yaml` directly.

**1. Create an enrollment key** in the admin UI (`/ui` → Enrollment Keys) or via the API:

```bash
curl -u admin:password -X POST http://localhost:8080/admin/enrollment-keys \
  -H 'Content-Type: application/json' \
  -d '{"description": "my-server"}'
```

**2. Run `register` on the agent machine:**

```bash
# Via flags
crowdsec-capi register \
  --url http://<your-server>:8080 \
  --enrollment-key <key> \
  --name my-server \
  --output /etc/crowdsec/online_api_credentials.yaml

# Via environment variables (suitable for containers and CI)
CAPI_URL=http://<your-server>:8080 \
CAPI_ENROLLMENT_KEY=<key> \
CAPI_MACHINE_NAME=my-server \
CAPI_OUTPUT=/etc/crowdsec/online_api_credentials.yaml \
crowdsec-capi register
```

Add `--insecure` to skip TLS certificate verification for self-signed certs.

**3. Restart the CrowdSec agent:**

```bash
systemctl restart crowdsec
```

## Configuration

```yaml
server:
  listen: "0.0.0.0:8080"
  jwt_ttl: "24h"
  secure_cookies: false  # set to true when serving over HTTPS

admin:
  username: "admin"
  password: ""       # auto-generated if empty (printed to logs at startup)
  api_key: ""        # optional static Bearer token for admin API

auth:
  oidc:
    enabled: false
    issuer: ""           # e.g. https://accounts.google.com
    client_id: ""
    client_secret: ""
    redirect_url: "http://localhost:8080/auth/callback"
    scopes: ["openid", "profile", "email"]
    allowed_emails: []   # restrict to specific emails (empty = allow all)
    allowed_domains: []  # restrict by domain e.g. ["mycompany.com"]

allowlists:
  file: ""           # path to allowlists-as-code YAML file

database:
  dsn: "postgresql://capi:secret@localhost:5432/capi?sslmode=disable"

upstream:
  enabled: true
  base_url: "https://api.crowdsec.net"
  machine_id: ""     # upstream CAPI credentials
  password: ""
  sync_interval: "1h"
  push_signals: false

decisions:
  default_duration: "24h"

log:
  level: "info"      # debug | info | warn | error
  format: "json"     # json | pretty
```

All keys are settable via environment variables with the `CAPI_` prefix (dots become underscores), e.g. `CAPI_DATABASE_DSN`, `CAPI_ADMIN_PASSWORD`, `CAPI_AUTH_OIDC_ISSUER`.

## Admin Authentication

Three methods are accepted for `/admin/*` endpoints, checked in order:

1. **OIDC session cookie** — set after a browser-based SSO login via `/auth/login`
2. **Bearer API key** — `Authorization: Bearer <api_key>` (set `admin.api_key` in config)
3. **HTTP Basic Auth** — `Authorization: Basic base64(username:password)`

Basic Auth and Bearer key always work regardless of whether OIDC is configured, making them suitable for scripts and CI/CD.

### OIDC / SSO Setup

Enable OIDC to allow browser-based login with any OpenID Connect provider (Google, Keycloak, Authentik, Okta, etc.):

```yaml
auth:
  oidc:
    enabled: true
    issuer: "https://accounts.google.com"
    client_id: "my-client-id"
    client_secret: "my-secret"
    redirect_url: "http://localhost:8080/auth/callback"
    allowed_domains: ["mycompany.com"]   # optional
```

Register `http://localhost:8080/auth/callback` as an allowed redirect URI in your provider. When OIDC is enabled the web UI shows a "Sign in with SSO" button instead of the username/password form.

## Allowlists as Code

Define allowlists in a YAML file to version-control your trusted IPs:

```yaml
# allowlists.yaml
allowlists:
  - name: trusted-networks
    label: "Trusted Networks"
    description: "Internal and office ranges"
    entries:
      - scope: Range
        value: "10.0.0.0/8"
        comment: "RFC1918"
      - scope: Ip
        value: "203.0.113.10"
        comment: "Office egress"
      - scope: Country
        value: "FR"
```

Enable with `allowlists.file: ./allowlists.yaml` (or `CAPI_ALLOWLISTS_FILE`). The file is re-applied on every startup. Allowlists defined this way are marked **managed** and cannot be deleted via the API or UI.

## Web UI

Dashboard available at `http://localhost:8080/ui`:

- **Dashboard** — stat cards (machines, decisions, signals), donut charts by origin/type, world map of attack origins by country
- **Machines** — block, unblock, delete registered agents
- **Decisions** — view all active decisions, add manual bans, delete entries
- **Allowlists** — manage allowlists and entries
- **Enrollment Keys** — generate and revoke enrollment keys
- **Upstream** — view upstream CAPI sync status

## Commands

| Command | Description |
|---|---|
| `crowdsec-capi serve -c config.yaml` | Start the API server |
| `crowdsec-capi register --url ... --enrollment-key ...` | Register and enroll an agent, write credentials file |

## Building

```bash
go build -trimpath -ldflags="-s -w" -o crowdsec-capi .
./crowdsec-capi serve -c config.yaml
```

Multi-stage Docker build (Go 1.25 → distroless):

```bash
docker build -t crowdsec-capi:latest .
```
