# One-hop PoA trust and credential/contract status baseline.
#
# ADR-24 fixes the v1 delegation depth at one. ADR-29 makes the issuing
# organization authoritative for the organization claim. ADR-5 retains the
# XFSC Status List Service, while ADR-20 requires the credential to be checked
# again at the signing ceremony. Production trust profiles are deliberately
# outside this feature; the Dev Root is test-only.

@UC-14 @UC-04 @DCS-FR-SM-03 @DCS-FR-SM-04 @DCS-FR-SM-05 @DCS-IR-CI-09
Feature: One-hop PoA trust and status-list enforcement

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC1 @REQ-poa-one-hop-trust-and-statuslist-baseline-AC6 @DCS-FR-UC-14-1 @ADR-24 @ADR-29
  Scenario: A holder-bound active PoA is accepted only with a one-hop chain to the Dev Root and the signing organization
    Given contract "PoA One Hop Happy" is APPROVED and has completed a signing ceremony for signatory "PoaOneHop"
    When the signer publishes the OID4VP signing request for contract "PoA One Hop Happy"
    Then get http 200:Success code
    When the wallet signs contract "PoA One Hop Happy" by consuming the OID4VP signing request as "PoaOneHop"
    Then the contract "PoA One Hop Happy" has completed signing
    And the accepted PoA evidence for contract "PoA One Hop Happy" is holder-bound, active, organization-matched, and has one issuer leaf directly under the Dev Root

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC2 @two-instance @DCS-FR-SM-04 @DCS-IR-SI-06
  Scenario: A peer signature transports and revalidates the original one-hop PoA presentation context
    Given instance A and instance B are both running and trust each other
    When the initiator on instance A creates and offers a contract with instance B as counterparty
    Then the contract appears on instance B in state OFFERED within a few seconds
    When instance A drives the contract to APPROVED through its own local workflow
    And instance A applies a ceremony-backed signature to the contract
    Then instance B stores the revalidated urn:dcs:poa:v1 evidence with exactly instance A's original ceremony nonce and audience

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC2 @missing-poa-evidence @two-instance @DCS-FR-SM-04 @DCS-FR-SM-26
  Scenario: A peer signature without transferable PoA evidence is rejected before signature provenance is persisted
    Given instance A and instance B are both running and trust each other
    When the initiator on instance A creates and offers a contract with instance B as counterparty
    Then the contract appears on instance B in state OFFERED within a few seconds
    When instance A drives the contract to APPROVED through its own local workflow
    And instance A sends an otherwise valid peer-signed acceptance without transferable PoA evidence
    Then instance B rejects the signed acceptance before storing sync provenance because transferable PoA evidence is missing
    And instance B records a traceable Power of Attorney finding for the rejected peer signature

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC2 @invalid-poa-evidence @two-instance @DCS-FR-SM-04 @DCS-FR-SM-26
  Scenario Outline: Receiver-side revalidation rejects one invalid field in otherwise valid peer-signed PoA evidence
    Given instance A and instance B are both running and trust each other
    When the initiator on instance A creates and offers a contract with instance B as counterparty
    Then the contract appears on instance B in state OFFERED within a few seconds
    When instance A drives the contract to APPROVED through its own local workflow
    And instance A sends an otherwise valid peer-signed acceptance whose transferable PoA evidence has defect "<defect>"
    Then instance B rejects the peer-signed acceptance before storing sync provenance at the "<defect>" Power of Attorney gate
    And instance B records a traceable Power of Attorney finding for the rejected peer signature

    Examples:
      | defect            |
      | invalid signature |
      | wrong nonce       |
      | wrong audience    |
      | wrong organization|

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC3 @DCS-FR-SM-03 @DCS-FR-SM-04
  Scenario: A PoA rooted outside the Dev Root is rejected before the ceremony becomes verified
    Given contract "PoA Untrusted Root" has reached contract state "APPROVED"
    When a signing ceremony is started for the signing party of contract "PoA Untrusted Root"
    And the ceremony presentation is completed with a holder-bound PoA from an untrusted root
    Then the signing request is rejected because the Power of Attorney trust chain is untrusted
    And the rejected PoA carried a formally valid leaf-to-self-signed-root x5c chain
    And the ceremony remains unverified and no PoA authority is persisted

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC4 @REQ-poa-one-hop-trust-and-statuslist-baseline-AC6 @DCS-FR-SM-18 @DCS-IR-CI-09
  Scenario Outline: A newly revoked or suspended PoA is rejected by a fresh signing ceremony within five minutes
    Given contract "<contract>" has reached contract state "APPROVED"
    When a signing ceremony is started for the signing party of contract "<contract>"
    And the ceremony presentation uses a PoA whose XFSC status is changed to "<status>"
    Then the signing request is rejected for live credential status within 5 minutes
    And the ceremony remains unverified and no PoA authority is persisted

    Examples:
      | contract          | status    |
      | PoA Revoked Live  | revoked   |
      | PoA Suspended Live| suspended |

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC6 @DCS-FR-SM-05 @DCS-IR-CI-09
  Scenario: XFSC represents both PoA revocation and suspension with the same fail-closed binary bit
    Given contract "PoA XFSC Suspension" has reached contract state "APPROVED"
    When a signing ceremony is started for the signing party of contract "PoA XFSC Suspension"
    And the ceremony presentation uses a PoA whose XFSC status is changed to "suspended"
    Then the signing request is rejected for live credential status within 5 minutes
    And the XFSC binary status is normalized as revoked rather than active

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC6 @DCS-FR-SM-05 @DCS-IR-CI-09
  Scenario Outline: A signed W3C Bitstring Status List normalizes active, revoked, and suspended PoAs
    Given contract "<contract>" has reached contract state "APPROVED"
    When a signing ceremony is started for the signing party of contract "<contract>"
    And the ceremony presentation uses a PoA with signed W3C status "<status>"
    Then the W3C PoA status is normalized to decision "<decision>"

    Examples:
      | contract          | status    | decision |
      | PoA W3C Active    | active    | accepted |
      | PoA W3C Revoked   | revoked   | rejected |
      | PoA W3C Suspended | suspended | rejected |

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC7 @REQ-poa-one-hop-trust-and-statuslist-baseline-AC9 @DCS-FR-UC-01-4 @DCS-IR-CI-09
  Scenario Outline: Unusable PoA status evidence fails closed
    Given contract "<contract>" has reached contract state "APPROVED"
    When a signing ceremony is started for the signing party of contract "<contract>"
    And the ceremony presentation uses otherwise valid PoA evidence with status defect "<defect>"
    Then the signing request is rejected for a traceable status-evidence reason
    And the ceremony remains unverified and no PoA authority is persisted

    Examples:
      | contract                | defect           |
      | PoA Status Missing      | missing reference|
      | PoA Status Bad Ref      | bad reference    |
      | PoA Status Bad Signature| bad signature    |
      | PoA Status Untrusted Sig| untrusted signature |
      | PoA Status Unknown      | unknown mechanism|
      | PoA Status Bad Encoding | bad encoding     |
      | PoA Status Unavailable  | unavailable      |

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC8 @DCS-OR-C2PA-005 @DCS-IR-CI-09
  Scenario: Suspending a contract durably publishes its deterministic XFSC bit within five minutes
    Given contract "Contract Status Suspended" has reached contract state "SIGNED"
    When the signature for contract "Contract Status Suspended" is revoked as a post-sign C2PA update
    Then a durable status-publication queue entry exists for contract "Contract Status Suspended" and lifecycle "suspended"
    And the deterministic XFSC status bit for contract "Contract Status Suspended" is revoked within 5 minutes

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC8 @DCS-OR-C2PA-005 @DCS-IR-CI-09
  Scenario: Terminating a contract durably publishes its deterministic XFSC bit within five minutes
    Given contract "Contract Status Terminated" has reached contract state "SIGNED"
    When the contract manager terminates contract "Contract Status Terminated" with reason "BDD terminal status publication"
    Then a durable status-publication queue entry exists for contract "Contract Status Terminated" and lifecycle "terminated"
    And the deterministic XFSC status bit for contract "Contract Status Terminated" is revoked within 5 minutes

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC8 @idempotent-status-publication @DCS-OR-C2PA-005 @DCS-IR-CI-09
  Scenario: Concurrent retries of the same desired contract status produce one logical publication and no duplicate effect
    Given contract "Contract Status Idempotent" has reached contract state "SIGNED"
    When two concurrent manager requests enqueue the same "terminated" status for contract "Contract Status Idempotent"
    Then both accepted lifecycle requests settle as exactly one logical "terminated" publication for contract "Contract Status Idempotent"
    And the deterministic XFSC status bit for contract "Contract Status Idempotent" is revoked within 5 minutes

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC9 @pdf-verifier-status @DCS-OR-C2PA-006
  Scenario: Verification separates live revocation from the terminated lifecycle banner
    Given contract "Verifier Status Separation" has reached contract state "SIGNED"
    When the contract manager terminates contract "Verifier Status Separation" with reason "BDD verifier separation"
    And the Contract Manager exports and verifies contract "Verifier Status Separation" as PDF
    Then the verifier reports live status "revoked" separately from lifecycle banner "terminated"

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC9 @pdf-verifier-status @DCS-OR-C2PA-006
  Scenario Outline: Verification keeps an active live bit separate from non-terminal lifecycle banners
    Given contract "<contract>" has reached contract state "<state>"
    When the Contract Manager exports and verifies contract "<contract>" as PDF
    Then the verifier reports live status "active" separately from lifecycle banner "<banner>"

    Examples:
      | contract                    | state  | banner |
      | Verifier Draft Separation   | DRAFT  | draft  |
      | Verifier Active Separation  | SIGNED | active |

  @REQ-poa-one-hop-trust-and-statuslist-baseline-AC9 @pdf-verifier-status @status-service-outage @DCS-OR-C2PA-006 @DCS-IR-CI-09
  Scenario: PDF verification fails closed when its genuine XFSC status service is unavailable
    Given contract "Verifier Status Unavailable" has reached contract state "SIGNED"
    When the Contract Manager exports contract "Verifier Status Unavailable" as PDF while XFSC is available
    And the genuine XFSC status service becomes unavailable only for PDF verification
    Then the verifier never reports the contract active and fails closed on the unavailable live status
