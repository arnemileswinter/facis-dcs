# Connected QA acceptance — 2026-10-16

Status: **P04, P05, P06, P09, F02, F03 and F10 passed connected local
non-BDD acceptance**. Independent verification accepted AC1–AC9; subsequent
revision reported no findings and made no changes. This report records AC10.
BDD execution and external QA/IONOS checks were explicitly excluded. A separate
pre-existing Helm lifecycle check remains red; this is not a general merge or
target-deployment approval.

The [earlier P04/P06/F02 report](qa-fixes-verification-2026-10-16.md) and
[P05/P09/F03/F10 report](qa-p05-p09-f03-f10-verification-2026-10-16.md) retain
their original test counts and fixture boundaries. User steps are in the
[existing guide](user-manual/archive-search-template-classification-evidence.md).

## Accepted scope

| AC / problem | Evidence and result |
|---|---|
| AC1 | Anonymous request fixtures now supply valid required evidence/template fields and explicitly clear bearer headers. Statically checked only: `features/01_authentication_authorization/auth_and_access_control.feature:19`. |
| AC2 / P04 | Real archive summary persists across reload and a tags-only update. Browser: `frontend/ClientApp/e2e/qa-connected-fixes.spec.ts:52`; API: `tests/integration/qa_fixes_connected_smoke.py:231`. Prepared BDD: `features/07_contract_storage_security/archive_management.feature:157`. |
| AC3 / P05 | Observer searches and opens the exact read-only archived version. API snapshot remains unchanged after the live contract is terminated; unknown version returns 404. Browser: `frontend/ClientApp/e2e/qa-connected-fixes.spec.ts:87`; API: `tests/integration/qa_fixes_connected_smoke.py:219`. Prepared BDD: `features/07_contract_storage_security/qa_fixes_closure.feature:3`. |
| AC4 / P09 | Explicit party type survives create/edit/history/copy, real catalogue publication and combined local/catalogue filters. Both SINGLE_PARTY and MULTI_PARTY were imported into a separate real DCS instance with unchanged classification. API: `tests/integration/qa_fixes_connected_smoke.py:77`; import logs below. The four template/party combinations are prepared, not BDD-executed: `features/07_contract_storage_security/qa_fixes_closure.feature:13`. |
| AC5 / F03 | Real organization-scoped contract lists hide foreign contracts; retrieve/history/signature/PDF/bundle reads return 403. Additive Auditor/Observer access succeeds. API: `tests/integration/qa_fixes_connected_smoke.py:117`. Dashboard/task and indirect-read BDD scenarios remain unexecuted: `features/07_contract_storage_security/qa_fixes_closure.feature:33`. |
| AC6 / F10 | Manager stores binary evidence with description; reload/download retains exact bytes and SHA-256. Deleting the test contract makes evidence unreadable (404). API: `tests/integration/qa_fixes_connected_smoke.py:146`, `:249`; browser: `frontend/ClientApp/e2e/qa-connected-fixes.spec.ts:16`. Prepared BDD: `features/07_contract_storage_security/qa_fixes_closure.feature:60`. |
| AC7 / P06 | Canonical Kubo configuration renders secret > explicit URL > enabled subchart endpoint > empty, with required secret key and no legacy alias. `deployment/helm/templates/deployment.yaml:308`, `deployment/helm/templates/_helpers.tpl:248`. IPFS rendering matrix, PAC/ORCE shell checks and default/dev/acceptance/prod Helm lint passed. |
| AC8 | All 69 SQL migration files matched the fresh first-start log and actual `schema_migrations` table; the actual backend started and `/readyz` returned 200. |
| AC9 / P06 | Real audit executor and JSON/CSV/PDF exports passed MIME/timeline checks. Saved events/findings stay frozen after later lifecycle changes; key shredding makes the saved report unavailable. API: `tests/integration/qa_fixes_connected_smoke.py:162`, `:244`; browser: `frontend/ClientApp/e2e/qa-connected-fixes.spec.ts:124`. |
| AC9 / F02 | Six authorization denials are recorded as DENIED, while the same principal remains able to make an authorized request. Auditor reads the recorded attempts with justification. `tests/integration/qa_fixes_connected_smoke.py:132`. |
| AC10 | This report, corrected P06 configuration note, existing guide and requirement-row links record the evidence and outstanding boundaries. |

The connected flow also completed the project wallet ceremony, PDF preparation,
actual DSS signing and automatic archive entry
(`tests/integration/qa_fixes_connected_smoke.py:188`). The independent browser
run passed **1 Chromium test in 51.6 seconds**; the API acceptance ended with
`P04 P05 P06 P09 F02 F03 F10 connected acceptance PASS`.

## Runtime and migration provenance

