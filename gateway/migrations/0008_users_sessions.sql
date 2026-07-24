-- Dashboard authentication: users and login sessions.

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer', -- 'admin' | 'viewer'
    created_at    TEXT NOT NULL
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
