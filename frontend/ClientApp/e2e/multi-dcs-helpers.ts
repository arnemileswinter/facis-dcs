import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import { homedir, tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import type { Browser, BrowserContext, Page } from '@playwright/test'
import {
  E2E_API_BASE,
  E2E_API_BASE_B,
  E2E_DSS_URL,
  E2E_FRONTEND_B_ORIGIN,
  E2E_STATUSLIST_URL,
} from '../playwright.config'
import { applySession, type DcsRole, expect, mintSession } from './dcs-test'

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(here, '../../..')
const python = process.env.E2E_BDD_PYTHON || path.join(homedir(), '.dcs-bdd-venv', 'bin', 'python3')

/**
 * Where the vertical persists every hop's PDF and its embedded JSON-LD for human
 * supervision — sibling of the e2e dir, outside Playwright's test-results output
 * (which it wipes at run start), and uploaded whole by CI as vertical-pdf-artifacts.
 */
const artifactDir = path.resolve(here, '../vertical-artifacts')

/**
 * A single DCS instance the two-instance vertical drives from its own UI: its
 * browser context/page bound to that DCS's frontend origin, its API base, and a
 * per-navigation session minter. Hydra rotates refresh tokens single-use, so
 * each top-level navigation re-mints a fresh role session for that instance.
 */
export interface Instance {
  readonly page: Page
  readonly context: BrowserContext
  readonly origin: string
  readonly apiBase: string
  gotoAs(role: DcsRole, url: string): Promise<void>
}

function makeInstance(page: Page, context: BrowserContext, origin: string, apiBase: string): Instance {
  return {
    page,
    context,
    origin,
    apiBase,
    async gotoAs(role, url) {
      await applySession(context, page, origin, mintSession(role, apiBase))
      await page.goto(url)
    },
  }
}

/** Wraps the test's own fixture page/context as instance A (the originator). */
export function instanceA(page: Page, context: BrowserContext, origin: string): Instance {
  return makeInstance(page, context, origin, E2E_API_BASE)
}

/** Opens a second browser context/page for instance B (the counterparty), on
 *  B's own frontend origin and API base — the DCS-to-DCS peer. */
export async function openInstanceB(browser: Browser): Promise<Instance> {
  const context = await browser.newContext({ baseURL: E2E_FRONTEND_B_ORIGIN })
  const page = await context.newPage()
  return makeInstance(page, context, E2E_FRONTEND_B_ORIGIN, E2E_API_BASE_B)
}

/**
 * Signs an APPROVED contract on a given instance through that instance's Secure
 * Contract Viewer, exactly as a real signer would (ADR-12): open from the
 * signing list, verify, run the wallet PID+PoA ceremony (the wallet leg arrives
 * over the wallet's own webhook channel against this instance's API base),
 * download the to-be-signed PDF, sign it externally with the test wallet's key
 * via the DSS SCA, upload it, and confirm SIGNED. The signature field is the
 * signing party's own DCS DID slot; the wallet discovers it from the PDF.
 */
export async function signOnInstance(inst: Instance, contractDid: string, signatory: string): Promise<void> {
  await inst.gotoAs('Contract Signer', '/ui/signing')
  const row = inst.page.getByRole('row').filter({ hasText: contractDid })
  await expect(row).toBeVisible()
  await row.getByRole('link', { name: /Open/ }).click()
  await expect(inst.page).toHaveURL(/\/signing\/.+/)

  await inst.page.getByRole('button', { name: 'Verify', exact: true }).click()
  await expect(inst.page.getByText('Verified', { exact: true })).toBeVisible()

  const ceremonyStarted = inst.page.waitForResponse(
    (r) => r.url().includes('/signature/request') && r.request().method() === 'POST' && r.ok(),
    { timeout: 30_000 },
  )
  const preparedDownload = inst.page.waitForEvent('download', { timeout: 30_000 })
  await inst.page.getByRole('button', { name: /download document to sign/ }).click()
  const ceremony = (await (await ceremonyStarted).json()) as { ceremony_id: string }
  expect(ceremony.ceremony_id).toBeTruthy()

  execFileSync(python, [path.join(here, 'complete_signing_webhook.py'), ceremony.ceremony_id], {
    cwd: repoRoot,
    env: { ...process.env, STATUSLIST_SERVICE_URL: E2E_STATUSLIST_URL, BDD_DCS_BASE_URL: inst.apiBase },
    stdio: 'pipe',
  })

  const preparedPath = (await (await preparedDownload).path())!
  const signedPath = path.join(tmpdir(), `signed-${ceremony.ceremony_id}.pdf`)
  execFileSync(python, [path.join(here, 'sign_prepared_pdf.py'), preparedPath, signedPath], {
    cwd: repoRoot,
    env: { ...process.env, DSS_URL: E2E_DSS_URL, E2E_SIGNATORY: signatory },
    stdio: 'pipe',
  })

  await inst.page.locator('input[type="file"]').setInputFiles(signedPath)
  await expect(inst.page.getByText('SIGNED', { exact: true })).toBeVisible({ timeout: 60_000 })
}

/**
 * Establishes a role session on the instance and returns the Authorization
 * header its raw page.request calls need. applySession injects the token into
 * localStorage, but only the app's axios interceptor turns that into a bearer —
 * a raw page.request forwards cookies but omits the header, so JWT-scoped
 * endpoints 401. The navigation also refreshes the role's single-use token.
 */
export async function apiAuthHeaders(
  inst: Instance,
  role: DcsRole,
  landing: string,
): Promise<{ Authorization: string }> {
  await inst.gotoAs(role, landing)
  const token = await inst.page.evaluate(() => window.localStorage.getItem('access_token'))
  expect(token, `no access token for ${role} on ${inst.origin}`).toBeTruthy()
  return { Authorization: `Bearer ${token}` }
}

/**
 * Reads the contract's current optimistic-lock token (updated_at) from the
 * instance's own authenticated retrieve-by-id — the value state-transition POSTs
 * (offer/deploy/…) must echo. Fails loudly with the response shape if absent.
 */
export async function contractUpdatedAt(
  inst: Instance,
  contractDid: string,
  auth: { Authorization: string },
): Promise<string> {
  const resp = await inst.page.request.get(`${inst.apiBase}/contract/retrieve/${encodeURIComponent(contractDid)}`, {
    headers: auth,
  })
  expect(
    resp.ok(),
    `retrieve ${contractDid} on ${inst.origin}: HTTP ${resp.status()} ${await resp.text()}`,
  ).toBeTruthy()
  const body = (await resp.json()) as { updated_at?: string }
  expect(body.updated_at, `retrieve ${contractDid} on ${inst.origin} returned no updated_at`).toBeTruthy()
  return body.updated_at!
}

/**
 * Independently verifies the contract's exported PDF is a real, conformant
 * artifact — PDF/A-3a (veraPDF) + a valid C2PA manifest (c2patool/c2pa-rs) —
 * exporting it through the instance's own Contract Viewer and shelling out to
 * e2e/verify_artifact.py (the same external validators pdf-core runs). The
 * optional lifecycle is the SRS C2PA banner (draft during negotiation, active
 * once signed) — NOT the extrinsic negotiation phase.
 */
export async function verifyArtifact(
  inst: Instance,
  contractDid: string,
  opts: { lifecycle?: string; save?: string } = {},
): Promise<void> {
  const pdfPath = await exportContractPdf(inst, contractDid)
  const args = [path.join(here, 'verify_artifact.py'), pdfPath]
  if (opts.lifecycle) args.push('--lifecycle', opts.lifecycle)
  execFileSync(python, args, {
    cwd: repoRoot,
    stdio: 'pipe',
    timeout: 60_000,
    env: { ...process.env, PYTHONWARNINGS: 'ignore' },
  })
  if (opts.save) persistArtifact(pdfPath, opts.save)
}

/** Exports the contract's PDF through the instance's own Contract Viewer (the
 *  Export PDF download) and returns the local path to the downloaded bytes. */
async function exportContractPdf(inst: Instance, contractDid: string): Promise<string> {
  await inst.gotoAs('Contract Manager', `/ui/contracts/view/${contractDid}`)
  // IPFS-backed export: the signed hops fetch the frozen PDF from IPFS, which on
  // a loaded stack legitimately approaches ~45s — kept above the 30s download cap
  // so a real export isn't cut off, while still failing a genuine hang fast.
  const download = inst.page.waitForEvent('download', { timeout: 45_000 })
  await inst.page.getByRole('button', { name: 'Export PDF' }).click()
  // Save under a .pdf name: veraPDF (run by verify_artifact.py) refuses to
  // process a file without a .pdf extension, and Playwright's download.path()
  // is an extensionless temp file.
  const out = path.join(tmpdir(), `export-${contractDid}-${Date.now()}.pdf`)
  await (await download).saveAs(out)
  return out
}

/**
 * Persists a hop's exported PDF and its embedded JSON-LD payload into the
 * vertical-artifacts dir (uploaded by CI for human supervision): `{label}.pdf`
 * beside `{label}.jsonld`. The JSON-LD is extracted from the very bytes we
 * saved, so the machine-readable payload always matches the human-readable PDF.
 */
function persistArtifact(pdfPath: string, label: string): void {
  fs.mkdirSync(artifactDir, { recursive: true })
  const outPdf = path.join(artifactDir, `${label}.pdf`)
  fs.copyFileSync(pdfPath, outPdf)
  execFileSync(
    python,
    [
      path.join(here, 'verify_artifact.py'),
      outPdf,
      '--extract-only',
      '--dump-jsonld',
      path.join(artifactDir, `${label}.jsonld`),
    ],
    { cwd: repoRoot, stdio: 'pipe', timeout: 60_000, env: { ...process.env, PYTHONWARNINGS: 'ignore' } },
  )
}

/** Saves a hop's PDF + embedded JSON-LD for the party's copy without running the
 *  heavyweight veraPDF/c2patool validators (those run at the verify hops). */
export async function saveArtifact(inst: Instance, contractDid: string, label: string): Promise<void> {
  persistArtifact(await exportContractPdf(inst, contractDid), label)
}

/** The C2PA manifest-history URL for a contract on an instance. The C2PA
 *  service is mounted on the API muxer (backend cmd/dcs/http.go), so it lives
 *  under DCS_API_PATH (/digital-contracting-service/api) like every other app
 *  endpoint — not at the service root (that is did.json, on the raw mux). */
function manifestHistoryUrl(inst: Instance, contractDid: string): string {
  return `${inst.apiBase}/c2pa/manifest/${encodeURIComponent(contractDid)}?history=true`
}

/**
 * Asserts the contract's C2PA manifest ingredient chain on this instance has
 * grown past prevCount (each PDF exchange adds one ingredient, so the
 * counterparty's provenance is chained rather than reset) and returns the new
 * length. Call on BOTH instances across a negotiation exchange.
 */
export async function assertManifestChainGrew(inst: Instance, contractDid: string, prevCount: number): Promise<number> {
  // The new PDF and its grown C2PA chain are produced by the event-driven
  // background regenerator AFTER the negotiate/sign call returns, and the peer's
  // copy replicates asynchronously over the PDF exchange. Until the regen lands
  // the export route reports "being regenerated" (backend exportcontract.go), so
  // poll the manifest history until the chain grows past prevCount, tolerating
  // the transient not-ready response.
  const deadline = Date.now() + 45_000
  let lastStatus = 0
  let lastLen = -1
  while (Date.now() < deadline) {
    const resp = await inst.page.request.get(manifestHistoryUrl(inst, contractDid))
    lastStatus = resp.status()
    if (resp.ok()) {
      const chain = (await resp.json()) as unknown[]
      if (Array.isArray(chain)) {
        lastLen = chain.length
        if (chain.length > prevCount) return chain.length
      }
    }
    await inst.page.waitForTimeout(1500)
  }
  expect(
    lastLen,
    `C2PA manifest chain on ${inst.origin} should grow past ${prevCount} within 45s (last HTTP ${lastStatus}, last length ${lastLen})`,
  ).toBeGreaterThan(prevCount)
  return lastLen
}

/** Current length of the contract's C2PA manifest chain on an instance (0 if
 *  none yet), for seeding assertManifestChainGrew. */
export async function manifestChainLength(inst: Instance, contractDid: string): Promise<number> {
  const resp = await inst.page.request.get(manifestHistoryUrl(inst, contractDid))
  if (!resp.ok()) return 0
  const chain = (await resp.json()) as unknown[]
  return Array.isArray(chain) ? chain.length : 0
}

/**
 * Polls the instance's own /contract/retrieve until the contract's state
 * matches expected (the peer-facing copy replicates asynchronously over the
 * PDF exchange, so allow the same window the peer-trust steps use).
 */
export async function assertReceivedInState(inst: Instance, contractDid: string, expected: string): Promise<void> {
  // Establish a Contract Manager session on this instance so the raw retrieve
  // carries the bearer the JWT-scoped endpoint requires (page.request forwards
  // cookies but not the Authorization header the app's axios interceptor adds).
  const auth = await apiAuthHeaders(inst, 'Contract Manager', `/ui/contracts/view/${contractDid}`)
  const deadline = Date.now() + 45_000
  let lastState = ''
  let lastStatus = 0
  let lastBody = ''
  while (Date.now() < deadline) {
    const resp = await inst.page.request.get(`${inst.apiBase}/contract/retrieve/${encodeURIComponent(contractDid)}`, {
      headers: auth,
    })
    lastStatus = resp.status()
    lastBody = await resp.text()
    if (resp.ok()) {
      lastState = String((JSON.parse(lastBody) as { state?: string }).state ?? '').toUpperCase()
      if (lastState === expected.toUpperCase()) return
    }
    await inst.page.waitForTimeout(1500)
  }
  // Peer replication failed: surface exactly what this instance sees so the CI
  // log disambiguates a never-created copy (ship/trust rejection at PostPdf) from
  // a slow or errored sync — the two look identical as an empty state otherwise.
  expect(
    lastState,
    `contract ${contractDid} on ${inst.origin} never reached ${expected} within 45s ` +
      `(last retrieve HTTP ${lastStatus}, state "${lastState}", body ${lastBody.slice(0, 400)})`,
  ).toBe(expected.toUpperCase())
}

/** Confirms the shared ConfirmationModal (comment/decision-note dialogs) on an
 *  instance's page. */
async function confirmModalOn(inst: Instance, buttonName: 'Submit' | 'Confirm'): Promise<void> {
  const dialog = inst.page.getByRole('dialog').filter({ hasText: 'Confirmation' })
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: buttonName, exact: true }).click()
}