The native backend was built from
`b1a0d6a2921c946163141efc4067683d0b6b3519`, with `vcs.modified=false`.
Binary SHA-256:
`7e3a0b6c6add9c156b8632ff8d8e7d4244f4231be5cc1a1d9862997a875df7d4`.
Subsequent closure changes affect configuration, tests and documentation, not
backend implementation. The local setup used real PostgreSQL, Kubo, SoftHSM
PKCS#11, NATS, Hydra, development credential issuer, ORCE, federated catalogue,
Keycloak, Fuseki, pdf-core and DSS. No HTTP mocks replaced these services.
Backend, frontend and PDF processes ran natively with containerized dependencies;
no Kubernetes target deployment was performed. The project test wallet/development
issuer is not a production EUDI wallet, and SoftHSM is not a physical HSM.

Use a **fresh disposable database** for this acceptance. The pre-release
migrations add required columns without defaults or backfill: F02 needs empty
`access_attempts` (`backend/migrations/sql/20261015_access_attempt_outcome.sql:1`),
P06 empty `pac_audit_runs`
(`backend/migrations/sql/20261015_pac_audit_snapshot.sql:2`), and P09 empty
`contract_templates` and `contract_templates_history`
(`backend/migrations/sql/20261016_template_party_type.sql:1`). Existing databases
were not reset or migrated. No compatibility/backfill mechanism was introduced.

## Reproduce and inspect

The isolated acceptance environment has been stopped and its disposable data
removed; the following URLs are examples from that run. Recreate the local
dependencies and fresh database, configure/start the native backend and frontend,
and provide the existing wallet/support Python dependencies before using them.
The scripts create fixtures; the API test destroys only its own contract's
archive/key for the erasure check (`tests/integration/qa_fixes_connected_smoke.py:249`).

From the repository root:

```sh
QA_DCS_API_BASE=http://localhost:58991/api \
ISSUER_BASE_URL=http://localhost:30181 \
DSS_URL=http://localhost:58099 \
/tmp/dcs-qa-closure-runtime/python/bin/python tests/integration/qa_fixes_connected_smoke.py
```

From `frontend/ClientApp/`, with fresh test wallet signing material:

```sh
E2E_DCS_API_BASE=http://localhost:58991/api \
E2E_ISSUER_BASE_URL=http://localhost:30181 \
E2E_DSS_URL=http://localhost:58099 \
E2E_BDD_PYTHON=/tmp/dcs-qa-closure-runtime/python/bin/python \
BDD_TEST_WALLET_KEYS_DIR=/tmp/dcs-qa-closure-runtime/wallet-keys \
npx --no-install playwright test e2e/qa-connected-fixes.spec.ts --workers=1 --reporter=list
```

Despite the existing environment-variable names, this runs Playwright and wallet
helpers, not a BDD runner. No Behave scenarios or dry-run were executed. Only
feature parsing, Python syntax and static step-binding checks were performed for
the prepared BDDs (three feature files, six Python files, 52 tagged references).

Frontend lint, full formatting check and build/typecheck passed; the final spec
formatting check also passed. Unchanged backend/Go and Vitest suites were not
repeated after their previous green independent verification. IPFS matrix:
`bash deployment/helm/tests/ipfs_endpoint_test.sh`. Existing PAC/ORCE checks:
`bash deployment/helm/tests/pac_audit_timeout_test.sh` and
`bash deployment/helm/tests/orce_checkpoint_timeout_test.sh`.

Observed transient evidence under `/tmp/dcs-qa-closure-runtime/`:
`independent-api.log`, `independent-playwright.log`,
`independent-import.log`, `independent-import.json`,
`import-instance/result.json`, `backend.log` and `import-instance/backend.log`.
The two import records cover SINGLE_PARTY (independent rerun) and MULTI_PARTY.
These local logs are not durable versioned artifacts. Only the owned acceptance
processes, containers, disposable volumes and networks were removed; existing
Rancher/Kind environments were left untouched.

## Remaining boundary

`bash deployment/helm/tests/federated_catalogue_lifecycle_test.sh` still fails
with `realm provisioning retains polling, retry, or artificial sleep`, both
on this diff and unchanged HEAD. Its assertion
(`deployment/helm/tests/federated_catalogue_lifecycle_test.sh:288`) conflicts
with the existing bounded admin-token retry loop
(`deployment/helm/templates/fc-realm-provision-job.yaml:132`) and the accepted
no-retry posture (`docs/adr-5-xfsc-component-posture.md:51`). This separate,
pre-existing inconsistency is not resolved here and prevents treating the whole
deployment check suite as green.

The historical QA `/pac/report` 500 cause remains unproven. The current local
exports work; no external QA deployment was inspected. Production wallet/HSM,
BDD acceptance, target deployment and full Unicode PDF-font coverage remain
outside this result. No new ADR was needed: the closure applies existing
storage/configuration patterns and adds verification evidence, without a new
architecture decision. The ADR-5 contradiction remains explicitly open.
