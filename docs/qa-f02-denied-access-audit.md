# F02: denied access attempts

Status: verified on 2026-10-16 in the agreed non-BDD scope; BDD acceptance remains open.
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
automatic deletion. Migrations were executed only in fresh temporary PostgreSQL 16
test schemas; no existing deployment was migrated or reset.

## Verification

The real PostgreSQL/JWT integration exposed a timezone defect in failure
counting: local-time boundaries were compared with UTC values in timestamp
columns. Repository time parameters now use UTC consistently, including
insertion, failure counting, retention and cursor boundaries. The regression
compares UTC and explicit UTC+2 failure windows and verifies lockout
(`backend/internal/auth/db/pq/accessattemptrepository.go:25`,
`backend/internal/service/qa_access_integration_test.go:171`). Permission
denials remain separate from invalid credentials.

The service suite, focused authentication tests, Vue tests and browser
regressions passed. Browser checks cover roles, justification, outcomes,
pagination, errors/retry, keyboard access and narrow/wide layouts. AC1–AC6 BDD
specifications remain unexecuted at the user's request. Independent review
reported no findings within this non-BDD scope.

See the [verification report](qa-fixes-verification-2026-10-16.md) for commands,
counts and limits.

## Subsequent connected acceptance

The [connected local acceptance report](qa-fixes-connected-verification-2026-10-16.md)
records the later real-service checks for all seven QA fixes. The results and
fixture boundaries above describe the earlier run and remain unchanged. BDD
was still not executed; external deployment acceptance remains open.
