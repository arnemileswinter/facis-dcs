-- A DRAFT never expires: the contract state machine (ADR-2) reaches EXPIRED
-- only from a contract in force, and the expiry cron already skips DRAFT
-- (ReadExpiredContracts). The effective views read a DRAFT past its exp_date
-- as EXPIRED, which blocked editing the draft and offered it for renewal. The
-- views now exclude DRAFT like the cron does. Definitions mirror
-- 20260706b_contracts_effective_views_new_terminal_states.sql and
-- 20260720_contract_content_updated_at.sql; only the exclusion list changes.

CREATE OR REPLACE VIEW contracts_effective AS
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
            AND state NOT IN ('DRAFT', 'TERMINATED', 'REJECTED', 'EXPIRED', 'WITHDRAWN', 'REVOKED')
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
    template_version
FROM contracts;

CREATE OR REPLACE VIEW contracts_effective_metadata AS
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
            AND state NOT IN ('DRAFT', 'TERMINATED', 'REJECTED', 'EXPIRED', 'WITHDRAWN', 'REVOKED')
            THEN 'EXPIRED'::contract_state
        ELSE state
        END AS state,
    contract_version,
    name,
    description,
    responsible,
    template_did,
    template_version
FROM contracts;

CREATE OR REPLACE VIEW contracts_effective_process_data AS
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
            AND state NOT IN ('DRAFT', 'TERMINATED', 'REJECTED', 'EXPIRED', 'WITHDRAWN', 'REVOKED')
            THEN 'EXPIRED'::contract_state
        ELSE state
        END AS state,
    contract_version,
    content_updated_at
FROM contracts;
