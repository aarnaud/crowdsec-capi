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

Edit `/etc/crowdsec/online_api_credentials.yaml` on each agent:

```yaml
url: http://<your-server>:8080
login: <machine_id>
password: <password>
```

Then re-register: `sudo cscli capi register`

## Configuration

```yaml
server:
  listen: "0.0.0.0:8080"
  jwt_ttl: "24h"

admin:
  username: "admin"
  password: ""       # auto-generated if empty (printed to logs at startup)
  api_key: ""        # optional static Bearer token for admin API

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

All keys are also settable via environment variables with the `CAPI_` prefix (dots become underscores), e.g. `CAPI_DATABASE_DSN`, `CAPI_ADMIN_PASSWORD`, `CAPI_UPSTREAM_MACHINE_ID`.

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

## Building

```bash
go build -trimpath -ldflags="-s -w" -o crowdsec-capi .
./crowdsec-capi serve -c config.yaml
```

Multi-stage Docker build:

```bash
docker build -t crowdsec-capi:latest .
```
