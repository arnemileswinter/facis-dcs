# F02: denied access attempts

Status: implemented in source; verification deferred by explicit user request.
Decision date: 2026-10-15.

Source: `QA_DCS_SRS_Requested_Fixes_06_10_2026.xlsx`, sheet
`Problems & solutions`, C18:D18. Related requirements: DCS-FR-CSA-02 and
DCS-FR-PACM-04.

## Cause and decision

The existing `access_attempts.success` flag records credential validity.
Role denials deliberately kept this flag true so they would not contribute
to invalid-credential IP lockout. Treating it as the access decision therefore
misrepresented denials. PAC also rejects some scopes after JWT authorization,
and the application previously had no read endpoint or view for access attempts.

The separate `outcome` field now distinguishes `SUCCESS` (access allowed),
`DENIED` (permission refused) and `FAILED` (authentication failed). The
credential flag and lockout queries retain their existing semantics.
Failed credentials and JWT role denials are recorded immediately; authenticated
requests are finalized once by the existing endpoint middleware. A returned
`forbidden` or `unauthorized` error records DENIED, including PAC scope denials.
A business error after permitted access does not turn into an authentication
failure. Logging retains the existing best-effort behavior on database errors.

## API and view

`GET /pac/access-attempts` requires the human `Auditor` role and a nonblank
`justification`. Archive Managers and System Auditors cannot read this global
list, consistent with the role separation in [ADR-16](adr-16-audit-checkpoints-external-anchoring.md).

The optional `limit` defaults to 25 and is bounded to 1–100. The opaque
`cursor` identifies a position in descending `(attempted_at, id)` order.
Responses contain `attempts` and an optional `next_cursor`. Each attempt exposes
its timestamp, known identity and roles, service/action, scope, resource DID,
justification and outcome. Neither IP addresses nor the credential-success flag
are exposed. Reading the list is itself logged; cursor pagination prevents
these new entries from shifting subsequent pages.

The Audit view includes an Auditor-only “Access attempts” section. Loading
requires an explicit justification; results distinguish loading, empty and
failed requests, label each outcome, and support next/previous navigation.

Migration `20261015_access_attempt_outcome.sql` adds the outcome and pagination
index. It requires an empty `access_attempts` table: old authorization outcomes
cannot be reconstructed from credential validity. There is no backfill or
automatic deletion. No migration or database reset was executed.

## Verification still required

AC1–AC6 have BDD specifications in `08_audit_compliance/denied_access_audit.feature`,
the existing audit-access scenario and the authentication lockout scenario.
They cover denial classification, one record per attempt, metadata, unchanged
credential failure counts, role restrictions, pagination and read justification.
Unit-test sources cover finalization, metadata extraction, cursor validation
and response pagination. AC7 has focused Vue component tests; browser layout,
keyboard behavior and existing audit/export regressions remain to be checked.

No tests, builds, lint or pre-commit checks were run. Goa generation, Go
formatting and formatting of the changed frontend files were performed.
Independent acceptance and revision remain deferred; no green result is claimed.
