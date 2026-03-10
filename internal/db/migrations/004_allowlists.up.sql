CREATE TABLE IF NOT EXISTS allowlists (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    label       TEXT,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS allowlist_entries (
    id           BIGSERIAL PRIMARY KEY,
    allowlist_id BIGINT NOT NULL REFERENCES allowlists(id) ON DELETE CASCADE,
    scope        VARCHAR(50) NOT NULL,
    value        TEXT NOT NULL,
    comment      TEXT,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(allowlist_id, scope, value)
);

CREATE INDEX IF NOT EXISTS idx_allowlist_entries_scope_value ON allowlist_entries(scope, value);