/** Waits until a template detail view finished loading (Global Name populated). */
async function waitForTemplateLoadedOn(inst: Instance, name: string): Promise<void> {
  await expect(inst.page.getByRole('group').filter({ hasText: 'Global Name' }).getByRole('textbox')).toHaveValue(name)
}

/**
 * Asserts a PDF/A can be exported for a document at the current lifecycle step
 * on an instance, using that instance's active bearer token and API base — the
 * same authenticated GET /pdf/export/{kind}/{did} the Export PDF button issues.
 */
async function assertPdfExportOn(
  inst: Instance,
  kind: 'template' | 'contract',
  did: string,
  step: string,
): Promise<void> {
  const token = await inst.page.evaluate(() => window.localStorage.getItem('access_token'))
  const resp = await inst.page.request.get(`${inst.apiBase}/pdf/export/${kind}/${encodeURIComponent(did)}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(resp.ok(), `export ${kind} PDF at "${step}" on ${inst.origin}: HTTP ${resp.status()}`).toBeTruthy()
  const bytes = await resp.body()
  expect(bytes.subarray(0, 5).toString('latin1'), `PDF/A magic bytes at "${step}"`).toBe('%PDF-')
}

/** A non-trivial SHACL NodeShape TTL for the hub-publish stage: a payment
 *  clause asset type with a constrained monetary amount and currency. */
function paymentShapeTtl(name: string): string {
  return `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <https://example.org/${name}#> .

ex:PaymentClauseShape
  a sh:NodeShape ;
  sh:targetClass ex:PaymentClause ;
  sh:property [
    sh:path ex:amount ;
    sh:datatype xsd:decimal ;
    sh:minInclusive 0 ;
    sh:minCount 1 ;
  ] ;
  sh:property [
    sh:path ex:currency ;
    sh:datatype xsd:string ;
    sh:in ( "EUR" "USD" ) ;
    sh:minCount 1 ;
  ] .
`
}

/**
 * Stage 1 — publishes a brand-new, non-trivial SHACL shapes-graph entry into
 * the instance's Semantic Hub through the dashboard UI (the Gaia-X case: an
 * external shape enters the running instance without a rebuild), then confirms
 * it resolves through the hub's public route. The vertical authors its own
 * vocabulary rather than assuming a seeded fixture.
 */
export async function publishShapeOnInstance(inst: Instance, name: string): Promise<void> {
  await inst.gotoAs('Template Manager', '/ui/semantic-hub')
  await expect(inst.page.getByRole('heading', { name: 'Semantic Hub' })).toBeVisible()
  await inst.page.getByLabel('Entry name').fill(name)
  await inst.page.getByLabel('Entry kind').selectOption('shapes')
  await inst.page.getByLabel('Entry content').fill(paymentShapeTtl(name))
  await inst.page.getByRole('button', { name: 'Publish entry' }).click()
  await expect(inst.page.getByRole('heading', { name })).toBeVisible()
  await expect(inst.page.getByText('active').first()).toBeVisible()

  const resolved = await inst.page.request.get(`${inst.apiBase}/semantic/shapes/${name}`)
  expect(resolved.ok(), `published shape ${name} resolves on ${inst.origin}`).toBeTruthy()
  expect(await resolved.text()).toContain('PaymentClauseShape')
}

/**
 * Stage 2 — builds a Component template with a semantic clause through the real
 * editor: a titled clause carrying human prose beside its machine-readable ODRL
 * meaning, bound to a SHACL-backed hub requirement field (Payment Amount), with
 * a permission bounded by that field, placed into the document outline. Returns
 * the created component's DID.
 */
export async function authorSemanticComponent(inst: Instance, name: string): Promise<string> {
  await inst.gotoAs('Template Creator', '/ui/templates/new')
  await inst.page.getByRole('button', { name: /Component/ }).click()
  await inst.page.getByRole('group').filter({ hasText: 'Global Name' }).getByRole('textbox').fill(name)
  await inst.page
    .getByRole('group')
    .filter({ hasText: 'Base Description' })
    .getByRole('textbox')
    .fill('Payment component authored by the two-instance vertical.')

  await inst.page.getByRole('tab', { name: /Clauses/ }).click()
  const editor = inst.page.getByTestId('split-clause-editor')
  await editor.getByPlaceholder('Clause title').fill('Payment terms')
  await editor.locator('select').first().selectOption({ label: 'Payment Amount' })
  await editor.locator('.clause-editor').first().click()
  await inst.page.keyboard.type('The provider invoices the agreed payment amount.')

  const ruleSelect = (label: string) =>
    editor.locator('label.form-control').filter({ hasText: label }).locator('select')
  await ruleSelect('Rule').selectOption({ label: 'Permission — the assignee MAY' })
  await ruleSelect('Action').selectOption({ label: 'use' })
  await editor.getByRole('button', { name: '+ constraint' }).click()
  const constraint = editor.locator('.flex.flex-wrap.items-center.gap-1').last()
  await constraint.locator('select').nth(0).selectOption({ label: 'Payment Amount' })
  await constraint.locator('select').nth(1).selectOption({ label: 'must be at most' })
  await constraint.locator('input[placeholder="value"]').fill('500')

  await editor.getByRole('button', { name: 'Add clause', exact: true }).click()
  await expect(editor.getByPlaceholder('Clause title')).toHaveValue('')

  const modal = inst.page.getByRole('dialog')
  await inst.page.getByRole('button', { name: 'Place in document' }).first().click()
  await expect(modal.getByText('Selected clause')).toBeVisible()
  await modal.getByRole('button', { name: /Payment terms/ }).click()
  await expect(inst.page.getByRole('dialog')).toBeHidden()

  const created = inst.page.waitForResponse(
    (r) => r.url().includes('/template/create') && r.request().method() === 'POST' && r.ok(),
  )
  await inst.page.getByRole('button', { name: 'Create', exact: true }).click()
  const componentDid = ((await (await created).json()) as { did: string }).did
  expect(componentDid).toBeTruthy()
  await assertPdfExportOn(inst, 'template', componentDid, 'component DRAFT')
  return componentDid
}

/** DRAFT → SUBMITTED → REVIEWED → APPROVED for one template on an instance,
 *  via the real UI (submit, verify + reviewer recommendation, approval). */
export async function submitReviewApproveTemplateOn(inst: Instance, did: string, name: string): Promise<void> {
  await inst.gotoAs('Template Creator', `/ui/templates/view/${did}`)
  const submitted = inst.page.waitForResponse(
    (r) => r.url().includes('/template/submit') && r.request().method() === 'POST' && r.ok(),
  )
  await inst.page.getByRole('button', { name: 'Submit', exact: true }).click()
  await submitted
  await assertPdfExportOn(inst, 'template', did, `${name} SUBMITTED`)

  await inst.gotoAs('Template Reviewer', `/ui/templates/review/${did}`)
  await waitForTemplateLoadedOn(inst, name)
  const verified = inst.page.waitForResponse(
    (r) => r.url().includes('/template/verify') && r.request().method() === 'POST' && r.ok(),
  )
  await inst.page.getByRole('button', { name: 'Verify', exact: true }).click()
  await verified
  await inst.page.getByRole('dialog').getByRole('button', { name: 'Close', exact: true }).click()
  const forwarded = inst.page.waitForResponse(
    (r) => r.url().includes('/template/submit') && r.request().method() === 'POST' && r.ok(),
  )
  await inst.page.getByRole('button', { name: 'Approve', exact: true }).click()
  await confirmModalOn(inst, 'Submit')
  await forwarded

  await inst.gotoAs('Template Approver', `/ui/templates/approve/${did}`)
  await waitForTemplateLoadedOn(inst, name)
  const approved = inst.page.waitForResponse(
    (r) => r.url().includes('/template/approve') && r.request().method() === 'POST' && r.ok(),
  )
  await inst.page.getByRole('button', { name: 'Approve', exact: true }).click()
  await confirmModalOn(inst, 'Submit')
  await approved
  await assertPdfExportOn(inst, 'template', did, `${name} APPROVED`)
}

/**
 * Stage 3 — composes a Contract Template on an instance that pins the approved
 * component as a sub-template snapshot and references it in a custom document
 * wrapping (Builder outline). Returns the created contract template's DID.
 */
export async function authorContractTemplate(inst: Instance, name: string, componentName: string): Promise<string> {
  await inst.gotoAs('Template Creator', '/ui/templates/new')
  await inst.page.getByRole('button', { name: /parent for other contracts/ }).click()
  await inst.page.getByRole('group').filter({ hasText: 'Global Name' }).getByRole('textbox').fill(name)
  await inst.page
    .getByRole('group')
    .filter({ hasText: 'Base Description' })
    .getByRole('textbox')
    .fill('Contract template composed by the two-instance vertical.')

  await inst.page.getByText('Component Templates', { exact: true }).click()
  await inst.page.getByPlaceholder('Search templates…').fill(componentName)
  await inst.page.getByRole('button', { name: componentName }).click()
  await expect(inst.page.getByText('No component templates selected yet.')).toBeHidden()

  await inst.page.getByRole('tab', { name: /Builder/ }).click()
  await inst.page
    .getByRole('button', { name: /add.*block/i })
    .first()
    .click()
  const modal = inst.page.getByRole('dialog')
  await expect(modal.getByText('Approved sub-templates:')).toBeVisible()
  await modal.getByText(componentName).first().click()
  await expect(inst.page.getByRole('dialog')).toBeHidden()

  const created = inst.page.waitForResponse(
    (r) => r.url().includes('/template/create') && r.request().method() === 'POST' && r.ok(),
  )
  await inst.page.getByRole('button', { name: 'Create', exact: true }).click()
  const contractTemplateDid = ((await (await created).json()) as { did: string }).did
  expect(contractTemplateDid).toBeTruthy()
  return contractTemplateDid
}

/** Stage 3 tail — registers an approved contract template (publishes it to the
 *  Federated Catalogue) so contracts can be derived from it. */
export async function registerTemplateOn(inst: Instance, did: string, name: string): Promise<void> {
  await inst.gotoAs('Template Manager', `/ui/templates/view/${did}`)
  await waitForTemplateLoadedOn(inst, name)
  const registered = inst.page.waitForResponse(
    (r) => r.url().includes('/template/register') && r.request().method() === 'POST' && r.ok(),
  )
  await inst.page.getByRole('button', { name: 'Register', exact: true }).click()
  await registered
}

/** The counterparty's own did:web, resolved from its origin-root DID document
 *  (/.well-known/did.json) — the value A types into the R6 counterparty input. */
export async function resolveDidWeb(inst: Instance): Promise<string> {
  const resp = await inst.page.request.get(`${inst.origin}/.well-known/did.json`)
  expect(resp.ok(), `DID document for ${inst.origin}: HTTP ${resp.status()}`).toBeTruthy()
  const id = String(((await resp.json()) as { id?: string }).id ?? '')
  expect(id).toBeTruthy()
  return id
}

/**
 * Stage 4 — derives a contract from a registered template through the real UI,
 * naming the counterparty via the R6 ParticipantSelectionDialog (a single
 * counterparty did:web input). Returns the created contract's DID.
 */
export async function createContractViaUi(inst: Instance, templateName: string, counterparty: string): Promise<string> {
  await inst.gotoAs('Contract Creator', '/ui/contracts/new')
  const picker = inst.page.locator('select').first()
  const option = picker.locator('option', { hasText: templateName })
  await expect(option).toHaveCount(1)
  await picker.selectOption({ label: (await option.textContent())!.trim() })

  await inst.page.getByRole('button', { name: 'Create', exact: true }).click()
  const dialog = inst.page.getByRole('dialog').filter({ hasText: 'Contract Counterparty' })
  await expect(dialog).toBeVisible()
  await dialog.getByPlaceholder('did:web:...').fill(counterparty)
  const created = inst.page.waitForResponse(
    (r) => r.url().includes('/contract/create') && r.request().method() === 'POST',
  )
  await dialog.getByRole('button', { name: 'Apply', exact: true }).click()
  const resp = await created
  expect(resp.ok(), `contract create ${resp.status()}: ${await resp.text()}`).toBeTruthy()
  const contractDid = String(((await resp.json()) as { did?: string }).did ?? '')
  expect(contractDid).toBeTruthy()
  return contractDid
}

/**
 * Makes a non-trivial counter-offer on the instance's Negotiate view: edits a
 * requirement value in the contract editor (producing a change request) and
 * submits it, which regenerates the PDF and re-ships it to the counterparty.
 * NOTE: the editor field-drilling here is the coordination seam with the
 * backend R5 (counter-offer round-trip) — refine the selector during
 * integration once the negotiate → settle flow is wired end to end.
 */
export async function counterOffer(inst: Instance, contractDid: string, opts: { value: string }): Promise<void> {
  // The counterparty makes a counter-offer by proposing a redline through the
  // real Negotiate UI. Its received copy is OFFERED and it holds the Negotiator
  // role (not Creator), so it cannot /contract/submit (Creator-only) — instead
  // its "Change Proposal" (/contract/negotiate) opens negotiation directly
  // (Offered --EventNegotiate--> Negotiation; SRS DCS-IR-CWE-03/DCS-FR-CWE-18).
  await inst.gotoAs('Contract Manager', `/ui/contracts/negotiate/${contractDid}`)
  const firstValue = inst.page.locator('input[type="text"], input[type="number"]').first()
  await expect(firstValue).toBeVisible({ timeout: 30_000 })
  await firstValue.fill(opts.value)
  const proposed = inst.page.waitForResponse(
    (r) => r.url().includes('/contract/negotiate') && r.request().method() === 'POST' && r.ok(),
    { timeout: 30_000 },
  )
  await inst.page.getByRole('button', { name: 'Change Proposal' }).click()
  await proposed
}


/**
 * Stage 5 — A transmits the DRAFT contract to its counterparty through the real
 * UI: the Contract Creator's "Offer to counterparty" action on the contract view
 * (DRAFT -> OFFERED). command/offer.go gates this on the ContractCreator role and
 * EventOffer, which the state machine allows only from DRAFT (SRS DCS-IR-CWE-01;
 * §1.2 offer→acceptance). The transition ships the PDF to the trusted peer.
 */
export async function offerToCounterparty(inst: Instance, contractDid: string): Promise<void> {
  await inst.gotoAs('Contract Creator', `/ui/contracts/view/${contractDid}`)
  const offered = inst.page.waitForResponse(
    (r) => r.url().includes('/contract/offer') && r.request().method() === 'POST' && r.ok(),
    { timeout: 30_000 },
  )
  await inst.page.getByRole('button', { name: 'Offer to counterparty' }).click()
  await offered
}

/**
 * Stage 7 pre-settle gate — asserts a contract is not yet signable on an instance.
 * ADR-2 allows EventSign only from APPROVED, so before the contract is approved
 * the Secure Contract Viewer's signing list must not offer it. This is the real
 * UI gate a signer hits (there is no /signature/apply route to POST against).
 */
export async function assertNotYetSignable(inst: Instance, contractDid: string): Promise<void> {
  await inst.gotoAs('Contract Signer', '/ui/signing')
  await expect(inst.page.getByRole('heading', { name: /Signing/ })).toBeVisible()
  await expect(inst.page.getByRole('row').filter({ hasText: contractDid })).toHaveCount(0)
}

/**
 * Stage 7 settle — drives an instance's contract from an open negotiation round
 * to APPROVED through the real UI, the SRS consolidation path (there is no
 * /contract/settle route; ACCEPTED is not a contract state). Accepts the
 * outstanding change request (NegotiationList Show → Accept → /contract/respond),
 * submits the merged round (NEGOTIATION → SUBMITTED), reviews it (SUBMITTED →
 * REVIEWED), and approves it (REVIEWED → APPROVED, EventApprove). Mirrors the
 * proven single-instance submit→review→approve sequence.
 */
export async function settleToApprovedOn(inst: Instance, contractDid: string): Promise<void> {
  // Accept the outstanding change request so no open decision blocks Submit.
  await inst.gotoAs('Contract Creator', `/ui/contracts/negotiate/${contractDid}`)
  const showBtn = inst.page.getByRole('button', { name: 'Show' }).first()
  if (await showBtn.isVisible().catch(() => false)) {
    await showBtn.click()
    const responded = inst.page.waitForResponse(
      (r) => r.url().includes('/contract/respond') && r.request().method() === 'POST' && r.ok(),
      { timeout: 30_000 },
    )
    await inst.page.getByRole('button', { name: 'Accept', exact: true }).click()
    await confirmModalOn(inst, 'Confirm')
    await responded
  }

  // Submit the merged round: NEGOTIATION -> SUBMITTED.
  const submitted = inst.page.waitForResponse(
    (r) => r.url().includes('/contract/submit') && r.request().method() === 'POST' && r.ok(),
    { timeout: 30_000 },
  )
  await inst.page.getByRole('button', { name: 'Submit', exact: true }).click()
  await submitted

  // Review: SUBMITTED -> REVIEWED.
  await inst.gotoAs('Contract Reviewer', `/ui/contracts/review/${contractDid}`)
  const forwarded = inst.page.waitForResponse(
    (r) => r.url().includes('/contract/submit') && r.request().method() === 'POST' && r.ok(),
    { timeout: 30_000 },
  )
  await inst.page.getByRole('button', { name: 'Approve', exact: true }).click()
  await confirmModalOn(inst, 'Submit')
  await forwarded

  // Approve: REVIEWED -> APPROVED.
  await inst.gotoAs('Contract Approver', `/ui/contracts/approve/${contractDid}`)
  const approved = inst.page.waitForResponse(
    (r) => r.url().includes('/contract/approve') && r.request().method() === 'POST' && r.ok(),
    { timeout: 30_000 },
  )
  await inst.page.getByRole('button', { name: 'Approve', exact: true }).click()
  await confirmModalOn(inst, 'Confirm')
  await approved
}

/**
 * Stage 9 — the Contract Manager deploys the fully-signed contract to the target
 * system through the real UI: the "Deploy" action in ContractManagerActions
 * (SIGNED -> ACTIVE, EventDeploy), gated on the Manager role and SIGNED state.
 */
export async function deployContract(inst: Instance, contractDid: string): Promise<void> {
  await inst.gotoAs('Contract Manager', `/ui/contracts/view/${contractDid}`)
  const deployed = inst.page.waitForResponse(
    (r) => r.url().includes('/contract/deploy') && r.request().method() === 'POST' && r.ok(),
    { timeout: 30_000 },
  )
  await inst.page.getByRole('button', { name: 'Deploy', exact: true }).click()
  await deployed
}
