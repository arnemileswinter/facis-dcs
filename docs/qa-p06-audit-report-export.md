# P06: complete audit report export

Status: verified on 2026-10-16 in the agreed non-BDD scope; live-stack acceptance remains open.
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
existing rows. The migration was executed only in fresh temporary PostgreSQL 16
test schemas; no existing deployment was migrated.

## Verification

Focused Go tests and the complete service suite passed, including the new
PostgreSQL snapshot/export/shredding round trip. Browser tests passed for
JSON/CSV/PDF download bytes and filenames and 404/500 error presentation.
HTTP media types, OpenAPI declarations and report rendering have separate Go
test coverage. Original deferred-verdict/ODRL summary tests are retained.

See the [2026-10-16 verification report](qa-fixes-verification-2026-10-16.md)
for commands, counts and boundaries. Browser responses were mocked; the SQL
test uses real CEKs/encryption with test key-provider and blob-transport
fixtures. BDD was explicitly excluded by the user. Neither these results nor
the local configuration assessment establish the historical QA-500 cause.
The existing standard Helvetica PDF font does not provide full Unicode glyph
coverage.
