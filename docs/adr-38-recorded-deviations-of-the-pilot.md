# ADR-38: Recorded deviations of the pilot

Status: Accepted (2026-10-31).

## Context

Some SRS requirements are met in part or not at all by decision of the
product owner. Other ADRs record most of these decisions (ADR-17, ADR-21,
ADR-24, ADR-25, ADR-28, ADR-35, ADR-37). The decisions below have no other
ADR. The deviation register in `docs/TRACEABILITY_SRS_BDD.md` links here.

## Decisions

### 1. The instance signs the template provenance credential (DCS-FR-TR-09)

At registration, the DCS instance issues one W3C VC in JSON-LD
(ecdsa-rdfc-2019) per template version. The credential records the creator,
reviewer, approver and registrar claims, the content hash and a link to the
credential of the previous version
(`backend/internal/templaterepository/command/register.go`,
`IssueTemplateProvenanceVC` in `provenance.go`). The instance key signs it.
The role holders do not sign their own claims.

Reason (product-owner scope decision): each role holder acts after a
verified OID4VP login, and the audit trail records each act. A signature per
role holder needs a wallet signing ceremony for every review and approval
step. The pilot does not ask the holder for that ceremony; the instance
attests the claims that it recorded.

### 2. Signatures are applied in sequence, not in parallel (DCS-FR-SM-17)

Several signers sign the same contract, each in a separate PAdES signature,
and each signature is verifiable on its own. The signatures are applied one
after the other. A signature over a superseded revision is refused
(`assertLatestRevision`, `TestAssertLatestRevisionRefusesAStaleRevision`).

Reason (product-owner scope decision): each PAdES signature is an
incremental update whose byte range covers all earlier revisions. Two
signatures prepared over the same revision collide: the second one does not
cover the first one. Parallel signing needs a different container (for
example detached CAdES signatures per signer), which the pilot does not
provide.

### 3. Hardware roots of trust and secure boot are platform scope (DCS-IR-HI-03, DCS-NFR-SEC-17)

The DCS does not seal service credentials to a TPM 2.0 or Secure Enclave, does
not do remote attestation, and does not check secure boot. Service keys are in
the PKCS#11 token (DCS-IR-HI-01, ADR-1). The DCS runs as containers on the
operator's Kubernetes nodes.

Reason (product-owner scope decision): TPM sealing, attestation and secure
boot are properties of the host firmware, the operating system and the
cluster. The operator who runs the nodes provides them; a container cannot
provide them for its host.

### 4. The EU trusted lists and the time-stamp authority are reached over TLS 1.2 or 1.3 (DCS-IR-CI-01)

Every outbound HTTPS client of the backend requires TLS 1.3, except two. The
RFC 3161 time-stamp client accepts TLS 1.2 and TLS 1.3 (`NewClient` in
`backend/internal/base/tsa/tsa.go`, `TestTSAClientAcceptsTLS12`); the TSA signs
every time-stamp token, and the backend verifies that signature against the
configured TSA certificate. The client that downloads the EU List of Trusted
Lists and the national trusted lists also accepts TLS 1.2 and TLS 1.3 and
verifies the server certificate (`tslClient` in
`backend/internal/base/identity/eutrustpool.go`,
`TestTrustedListClientAcceptsTLS12`). It does not verify the XAdES signatures
of the lists.

Reason: not every time-stamp authority offers TLS 1.3. Each member state
publishes its national trusted list on its own server, and not all of these
servers offer TLS 1.3. With TLS 1.3 only, the
lists of these member states are not available, and the signer certificates
of their trust service providers are not trusted.

### 5. The CRL distribution point is an `http://` URL (DCS-IR-CI-01, DCS-NFR-SEC-01)

The certificates that the provisioning job issues name the CRL of the
instance in their `crlDistributionPoints` extension as an `http://` URL of
the backend Service (`CRL_URL` in
`deployment/helm/templates/hsm-provision-job.yaml`,
`scripts/c2pa-cert-provision.sh`).

Reason: RFC 5280 CRLs are signed by the issuing CA, and a relying party
verifies that signature. CRLs are served over plain HTTP so that revocation
checking does not depend on TLS: a check over HTTPS needs the certificate
of the CRL server, which can itself need a revocation check.

## Consequences

- A template user verifies the provenance credential against the instance
  key, not against a key of each role holder.
- A multi-signer contract takes one signing turn per signer.
- An operator who needs a hardware root of trust or secure boot provides it
  in the platform that runs the DCS.
- The time-stamp requests and the trusted-list download are the outbound
  HTTPS connections of the backend that can use TLS 1.2.
- A CRL fetch is plain HTTP; the CRL signature protects its content.
