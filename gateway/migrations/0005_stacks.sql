-- Persisted Docker Compose / stack definitions managed by Pulse.
CREATE TABLE stacks (
    id          TEXT PRIMARY KEY NOT NULL,
    host_id     TEXT NOT NULL,
    name        TEXT NOT NULL,
    content     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'created',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    UNIQUE(host_id, name)
);
