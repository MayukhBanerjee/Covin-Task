CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    event_id    TEXT NOT NULL,
    call_id     TEXT NOT NULL,
    account_id  TEXT NOT NULL,
    payload     JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_event_id ON events (event_id);

CREATE TABLE IF NOT EXISTS calls (
    call_id             TEXT PRIMARY KEY,
    account_id          TEXT NOT NULL,
    status              TEXT NOT NULL,
    duration_sec        INT  NOT NULL,
    recording_url       TEXT,
    recording_processed BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS account_stats (
    account_id         TEXT PRIMARY KEY,
    call_count         BIGINT NOT NULL DEFAULT 0,
    total_duration_sec BIGINT NOT NULL DEFAULT 0
);
