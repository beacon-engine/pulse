-- Per-host agent endpoint (pull). When set, the gateway dials this address to
-- reach the agent that serves this environment, instead of the single default
-- AGENT_ADDR. NULL keeps the environment on the default agent. Complements
-- agent_id (a push session); a host may use either.
ALTER TABLE environments ADD COLUMN agent_addr TEXT;
