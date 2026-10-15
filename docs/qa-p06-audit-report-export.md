# P06: complete audit report export

Status: implemented in source, verification deferred by explicit user request.
Decision date: 2026-10-15.

The source is row 7 (C7:D7) of
`QA_DCS_SRS_Requested_Fixes_06_10_2026.xlsx`: `/pac/report` reportedly returned
500 in every audit view, lacked declared response media types and omitted the
lifecycle timeline. Related requirements: DCS-FR-PACM-05, DCS-FR-PACM-01,
DCS-FR-CSA-24, DCS-FR-TR-05 and DCS-IR-PACM-02.

## Local version and configuration assessment

The QA commit reference `07b0df13` was not present in the available Git
history, so the QA deployment cannot be matched to a local revision.
Commit `b6439b56` already replaced the old IPFS document manager with direct
Kubo access. [ADR-36](adr-36-artifacts-are-stored-in-kubo-directly.md)
documents the old shared timeout/locking behavior and resulting 5xx errors.
The historical report failure could therefore already have been resolved by
that shared dependency change. This is a plausible explanation, not a proven
cause of the QA observation. No external QA checks were performed after the
user requested a repository-only assessment.

A separate local chart discrepancy remains: declared `ipfs.mfsBaseURL` and
`ipfs.mfsBaseURLSecretRef` are not consumed by the endpoint helper in
`deployment/helm/templates/_helpers.tpl`. It uses `ipfsClient.mfsBaseURL`, or
the bundled service when `ipfs.enabled` is true. With only an external secret
reference and the subchart disabled, the current backend has an empty endpoint
and fails startup; that does not establish the cause of an isolated report
500. No speculative IPFS or deployment changes are part of this patch.

The current HTTP path had an independently identifiable media-type defect:
constructing Goa's fallback encoder could modify a preselected CSV/PDF header.
Report responses now preserve the requested media type, and error responses
use JSON even when the requested report format or Accept header differs.

## Implemented contract

- The user selected a complete JSON report. It contains `reportId`, scope,
  generation metadata, original executor provenance and receipt, findings
  including result/evidence references, resource counts, and `events`.
- Events come from the saved executor request, ordered by timestamp then ID.
  Their actors and details retain the recorded evidence. CHECK evidence is
  not counted again as lifecycle events or executor findings.
- CSV and paginated PDF use the same saved run. Export performs no new audit
  and does not collect subsequently added lifecycle events. Generation time
  and encrypted artifact bytes may differ between exports.
- `requestHash` and `responseHash` identify the original byte sequences.
  `PAC_REPORT_GENERATED.report_hash` covers the actual download bytes; its CID
  points to the protected archive artifact.
- OpenAPI 3 declares JSON, CSV and PDF success content. OpenAPI 2 declares
  those produced media types for the download. Source metadata and the Goa
  generation plugin live under `backend/design/`; generated files are not
  edited manually.
- Stored requests, responses and reports use the existing CEKs as described
  in [ADR-28](adr-28-ipfs-encryption-at-rest-key-shredding.md). The user
  explicitly accepted that deleting one participating resource makes the
  whole affected saved report unreadable. Export then returns `404/not_found`.

Migration `20261015_pac_audit_snapshot.sql` replaces the plaintext response
and receipt columns with encrypted `snapshot_bytes`. It requires an empty
`pac_audit_runs` table: old runs lack the original request evidence, and there
is deliberately no backfill or legacy fallback. This change does not delete
existing rows or apply the migration to a running database.

## Verification still required

Test sources cover report contents and hashes, timeline ordering, pagination,
HTTP media types and JSON errors, OpenAPI generation, snapshot protection,
shredding and repeated exports. The BDD scenario uses a real workflow and
standard archive deletion. Original deferred-verdict/ODRL summary tests are
retained.

No tests, BDD runs, build, lint or pre-commit checks were run for this change.
Only Go source formatting and the required Goa generation were performed.
The existing UI download path is unchanged; AC6 browser acceptance remains
pending. The simple PDF renderer still uses its existing standard Helvetica
font; full Unicode glyph coverage is outside this change.
