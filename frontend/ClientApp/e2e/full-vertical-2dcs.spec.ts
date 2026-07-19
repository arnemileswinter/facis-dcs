import { expect, test } from './dcs-test'
import {
  assertManifestChainGrew,
  assertReceivedInState,
  authorContractTemplate,
  authorSemanticComponent,
  counterOffer,
  createContractViaUi,
  instanceA,
  manifestChainLength,
  openInstanceB,
  publishShapeOnInstance,
  registerTemplateOn,
  resolveDidWeb,
  signOnInstance,
  submitReviewApproveTemplateOn,
  verifyArtifact,
} from './multi-dcs-helpers'
import { E2E_API_BASE, E2E_FRONTEND_ORIGIN } from '../playwright.config'

/**
 * The normative two-instance negotiation vertical: instance A (originator) and
 * instance B (counterparty) drive a contract from its own authored vocabulary
 * and template — SHACL shape, component with a semantic clause, composed
 * contract template published to the Federated Catalogue — all the way through
 * proposal, a non-trivial negotiation ping-pong, mutual signature, deployment
 * and audit, with every exported artifact independently verified (PDF/A-3a via
 * veraPDF + a valid, GROWING C2PA chain via c2patool) on BOTH parties at every
 * hop. No seeded fixtures: A authors the whole contract through the real UI
 * before offering it to B.
 *
 * Marked test.fixme until the backend R5/R5c work is merged: the negotiation
 * counter-offer round-trip (each adjustment ships a new PDF, chain grows), the
 * settle/consolidation gate (signing refused pre-settle; extrinsic phase
 * proposed→agreed→executed on the retrieve API), and cross-instance double
 * signing (B signs on A's signed PDF). Un-fixme and iterate to green once those
 * land. The single-instance full-vertical.spec.ts stays as the local-only
 * lifecycle coverage until this supersedes it.
 */
test.fixme('full two-instance negotiation vertical (A <-> B)', async ({ page, context, browser }) => {
  test.setTimeout(900_000)
  const a = instanceA(page, context, E2E_FRONTEND_ORIGIN)
  const b = await openInstanceB(browser)

  const unique = Date.now()
  const shapeName = `e2e-payment-shape-${unique}`
  const componentName = `2DCS Component ${unique}`
  const contractTemplateName = `2DCS Contract ${unique}`

  let contractDid = ''
  let componentDid = ''
  let contractTemplateDid = ''

  // ---- Stage 1: A publishes a non-trivial SHACL shape through its Semantic Hub.
  await test.step('A publishes a SHACL shape via the Semantic Hub UI', async () => {
    await publishShapeOnInstance(a, shapeName)
  })

  // ---- Stage 2: A authors a component template with a semantic clause (prose +
  // SHACL-backed requirement field + ODRL policy), then review/approve it.
  await test.step('A authors a semantic component and approves it', async () => {
    componentDid = await authorSemanticComponent(a, componentName)
    await submitReviewApproveTemplateOn(a, componentDid, componentName)
  })

  // ---- Stage 3: A composes a contract template from the approved component
  // with custom wrapping, approves it, and registers it to the Federated Catalogue.
  await test.step('A composes a contract template and publishes it to the FC', async () => {
    contractTemplateDid = await authorContractTemplate(a, contractTemplateName, componentName)
    await submitReviewApproveTemplateOn(a, contractTemplateDid, contractTemplateName)
    await registerTemplateOn(a, contractTemplateDid, contractTemplateName)
  })

  // ---- Stage 4: A derives a contract from its registered template through the
  // real UI, naming B (B's own did:web) as the counterparty via the R6 dialog.
  await test.step('A creates a contract with B as counterparty', async () => {
    const bDidWeb = await resolveDidWeb(b)
    contractDid = await createContractViaUi(a, contractTemplateName, bDidWeb)
  })

  // ---- Stage 5: propose to B; B receives it OFFERED with a valid C2PA
  // artifact (banner draft), verified on B's own side.
  let aChain = 0
  let bChain = 0
  await test.step('propose to B; B receives OFFERED with a verifiable artifact', async () => {
    const retrieve = await a.page.request.get(`${E2E_API_BASE}/contract/retrieve/${encodeURIComponent(contractDid)}`)
    const updatedAt = ((await retrieve.json()) as { updated_at?: string }).updated_at
    const offered = await a.page.request.post(`${E2E_API_BASE}/contract/offer`, {
      data: { did: contractDid, updated_at: updatedAt },
    })
    expect(offered.ok(), `offer ${offered.status()}: ${await offered.text()}`).toBeTruthy()

    await assertReceivedInState(b, contractDid, 'OFFERED')
    await verifyArtifact(b, contractDid, { lifecycle: 'draft' })
    aChain = await manifestChainLength(a, contractDid)
    bChain = await manifestChainLength(b, contractDid)
  })

  // ---- Stage 6: non-trivial negotiation ping-pong. Every actionable
  // adjustment ships a new PDF; the C2PA chain grows by one on BOTH parties.
  await test.step('negotiation ping-pong 20000 -> 10000 -> 15000', async () => {
    await counterOffer(b, contractDid, { value: '10000' })
    bChain = await assertManifestChainGrew(b, contractDid, bChain)
    aChain = await assertManifestChainGrew(a, contractDid, aChain)

    await counterOffer(a, contractDid, { value: '15000' })
    aChain = await assertManifestChainGrew(a, contractDid, aChain)
    bChain = await assertManifestChainGrew(b, contractDid, bChain)
  })

  // ---- Stage 7: consolidation/settle. Signing is gated until settlement; the
  // extrinsic lifecycle flips to `agreed` on both sides.
  await test.step('consolidate; signing gated until settled', async () => {
    const early = await b.page.request.post(`${E2E_API_BASE}/signature/apply`, {
      data: { did: contractDid },
    })
    expect(early.ok(), 'a pre-settle signature attempt must be refused').toBeFalsy()

    const settle = await a.page.request.post(`${E2E_API_BASE}/contract/settle`, {
      data: { did: contractDid },
    })
    expect(settle.ok(), `settle ${settle.status()}: ${await settle.text()}`).toBeTruthy()
    await assertReceivedInState(a, contractDid, 'ACCEPTED')
    await assertReceivedInState(b, contractDid, 'ACCEPTED')
  })

  // ---- Stage 8: both sign. A signs A's field, ships; B signs on top of A's
  // signed PDF. The double-signed artifact is PDF/A-3a + C2PA valid (active).
  await test.step('both sign; double-signed artifact verifies', async () => {
    await signOnInstance(a, contractDid, 'Instance A Signatory')
    await assertReceivedInState(b, contractDid, 'SIGNED')
    await signOnInstance(b, contractDid, 'Instance B Signatory')
    await verifyArtifact(a, contractDid, { lifecycle: 'active' })
    await verifyArtifact(b, contractDid, { lifecycle: 'active' })
  })

  // ---- Stages 9-10: deploy to the target, receipt + async KPIs vs policy,
  // and the full audit trail on both instances.
  await test.step('deploy, receipt, KPI, audit', async () => {
    const deployed = await a.page.request.post(`${E2E_API_BASE}/contract/deploy`, {
      data: { did: contractDid },
    })
    expect(deployed.ok(), `deploy ${deployed.status()}: ${await deployed.text()}`).toBeTruthy()

    await a.gotoAs('Auditor', '/ui/audit')
    await a.page.getByLabel('Scope').selectOption('contracts')
    await expect(a.page.getByRole('cell', { name: contractDid }).first()).toBeVisible({
      timeout: 60_000,
    })
  })
})
