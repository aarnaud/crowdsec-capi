CREATE TABLE IF NOT EXISTS enrollment_keys (
    id          BIGSERIAL PRIMARY KEY,
    key         VARCHAR(64) NOT NULL UNIQUE,
    description TEXT,
    tags        TEXT[] DEFAULT '{}',
    max_uses    INTEGER,
    use_count   INTEGER NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS papi_orders (
    id          BIGSERIAL PRIMARY KEY,
    machine_id  VARCHAR(48) NOT NULL,
    order_type  VARCHAR(50) NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}',
    delivered   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS upstream_sync_state (
    id               INTEGER PRIMARY KEY DEFAULT 1,
    last_sync_at     TIMESTAMPTZ,
    last_startup_at  TIMESTAMPTZ,
    machine_id       TEXT,
    token            TEXT,
    token_expires_at TIMESTAMPTZ,
    decision_count   INTEGER DEFAULT 0,
    CHECK (id = 1)
);

INSERT INTO upstream_sync_state (id) VALUES (1) ON CONFLICT DO NOTHING;
