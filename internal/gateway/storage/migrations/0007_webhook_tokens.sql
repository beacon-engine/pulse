CREATE TABLE IF NOT EXISTS webhook_tokens (
    id          TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_key  TEXT NOT NULL,
    host_id     TEXT NOT NULL,
    token       TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE(entity_type, entity_key, host_id)
);
