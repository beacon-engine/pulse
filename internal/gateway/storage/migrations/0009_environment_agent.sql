-- Associate an environment with the push agent that represents it.
--
-- Each agent (a dial-in session, authenticated by its token) typically stands
-- for one environment. `agent_id` matches the id the agent registers under; a
-- request for this environment is then routed to that agent's live session.
-- NULL keeps the environment on the default pull agent (AGENT_ADDR).
ALTER TABLE environments ADD COLUMN agent_id TEXT;
