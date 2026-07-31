-- Mutual settlement evidence: a party's signed statement that its OWN
-- workflow reached the settlement milestone (NEGOTIATION -> SUBMITTED) over a
-- named contract document. Same shape as contract_sync_signatures: a JAdES
-- signed with the instance's did:web assertion key, verified against the
-- peer's published key before it is stored, kept verbatim so it stays
-- independently re-verifiable.
--
-- Deliberately NOT a column or a kind flag on contract_sync_signatures: that
-- table is PRIMARY KEY (did) with a single jades_signature per contract, so a
-- settlement and a signature would evict each other; a settlement is also
-- written for THIS instance (is_local) as the baseline the signing gate
-- compares peer statements against, which has no place in a table whose
-- semantics are "what the peer sent us"; and a settlement is superseded by
-- renegotiation while a signature is terminal.
CREATE TABLE IF NOT EXISTS contract_settlement_statements
(
    did              VARCHAR(255) NOT NULL CHECK (did <> ''),
    -- The settling party's did:web. Keyed per party, so both sides' statements
    -- coexist for one contract.
    party_did        VARCHAR(255) NOT NULL CHECK (party_did <> ''),
    -- TRUE for this instance's own statement (the baseline). Renegotiation
    -- overwrites it, which is what strands a stale peer statement.
    is_local         BOOLEAN      NOT NULL,
    -- sha256 over contract_data as persisted, hex. The cross-instance-stable
    -- identity of a document version; contract_version is not comparable
    -- across the boundary.
    document_hash    CHAR(64)     NOT NULL CHECK (document_hash <> ''),
    -- The settling party's own local counter. Provenance only, never compared.
    contract_version INT          NOT NULL,
    -- Signer-asserted, read from the signed artifact; a statement must be
    -- strictly newer than the one it replaces.
    settled_at       TIMESTAMP    NOT NULL,
    jades_signature  TEXT         NOT NULL CHECK (jades_signature <> ''),
    -- Own statements only: NULL until every counterparty accepted the ship.
    -- This is the settlement ship's retry queue — sync_fails is keyed by did
    -- alone and its success path deletes the row, so sharing it between two
    -- ship kinds would let one clear the other's retry entry.
    shipped_at       TIMESTAMP,
    received_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (did, party_did)
);

CREATE INDEX IF NOT EXISTS idx_contract_settlement_unshipped
    ON contract_settlement_statements (did)
    WHERE is_local AND shipped_at IS NULL;
