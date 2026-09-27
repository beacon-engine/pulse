-- Runtime-tunable gateway settings, editable via /api/settings (and, later, a
-- settings page) so operators don't have to bake these into the agent/gateway
-- container env. Seeded with defaults; INSERT OR IGNORE keeps existing values on
-- re-run.
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO settings (key, value) VALUES
    -- No heartbeat for this many seconds → the agent is marked offline.
    ('agent_offline_threshold_seconds', '90'),
    -- Wait this long after an *abrupt* disconnect before alerting, to absorb fast
    -- rolling replacements.
    ('agent_alert_grace_seconds', '60'),
    -- Delete an offline auto-registered environment after it has been offline for
    -- this many hours (retains it as history until then).
    ('agent_retention_hours', '6');
