CREATE TABLE IF NOT EXISTS decisions (
    id          BIGSERIAL PRIMARY KEY,
    uuid        UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    origin      VARCHAR(50) NOT NULL,
    type        VARCHAR(50) NOT NULL,
    scope       VARCHAR(50) NOT NULL,
    value       TEXT NOT NULL,
    duration    INTERVAL NOT NULL,
    scenario    TEXT,
    source_machine_id TEXT,
    simulated   BOOLEAN NOT NULL DEFAULT FALSE,
    is_deleted  BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_decisions_expires_at ON decisions(expires_at);
CREATE INDEX IF NOT EXISTS idx_decisions_updated_at ON decisions(updated_at);
CREATE INDEX IF NOT EXISTS idx_decisions_scope_value ON decisions(scope, value);
CREATE INDEX IF NOT EXISTS idx_decisions_is_deleted ON decisions(is_deleted);

CREATE TABLE IF NOT EXISTS machine_decision_cursors (
    machine_id     VARCHAR(48) PRIMARY KEY,
    last_pulled_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
