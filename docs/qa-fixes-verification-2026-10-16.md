# QA verification: P04, P06 and F02 — 2026-10-16

Status: **passed within the agreed non-BDD scope**. Independent revision
reported no findings. BDD was not run, as explicitly requested. This is local
verification, not acceptance of an existing QA deployment.

## Results

| Problem | Verified behavior and evidence |
|---|---|
| P04 | The summary survives both dialog confirmations and a mocked reload; cancellation/Escape sends no write, and a failed write is visible. `frontend/ClientApp/e2e/qa-archive-audit-regressions.spec.ts:48`. |
| P06 | Complete frozen report, original executor bytes, encrypted SQL snapshot and archived report, persisted audit/export events, and unreadability after participating-key shredding. Real PostgreSQL/CEK round trip: `backend/internal/service/qa_audit_report_integration_test.go:25`. Download bytes/filenames and JSON error handling: `frontend/ClientApp/e2e/qa-archive-audit-regressions.spec.ts:113`. Separate Go tests cover rendering, HTTP media types and OpenAPI declarations. |
| F02 | Signed JWT authentication, role/scope denials, distinct failed-credential counts, lockout, Auditor-only reads, justification, metadata and pagination against PostgreSQL: `backend/internal/service/qa_access_integration_test.go:82`. Browser checks cover display, keyboard, errors/retry and 390/1280px layouts: `frontend/ClientApp/e2e/qa-archive-audit-regressions.spec.ts:160`. |

The integration tests found a real F02 failure-window bug: UTC timestamps were
compared with local-time boundaries, causing failed attempts to be missed.
The shared repository now normalizes all time parameters to UTC
(`backend/internal/auth/db/pq/accessattemptrepository.go:25`); the regression
explicitly compares UTC+2 and UTC (`qa_access_integration_test.go:171` in the
service directory above).

Frontend build blockers were corrected without weakening assertions: explicit
optional editor input (`frontend/ClientApp/src/views/archive/ArchiveDashboardView.vue:86`),
fixture types/required fixture fields, an array access compatible with the
configured JavaScript target
(`frontend/ClientApp/src/core/components/WorkflowStageBanner.vue:24`), and import
formatting. Missing local development dependencies were restored from the
existing lockfile; no dependency versions were changed.

## Reproduce the checks

Run frontend commands from `frontend/ClientApp/`:

```sh
npm test
npx --no-install playwright test e2e/qa-archive-audit-regressions.spec.ts e2e/audit-layout-and-template-review-approval-verification.spec.ts --workers=2 --reporter=list
npm run lint
npm run format:check
npm run build
```

Results: **173 Vitest tests in 30 files**, **18 Chromium tests** (10 new QA
regressions plus 8 existing checks), lint, formatting and build passed.
The build retains a non-failing bundle-size warning.

Run backend commands from `backend/`. Set `QA_DATABASE_URL` to the connection
URL of a disposable PostgreSQL 16 test database with schema creation rights.
Both new integration tests create and remove their own schemas and execute the
real migrations. Without this variable they **skip**, which is not evidence of
integration success (`backend/internal/service/qa_access_integration_test.go:35`).

```sh
go test -count=1 -v ./internal/service
go test -v ./internal/auth ./internal/auth/db/pq ./internal/base/event ./internal/base/artifactstore ./internal/base/envelope ./cmd/dcs ./design ./migrations ./internal/middleware ./internal/base/datatype/userrole
go mod tidy -diff
./bin/golangci-lint run --new-from-rev=20905305
```

The complete service suite passed **116 top-level tests**, including both new
PostgreSQL tests. The other listed packages passed; two optional HSM tests
skipped because `SOFTHSM2_CONF` was unset. Tidy reported no changes and branch
lint reported **0 issues**. An initial sandbox restriction prevented service
HTTP test listeners; the permitted rerun and final complete suite passed.

Observed logs for this session: `/tmp/qa-final-vitest.log`,
`/tmp/qa-final-playwright.log`, `/tmp/qa-final-frontend-{lint,format,build}.log`,
`/tmp/qa-backend-service-complete-final.log`,
`/tmp/qa-backend-{tidy,lint}-complete-final.log`,
`/tmp/qa-backend-tests.log`, `/tmp/qa-backend-service-tests.log`,
`/tmp/qa-backend-auth-final.log`, `/tmp/qa-backend-migration-role-tests.log`.
These are local transient logs, not versioned artifacts. The temporary test
database container was removed and the frontend test servers were stopped.

## Boundaries

- Playwright uses mocked HTTP responses. It proves browser behavior and
  download transport, not a connected DCS stack or PDF document validity.
- SQL, migrations, signed JWT validation, CEK persistence and encryption are
  real in the integration tests. The external key provider and IPFS blob
  transport are fixtures; live HSM/Kubo integration was not exercised.
- Migrations ran only in fresh temporary test schemas. No existing deployment
  was migrated or reset. The historical QA-500 cause remains unproven.
- BDD acceptance and external deployment acceptance remain open. The existing
  PDF font's Unicode coverage was not expanded.

No new architecture decision was needed. Existing ADR-28 remains authoritative;
only its stale verification reference was updated.

