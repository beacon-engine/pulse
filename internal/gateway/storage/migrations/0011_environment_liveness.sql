-- Auto-registration + liveness lifecycle for agent-declared environments.
--
-- When an agent dials in (push mode) and self-registers, the gateway upserts a
-- row here keyed by its identity ("<token_id>:<instance_uuid>", stored in
-- agent_id). These rows go through a liveness lifecycle: heartbeats keep them
-- online, a dropped session / missed heartbeats mark them offline, and a
-- retention sweep hard-deletes them after they've been offline long enough.
-- Manually-created rows (the API, the seeded Local host) keep source='manual'
-- and are never touched by that lifecycle.

-- 'manual' (API / seeded) or 'auto' (self-registered by a dial-in agent).
ALTER TABLE environments ADD COLUMN source TEXT NOT NULL DEFAULT 'manual';
-- 'online' | 'offline'. Meaningful for 'auto' rows; 'manual' rows stay 'online'.
ALTER TABLE environments ADD COLUMN status TEXT NOT NULL DEFAULT 'online';
-- RFC3339 UTC, bumped on connect and on each heartbeat. NULL for manual rows.
ALTER TABLE environments ADD COLUMN last_seen TEXT;
-- RFC3339 UTC set when a row is marked offline; NULL while online. Drives the
-- alert-grace and retention windows.
ALTER TABLE environments ADD COLUMN offline_since TEXT;
-- 1 when the agent deregistered cleanly (SIGTERM / scale-down) — suppresses the
-- "agent down" alert so expected autoscaling churn is quiet.
ALTER TABLE environments ADD COLUMN graceful INTEGER NOT NULL DEFAULT 0;
-- 1 once a down alert has fired, so it fires at most once per offline episode.
ALTER TABLE environments ADD COLUMN alerted INTEGER NOT NULL DEFAULT 0;

-- One row per (token, instance): makes self-registration idempotent so a restart
-- or reconnect-overlap upserts the existing row instead of duplicating it.
-- Partial (auto only) so manual rows — which may share a NULL/blank agent_id —
-- are unaffected.
CREATE UNIQUE INDEX IF NOT EXISTS idx_environments_auto_agent
    ON environments(agent_id) WHERE source = 'auto';
