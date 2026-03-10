CREATE TABLE IF NOT EXISTS signals (
    id               BIGSERIAL PRIMARY KEY,
    uuid             UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    machine_id       VARCHAR(48) NOT NULL,
    scenario         TEXT NOT NULL,
    scenario_hash    TEXT,
    scenario_version TEXT,
    source_scope     VARCHAR(50),
    source_value     TEXT,
    source_ip        INET,
    source_range     CIDR,
    source_as_name   TEXT,
    source_as_number INTEGER,
    source_country   VARCHAR(10),
    source_latitude  FLOAT,
    source_longitude FLOAT,
    labels           JSONB DEFAULT '{}',
    start_at         TIMESTAMPTZ,
    stop_at          TIMESTAMPTZ,
    alert_count      INTEGER DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signals_machine_id ON signals(machine_id);
CREATE INDEX IF NOT EXISTS idx_signals_created_at ON signals(created_at);
