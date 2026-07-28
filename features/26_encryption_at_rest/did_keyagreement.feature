# The CEK wrap key in the DID document (ADR-28): gendid publishes a third
# verification method — the HSM's P-256 key-agreement PUBLIC key (label
# dcs-ecdh, env DCS_HSM_KEY_ECDH) — referenced by a top-level `keyAgreement`
# relation. Peers wrap content-encryption keys to this key when shipping
# contracts (ECDH-ES+A256KW); unwrapping requires the instance's HSM.
#
# The first verification method must stay #dev-key-1: several backend paths
# consume VerificationMethod[0] unconditionally (eIDAS certificate check,
# challenge-response verification), so new methods are APPENDED, never
# reordered.
#
# Read-only, unauthenticated scenario (GET /.well-known/did.json): it mutates
# no per-identity state, so the dedicated-organization rule for state-mutating
# scenarios does not apply here.

@DCS-NFR-SEC-14
Feature: The served DID document publishes the key-agreement method for CEK wrapping

  Scenario: did.json carries the dcs-ecdh verification method under keyAgreement without disturbing the existing methods
    When I fetch this instance's served DID document
    Then the DID document's keyAgreement relation names exactly one verification method with fragment "dcs-ecdh"
    And that key-agreement verification method is a P-256 JsonWebKey2020 appended after the existing verification methods
    And the first verification method keeps the fragment "dev-key-1"
