-- ADR-25: contract target systems become a configured registry, and each
-- contract designates the one it deploys to.
--
-- Deployment previously addressed a single endpoint held in CONTRACT_TARGET_URL
-- and read at dispatch time, so a DCS serving several execution environments
-- could not express where a contract should go (DCS-IR-SI-05 specifies the
-- interface against target systemS), and a failure had no target to name.

CREATE TABLE contract_targets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL UNIQUE,
    url         TEXT         NOT NULL,
    description TEXT,
    -- Disabled entries stay referenceable so a contract that already names one
    -- keeps a readable destination; dispatch to a disabled target is refused.
    enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The contract's own deployment destination (DCS-FR-SM-12's automatic trigger
-- has no human present to choose one). ON DELETE RESTRICT: removing a registry
-- entry a contract still names would leave that contract undeployable with no
-- record of where it was meant to go — the admin must repoint it first.
ALTER TABLE contracts
    ADD COLUMN target_id uuid REFERENCES contract_targets(id) ON DELETE RESTRICT;

CREATE INDEX idx_contracts_target_id ON contracts(target_id);

-- A deployment records WHICH registry entry it went to, and separately the
-- endpoint as it stood at dispatch: editing an entry's URL later must not
-- rewrite what an earlier deployment actually did.
ALTER TABLE contract_deployments
    ADD COLUMN target_id uuid REFERENCES contract_targets(id) ON DELETE SET NULL;

-- Rows were written 'DISPATCHED' BEFORE the outbound call was attempted and a
-- failed call was only written to the process log, so a deployment the target
-- never received was indistinguishable from one it acknowledged. Status
-- 'DISPATCH_FAILED' marks those, and dispatch_error records why, so the
-- compliance monitor can raise an alert naming the reason (DCS-FR-CWE-31).
ALTER TABLE contract_deployments
    ADD COLUMN dispatch_error TEXT;

CREATE INDEX idx_contract_deployments_status ON contract_deployments(status);

-- contracts_effective is the read path for a contract's live state; the
-- designated target has to travel with it or deployment cannot see it.
-- CREATE OR REPLACE cannot add a column to an existing view, so it is dropped
-- and rebuilt. Views are derived — nothing is lost.
DROP VIEW IF EXISTS contracts_effective;
CREATE VIEW contracts_effective AS
SELECT
    did,
    origin,
    created_by,
    created_at,
    updated_at,
    start_date,
    exp_date,
    exp_policy,
    exp_notice_period,
    CASE
        WHEN exp_date <= CURRENT_TIMESTAMP
            AND state NOT IN ('TERMINATED', 'REJECTED', 'EXPIRED', 'WITHDRAWN', 'REVOKED')
            THEN 'EXPIRED'::contract_state
        ELSE state
        END AS state,
    contract_version,
    name,
    description,
    contract_data,
    search_vector,
    responsible,
    template_did,
    template_version,
    target_id
FROM contracts;
