-- Migrate environments from INTEGER primary key to UUID (TEXT) primary key.
-- SQLite doesn't support ALTER COLUMN, so we rename + recreate.

ALTER TABLE environments RENAME TO _environments_old;

CREATE TABLE environments (
    id          TEXT    PRIMARY KEY NOT NULL,
    name        TEXT    NOT NULL,
    docker_host TEXT    NOT NULL DEFAULT 'unix:///var/run/docker.sock',
    is_local    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- Copy existing rows, generating UUID v4-shaped IDs via randomblob.
-- Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
INSERT INTO environments (id, name, docker_host, is_local, created_at)
SELECT
    lower(hex(randomblob(4)))               || '-' ||
    lower(hex(randomblob(2)))               || '-4' ||
    lower(substr(hex(randomblob(2)), 2))    || '-' ||
    lower(substr('89ab', (abs(random()) % 4) + 1, 1)) ||
    lower(substr(hex(randomblob(2)), 2))    || '-' ||
    lower(hex(randomblob(6))),
    name, docker_host, is_local, created_at
FROM _environments_old;

DROP TABLE _environments_old;
