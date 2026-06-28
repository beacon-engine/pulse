CREATE TABLE IF NOT EXISTS container_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    container_id   TEXT NOT NULL,
    container_name TEXT NOT NULL DEFAULT '',
    event       TEXT NOT NULL,  -- start | stop | restart | crash
    occurred_at TEXT NOT NULL   -- ISO-8601 UTC
);

CREATE TABLE IF NOT EXISTS alert_config (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    channel         TEXT NOT NULL,  -- slack | discord | webhook
    endpoint_url    TEXT NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL
);
