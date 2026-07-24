# Signing acceptance-path hardening negative scenarios (ADR-20): the ceremony
# callback must reject a certificate that doesn't name the verified PID, an
# invalid JAdES, a revoked PID, a signature below the contract's required
# level, and a replayed (already-consumed) callback. Nonce binding and TBS
# byte pinning are covered in oid4vp_document_retrieval.feature and
# real_signing_vertical.feature's request-nonce scenarios.

@UC-04 @DCS-FR-SM-16 @DCS-FR-SM-18 @ADR-20
Feature: Signing acceptance-path hardening

  @ADR-20
  Scenario: A certificate that does not name the ceremony's verified PID is rejected
    Given contract "SAH Cert Mismatch Contract" is APPROVED and has completed a signing ceremony for signatory "SAHCertMismatch"
    When the signer publishes the OID4VP signing request for contract "SAH Cert Mismatch Contract"
    Then get http 200:Success code
    When the wallet signs contract "SAH Cert Mismatch Contract" as "SAHCertMismatchWrongCert" with a certificate naming "Someone" "Else"
    Then the ceremony callback for contract "SAH Cert Mismatch Contract" rejects the signature

  @ADR-20
  Scenario: An invalid JAdES is rejected even when the PAdES signature is genuinely valid
    Given contract "SAH Invalid JAdES Contract" is APPROVED and has completed a signing ceremony for signatory "SAHInvalidJAdES"
    When the signer publishes the OID4VP signing request for contract "SAH Invalid JAdES Contract"
    Then get http 200:Success code
    When a raw invalid JAdES is submitted for the signed document on contract "SAH Invalid JAdES Contract"
    Then the ceremony callback for contract "SAH Invalid JAdES Contract" rejects the signature

  @ADR-20 @DCS-FR-SM-18
  Scenario: A revoked PID is rejected at the ceremony presentation
    Given contract "SAH Revoked PID Contract" has reached contract state "APPROVED"
    When I start a signing ceremony for contract "SAH Revoked PID Contract" field "SAHRevokedPidField" as "Contract Signer"
    And the PID for contract "SAH Revoked PID Contract"'s ceremony is revoked before it is presented
    Then the ceremony presentation for contract "SAH Revoked PID Contract" is rejected

  @ADR-20 @SM-01
  Scenario: An AES signature is rejected on a field requiring QES
    Given contract "SAH QES Required Contract" is APPROVED and has completed a signing ceremony requiring "QES" for signatory "SAHQesRequired"
    When the signer publishes the OID4VP signing request for contract "SAH QES Required Contract"
    Then get http 200:Success code
    When the wallet signs contract "SAH QES Required Contract" as "SAHQesRequired" with an ordinary AES signature
    Then the ceremony callback for contract "SAH QES Required Contract" rejects the signature

  @ADR-20
  Scenario: A replayed callback for an already-consumed ceremony is rejected
    Given contract "SAH Replay Contract" is APPROVED and has completed a signing ceremony for signatory "SAHReplay"
    When the signer publishes the OID4VP signing request for contract "SAH Replay Contract"
    Then get http 200:Success code
    When the wallet signs contract "SAH Replay Contract" as "SAHReplay" and the same callback is replayed
    Then the ceremony callback for contract "SAH Replay Contract" rejects the signature
