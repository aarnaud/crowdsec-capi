# API Reference

All responses are JSON. Timestamps are RFC 3339. Durations follow Go syntax (`24h`, `7d`, `30m`).

---

## Authentication

### Agent endpoints (`/v2/*`)
Bearer JWT obtained from `POST /v2/watchers/login`:
```
Authorization: Bearer <token>
```

### Admin endpoints (`/admin/*`)
Either HTTP Basic Auth or a static Bearer token:
```
Authorization: Basic base64(username:password)
Authorization: Bearer <admin_api_key>
```

---

## Health

### `GET /healthz`
Always returns `200 ok`. Use for liveness probes.

### `GET /readyz`
Returns `200 ok` when the database is reachable. Use for readiness probes.

---

## Agent API — `/v2`

### Register a machine

```
POST /v2/watchers
```

No authentication required.

**Request**
```json
{
  "machine_id": "myagent01",
  "password": "strongpassword"
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `machine_id` | string | yes | Max 48 chars, alphanumeric |
| `password` | string | yes | Min 8 chars |

**Response** `201`
```json
{ "message": "machine registered" }
```

---

### Login

```
POST /v2/watchers/login
```

No authentication required.

**Request**
```json
{
  "machine_id": "myagent01",
  "password": "strongpassword",
  "scenarios": [
    { "name": "crowdsecurity/http-bf", "version": "0.1", "hash": "abc123" }
  ]
}
```

**Response** `200`
```json
{
  "code": 200,
  "expire": "2024-01-02T15:04:05Z",
  "token": "<jwt>"
}
```

---

### Enroll a machine

```
POST /v2/watchers/enroll
```

Requires JWT. Validates an enrollment key and sets the machine status to `validated`.

**Request**
```json
{
  "attachment_key": "<enrollment_key>",
  "name": "my-agent",
  "tags": ["prod", "dc1"],
  "overwrite": false
}
```

**Response** `200`
```json
{ "message": "machine enrolled" }
```

---

### Reset password

```
POST /v2/watchers/reset
```

Requires JWT.

**Request**
```json
{
  "machine_id": "myagent01",
  "password": "newpassword"
}
```

**Response** `200`
```json
{ "message": "password reset" }
```

---

### Deregister

```
DELETE /v2/watchers/self
```

Requires JWT. Removes the calling machine from the database.

**Response** `204 No Content`

---

### Push signals

```
POST /v2/signals
```

Requires JWT. Accepts a batch of up to 250 signals. Each signal may include embedded decisions which are processed into the decision table (subject to allowlist check and deduplication).

**Request** — array of signal objects
```json
[
  {
    "uuid": "550e8400-e29b-41d4-a716-446655440000",
    "machine_id": "myagent01",
    "scenario": "crowdsecurity/http-bf",
    "scenario_hash": "abc123",
    "scenario_version": "0.1",
    "source": {
      "scope": "Ip",
      "value": "1.2.3.4",
      "ip": "1.2.3.4",
      "as_name": "ACME ISP",
      "as_number": 12345,
      "country": "CN",
      "latitude": 39.9042,
      "longitude": 116.4074
    },
    "decisions": [
      {
        "scope": "Ip",
        "value": "1.2.3.4",
        "type": "ban",
        "duration": "24h",
        "origin": "crowdsec"
      }
    ],
    "start_at": "2024-01-01T10:00:00Z",
    "stop_at":  "2024-01-01T10:05:00Z",
    "alert_count": 42
  }
]
```

**Response** `200`

---

### Pull decisions (stream)

```
GET /v2/decisions/stream?startup=true
```

Requires JWT.

| Query param | Values | Notes |
|---|---|---|
| `startup` | `true` / `false` | `true` returns all active decisions; `false` returns only changes since last pull |

**Response** `200`
```json
{
  "new": [
    {
      "id": 1,
      "uuid": "550e8400-...",
      "origin": "local-signal",
      "type": "ban",
      "scope": "Ip",
      "value": "1.2.3.4",
      "duration": "24h0m0s",
      "scenario": "crowdsecurity/http-bf",
      "simulated": false
    }
  ],
  "deleted": []
}
```

Decision origins: `local-signal` | `upstream-capi` | `manual`

---

### Sync decisions

```
POST /v2/decisions/sync
```

Requires JWT. Accepted for protocol compatibility; currently a no-op.

**Response** `200`

---

### Heartbeat

```
GET /v2/heartbeat
```

Requires JWT. Updates `last_seen_at` for the machine.

**Response** `200`
```json
{ "status": "ok" }
```

---

### Metrics

```
POST /v2/metrics
POST /v2/usage-metrics
```

Requires JWT. Updates `last_seen_at` for the machine. Both endpoints are accepted for compatibility.

**Response** `200`

---

### Allowlists

```
GET  /v2/allowlists
HEAD /v2/allowlists
POST /v2/allowlists/{name}
```

Requires JWT. Returns the list of configured allowlists. `POST` is accepted for compatibility.

---

### PAPI (console orders)

```
GET /v2/papi/v1/decisions
```

Requires JWT. Returns empty orders (stub for PAPI long-poll compatibility).

---

## Admin API — `/admin`

All endpoints require admin authentication (Basic Auth or Bearer API key).

---

### Stats

```
GET /admin/stats
```

**Response** `200`
```json
{
  "machines": {
    "total": 5,
    "validated": 4,
    "pending": 1,
    "blocked": 0
  },
  "decisions": {
    "total": 1520,
    "by_origin": {
      "local-signal":  320,
      "upstream-capi": 1180,
      "manual":        20
    },
    "by_type": {
      "ban":     1500,
      "captcha": 20
    }
  },
  "signals_last_24h": 87,
  "signals_by_country": [
    { "country": "CN", "count": 412 },
    { "country": "RU", "count": 201 }
  ]
}
```

`signals_by_country` covers the last 30 days, top 100 countries.

---

### Machines

#### List machines
```
GET /admin/machines
```

**Response** `200` — array of machine objects
```json
[
  {
    "MachineID": "myagent01",
    "Status": "validated",
    "Name": "prod-agent-1",
    "Tags": ["prod"],
    "IPAddress": "10.0.0.5",
    "LastSeenAt": "2024-01-01T12:00:00Z",
    "EnrolledAt": "2024-01-01T10:00:00Z",
    "CreatedAt": "2024-01-01T09:55:00Z"
  }
]
```

Machine statuses: `pending` | `validated` | `blocked`

#### Block a machine
```
PUT /admin/machines/{machine_id}/block
```

Sets machine status to `blocked`. Blocked machines can still authenticate but their signals are ignored.

**Response** `200`
```json
{ "message": "machine blocked" }
```

#### Unblock a machine
```
PUT /admin/machines/{machine_id}/unblock
```

Sets machine status back to `validated`.

**Response** `200`
```json
{ "message": "machine unblocked" }
```

#### Delete a machine
```
DELETE /admin/machines/{machine_id}
```

**Response** `204 No Content`

---

### Decisions

#### List decisions
```
GET /admin/decisions?include_deleted=false
```

| Query param | Default | Notes |
|---|---|---|
| `include_deleted` | `false` | Set to `true` to include soft-deleted and expired decisions |

**Response** `200` — array of decision objects

#### Create a manual decision
```
POST /admin/decisions
```

**Request**
```json
{
  "type":     "ban",
  "scope":    "Ip",
  "value":    "1.2.3.4",
  "duration": "48h",
  "scenario": "manual-block"
}
```

| Field | Values | Notes |
|---|---|---|
| `type` | `ban` \| `captcha` | Required |
| `scope` | `Ip` \| `Range` \| `Country` | Required |
| `value` | string | Required — IP, CIDR, or ISO country code |
| `duration` | Go duration string | Defaults to `decisions.default_duration` from config |
| `scenario` | string | Optional label |

**Response** `201`
```json
{ "message": "decision created" }
```

#### Delete a decision
```
DELETE /admin/decisions/{uuid}
```

Soft-deletes the decision. It will appear in the `deleted` list on agents' next decision stream pull.

**Response** `204 No Content`

---

### Allowlists

#### List allowlists
```
GET /admin/allowlists
```

**Response** `200`
```json
[
  {
    "ID": 1,
    "Name": "trusted-networks",
    "Label": "Trusted Networks",
    "Description": "Internal ranges",
    "Managed": true,
    "CreatedAt": "2024-01-01T00:00:00Z",
    "UpdatedAt": "2024-01-01T00:00:00Z"
  }
]
```

`Managed: true` means the allowlist is controlled by the `allowlists.file` config and cannot be deleted via the API.

#### Create an allowlist
```
POST /admin/allowlists
```

**Request**
```json
{
  "name":        "my-allowlist",
  "label":       "My Allowlist",
  "description": "Optional description"
}
```

**Response** `201` — created allowlist object

#### Delete an allowlist
```
DELETE /admin/allowlists/{id}
```

Returns `409 Conflict` if the allowlist is managed (file-controlled).

**Response** `204 No Content`

#### Add an entry to an allowlist
```
POST /admin/allowlists/{id}/entries
```

**Request**
```json
{
  "scope":   "Ip",
  "value":   "192.168.1.100",
  "comment": "My workstation"
}
```

| Field | Values |
|---|---|
| `scope` | `Ip` \| `Range` \| `Country` |
| `value` | IP address, CIDR range, or ISO 3166-1 alpha-2 country code |

**Response** `201`
```json
{ "message": "entry added" }
```

---

### Enrollment Keys

#### List enrollment keys
```
GET /admin/enrollment-keys
```

**Response** `200` — array of key objects

#### Generate an enrollment key
```
POST /admin/enrollment-keys
```

**Request**
```json
{
  "description": "datacenter-1 agents",
  "tags":        ["prod", "dc1"],
  "max_uses":    10,
  "expires_at":  "2025-01-01T00:00:00Z"
}
```

All fields are optional. `max_uses: null` means unlimited uses. The generated key is a random 64-character hex string. **It is only returned once** in the response — store it immediately.

**Response** `201`
```json
{
  "id":          1,
  "key":         "a3f8c2...",
  "description": "datacenter-1 agents",
  "tags":        ["prod", "dc1"],
  "max_uses":    10,
  "expires_at":  "2025-01-01T00:00:00Z",
  "created_at":  "2024-01-01T00:00:00Z"
}
```

#### Revoke an enrollment key
```
DELETE /admin/enrollment-keys/{id}
```

**Response** `204 No Content`

---

### Upstream

#### Get upstream sync status
```
GET /admin/upstream
```

**Response** `200`
```json
{
  "last_sync_at":   "2024-01-01T12:00:00Z",
  "machine_id":     "upstream-account-id",
  "decision_count": 1180
}
```

`last_sync_at` and `machine_id` are `null` when upstream sync has never run or is not configured.
