CREATE TABLE IF NOT EXISTS environments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    docker_host TEXT    NOT NULL DEFAULT 'unix:///var/run/docker.sock',
    is_local    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- Seed the built-in local environment; no-op on subsequent runs
INSERT OR IGNORE INTO environments (id, name, docker_host, is_local)
VALUES (1, 'Local', 'unix:///var/run/docker.sock', 1);
