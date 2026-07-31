-- A negotiation task is a fact about one ROUND, not about a contract. Each
-- accepted redline bumps contract_version and starts a new round, so the task
-- carries the version it was minted for; the round predicates (is this instance
-- a negotiator, are any tasks still open, accept my task) all scope to it.
--
-- The unique index is the idempotency mechanism: accepting the same offer twice
-- inserts once (the repository's Create uses ON CONFLICT DO NOTHING).
--
-- Existing rows land on version 1. A contract already past version 1 therefore
-- has no task for its current round and the party re-engages once.
ALTER TABLE contract_negotiation_task
    ADD COLUMN contract_version INT NOT NULL DEFAULT 1;

ALTER TABLE contract_negotiation_task
    ALTER COLUMN contract_version DROP DEFAULT;

CREATE UNIQUE INDEX uq_contract_negotiation_task_round
    ON contract_negotiation_task (did, negotiator, contract_version);
