import { execFileSync } from 'node:child_process'
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
  )
  const preparedDownload = inst.page.waitForEvent('download')
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
  await expect(inst.page.getByText('SIGNED', { exact: true })).toBeVisible({ timeout: 120_000 })
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
  opts: { lifecycle?: string } = {},
): Promise<void> {
  await inst.gotoAs('Contract Manager', `/ui/contracts/view/${contractDid}`)
  const download = inst.page.waitForEvent('download', { timeout: 90_000 })
  await inst.page.getByRole('button', { name: 'Export PDF' }).click()
  const pdfPath = (await (await download).path())!
  const args = [path.join(here, 'verify_artifact.py'), pdfPath]
  if (opts.lifecycle) args.push('--lifecycle', opts.lifecycle)
  execFileSync(python, args, { cwd: repoRoot, stdio: 'pipe' })
}

/** The public C2PA manifest-history URL for a contract on an instance (the
 *  `?history=true` parsed chain enumeration is a sibling of the API prefix). */
function manifestHistoryUrl(inst: Instance, contractDid: string): string {
  const root = inst.apiBase.replace(/\/api\/?$/, '')
  return `${root}/c2pa/manifest/${encodeURIComponent(contractDid)}?history=true`
}

/**
 * Asserts the contract's C2PA manifest ingredient chain on this instance has
 * grown past prevCount (each PDF exchange adds one ingredient, so the
 * counterparty's provenance is chained rather than reset) and returns the new
 * length. Call on BOTH instances across a negotiation exchange.
 */
export async function assertManifestChainGrew(inst: Instance, contractDid: string, prevCount: number): Promise<number> {
  const resp = await inst.page.request.get(manifestHistoryUrl(inst, contractDid))
  expect(resp.ok(), `C2PA manifest history HTTP ${resp.status()} for ${contractDid} on ${inst.origin}`).toBeTruthy()
  const chain = (await resp.json()) as unknown[]
  expect(Array.isArray(chain), `manifest history is a chain list on ${inst.origin}`).toBeTruthy()
  expect(chain.length, `C2PA manifest chain on ${inst.origin} should grow past ${prevCount}`).toBeGreaterThan(prevCount)
  return chain.length
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
  const deadline = Date.now() + 45_000
  let last = ''
  while (Date.now() < deadline) {
    const resp = await inst.page.request.get(`${inst.apiBase}/contract/retrieve/${encodeURIComponent(contractDid)}`)
    if (resp.ok()) {
      last = String(((await resp.json()) as { state?: string }).state ?? '').toUpperCase()
      if (last === expected.toUpperCase()) return
    }
    await inst.page.waitForTimeout(1500)
  }
  expect(last, `contract ${contractDid} on ${inst.origin} reached ${expected}`).toBe(expected.toUpperCase())
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
  await inst.gotoAs('Contract Manager', `/ui/contracts/negotiate/${contractDid}`)
  const firstValue = inst.page.locator('input[type="text"], input[type="number"]').first()
  await expect(firstValue).toBeVisible({ timeout: 30_000 })
  await firstValue.fill(opts.value)
  const proposed = inst.page.waitForResponse(
    (r) => r.url().includes('/contract/negotiate') && r.request().method() === 'POST' && r.ok(),
  )
  await inst.page.getByRole('button', { name: /Negotiate|Propose/ }).click()
  await proposed
}
