-- Migrate container_events and alert_config from INTEGER primary key to UUID (TEXT) primary key.
-- SQLite doesn't support ALTER COLUMN, so we rename + recreate.

-- ── container_events ─────────────────────────────────────────────────────────
ALTER TABLE container_events RENAME TO _container_events_old;

CREATE TABLE container_events (
    id             TEXT PRIMARY KEY NOT NULL,
    container_id   TEXT NOT NULL,
    container_name TEXT NOT NULL DEFAULT '',
    event          TEXT NOT NULL,
    occurred_at    TEXT NOT NULL
);

INSERT INTO container_events (id, container_id, container_name, event, occurred_at)
SELECT
    lower(hex(randomblob(4)))               || '-' ||
    lower(hex(randomblob(2)))               || '-4' ||
    lower(substr(hex(randomblob(2)), 2))    || '-' ||
    lower(substr('89ab', (abs(random()) % 4) + 1, 1)) ||
    lower(substr(hex(randomblob(2)), 2))    || '-' ||
    lower(hex(randomblob(6))),
    container_id, container_name, event, occurred_at
FROM _container_events_old;

DROP TABLE _container_events_old;

-- ── alert_config ─────────────────────────────────────────────────────────────
ALTER TABLE alert_config RENAME TO _alert_config_old;

CREATE TABLE alert_config (
    id           TEXT PRIMARY KEY NOT NULL,
    channel      TEXT NOT NULL,
    endpoint_url TEXT NOT NULL,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT NOT NULL
);

INSERT INTO alert_config (id, channel, endpoint_url, enabled, created_at)
SELECT
    lower(hex(randomblob(4)))               || '-' ||
    lower(hex(randomblob(2)))               || '-4' ||
    lower(substr(hex(randomblob(2)), 2))    || '-' ||
    lower(substr('89ab', (abs(random()) % 4) + 1, 1)) ||
    lower(substr(hex(randomblob(2)), 2))    || '-' ||
    lower(hex(randomblob(6))),
    channel, endpoint_url, enabled, created_at
FROM _alert_config_old;

DROP TABLE _alert_config_old;
