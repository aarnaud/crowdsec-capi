CREATE TABLE IF NOT EXISTS machines (
    id            BIGSERIAL PRIMARY KEY,
    machine_id    VARCHAR(48) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT,
    tags          TEXT[] DEFAULT '{}',
    scenarios     JSONB DEFAULT '[]',
    ip_address    INET,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    enrolled_at   TIMESTAMPTZ,
    last_seen_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
