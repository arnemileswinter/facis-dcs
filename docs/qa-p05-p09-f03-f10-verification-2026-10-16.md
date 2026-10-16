# QA verification: P05, P09, F03 and F10 — 2026-10-16

Status: **passed within the agreed non-BDD scope**. Independent backend and
frontend verification passed; independent revision reported no findings.
BDD was not run. This is local verification, not acceptance of a QA deployment.
See also the [user guide](user-manual/archive-search-template-classification-evidence.md).

## Results

| Problem | Verified behavior and evidence |
|---|---|
| P05 | Observers can search archives and open the exact frozen version, with read-only document preview and machine-readable content. Detail reads decrypt the archived artifact and check its hash and identity; they never substitute current contract data. `backend/internal/service/archive_detail.go:16`; PostgreSQL regression `backend/internal/service/qa_contract_access_integration_test.go:175`; browser regression `frontend/ClientApp/e2e/qa-next-fixes.spec.ts:77`. Errors clear old content and stale responses cannot replace the current selection. |
| P09 | Explicit mandatory `party_type` (`SINGLE_PARTY` or `MULTI_PARTY`) is independent of Contract/Component classification. Storage, history, copying, catalogue import and local/catalogue filtering retain it. There is no implicit choice. PostgreSQL, update/history and catalogue-import regressions: `backend/internal/service/qa_party_type_integration_test.go:22`, `:91`, `:123`. Combined text/party filters and stale responses: `frontend/ClientApp/e2e/qa-next-fixes.spec.ts:185`. |
| F03 | Organization-based authorization is shared by SQL lists and individual reads, including tasks, history, signing/ceremony status, audit, archive statistics, reports, PDF and bundles. Existing privileged roles and trusted remote-adoption access remain; users in the same organization are not isolated from each other. `backend/internal/base/contractreadpolicy/policy.go:30`; PostgreSQL/JWT/HTTP and DENIED regressions: `backend/internal/service/qa_contract_access_integration_test.go:37`, `qa_archive_access_integration_test.go:36`, `qa_read_http_regressions_test.go:41`. |
| F10 | Evidence stores a file, description, recording metadata and server-calculated SHA-256 in an encrypted contract-scoped package. An append-only mapping and audit event reference it. Managers can upload evidence and use the contract audit view; permitted readers can retrieve exact original bytes. Key shredding makes evidence unreadable. `backend/internal/contractworkflowengine/command/recordevidence.go:78`; PostgreSQL/crypto regression `backend/internal/service/qa_evidence_integration_test.go:35`; browser validation/retry/download checks `frontend/ClientApp/e2e/qa-next-fixes.spec.ts:246`. |

F03 also closes indirect disclosure through archive recent actions and audit
inputs. Saved reports are indivisible: a report containing an unauthorized
resource, including an erased placeholder, is rejected in full rather than
silently truncated or replaced by an older report
(`backend/internal/service/pac_contract_access.go:48`).
F10 expands contract audit access, not the global audit workstation. Pure
Auditor/Compliance accounts do not acquire a new general contract-view route;
existing additive role access and compliance entry points remain.

## Reproduce the checks

From `backend/`, set `QA_DATABASE_URL` to a disposable PostgreSQL 16 database
with schema creation rights. Integration fixtures create/remove isolated schemas
and run real migrations. Omitting the variable skips integration tests and is
not evidence of success.

```sh
export QA_DATABASE_URL='postgres://USER@HOST:PORT/TEST_DATABASE?sslmode=disable'
export GOCACHE=/tmp/qa-fixes-go-cache
go test -count=1 -json ./cmd/dcs ./design ./internal/service ./internal/base/conf ./internal/base/contractreadpolicy ./internal/base/envelope ./internal/base/artifactstore ./internal/base/validation ./internal/auth/... ./internal/templaterepository/... ./internal/templatecatalogueintegration/... ./internal/fcasset ./internal/contractworkflowengine/command/... ./internal/contractworkflowengine/query/... ./internal/contractworkflowengine/db/... ./internal/signingmanagement/query/... ./internal/signingmanagement/db/... ./internal/bundleexport ./internal/pdfgeneration/... ./internal/processauditandcompliance/...
go mod tidy -diff
GOLANGCI_LINT_CACHE=/tmp/qa-fixes-lint-cache ./bin/golangci-lint run --new-from-rev=HEAD
```

The initial scope verification passed **1,152 tests/subtests in 36 packages**;
two optional HSM tests skipped. After the final corrections, the affected
packages were independently rerun with:

```sh
go test -count=1 -json ./internal/service ./design ./internal/bundleexport ./internal/signingmanagement/query/... ./internal/templaterepository/command/... ./internal/templatecatalogueintegration/query/...
```

That final run passed **199 tests/subtests in 6 packages, with no skips**.
These are repeat coverage, not 199 additional tests to add to 1,152.
Tidy produced no diff; the repository hook linter reported **0 issues**.

From `frontend/ClientApp/`:

```sh
npm test
npx --no-install playwright test e2e/qa-next-fixes.spec.ts e2e/qa-archive-audit-regressions.spec.ts --workers=2 --reporter=list
npx --no-install playwright test e2e/audit-layout-and-template-review-approval-verification.spec.ts e2e/ui-truthful-action-states.spec.ts --workers=2 --reporter=list
npm run lint
npm run format:check
npm run build
```

Results: **193 Vitest tests in 36 files**, **33 Chromium tests** (22 + 11),
lint, formatting and build passed. The build retains existing non-failing
`config.js` and bundle-size warnings. The selected browser specs mock HTTP;
they do not require a running BDD stack.

Observed session logs: `/tmp/qa-next-independent-backend-tests.jsonl`,
`/tmp/qa-next-independent-audit-validation.jsonl`,
`/tmp/qa-next-independent-backend-final.jsonl`, and
`/tmp/qa-ui-final-{test,lint,format-check,build,playwright,other-playwright}.log`.
These are transient local logs, not versioned evidence artifacts.
The temporary database and frontend servers were stopped after verification.

## Migration, configuration and boundaries

- P09 adds required columns without defaults or backfill. Both template tables
  must be empty when applying `backend/migrations/sql/20261016_template_party_type.sql:1`.
  No existing database was migrated or reset. Existing clients/fixtures must
  explicitly supply the party classification when creating templates.
- F10 adds the evidence mapping in
  `backend/migrations/sql/20261016_contract_evidence.sql:1`. File and text limits
  are configurable through `DCS_EVIDENCE_MAX_FILE_BYTES` (default 10 MiB) and
  `DCS_EVIDENCE_MAX_TEXT_BYTES` (default 4096 bytes); invalid/nonpositive values
  or values above 100 MiB fall back to defaults
  (`backend/internal/base/conf/evidence.go:8`). The HTTP body is bounded before
  decoding; decoded file/text validation also applies.
- PostgreSQL, migrations, JWT validation, persisted encryption keys,
  cryptography and HTTP error mappings were exercised. Catalogue/PDF-core
  interactions used local HTTP test servers; blob transport, external key
  provider and audit executor used controlled adapters.
- No BDD, live federated catalogue, Kubo, HSM or external QA deployment was
  exercised. Browser tests prove UI behavior, not a fully connected stack.
  Existing BDD fixture payloads were adapted to the required party field but
  those scenarios remain unexecuted in this work.

No new architecture decision was needed. ADR-21 only received a corrected
source reference; its decision and implementation status remain unchanged.
