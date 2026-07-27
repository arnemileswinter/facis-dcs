import { expect, test } from './dcs-test'
import { buildApprovedContract, gotoAs, signApprovedContractViaViewer } from './lifecycle-helpers'

/**
 * A breached KPI must reach the compliance officer's screen (DCS-FR-CWE-31:
 * "Alerts MUST be raised for underperformance or missed targets").
 *
 * The backend path is already covered by a BDD scenario. What that scenario
 * cannot show is that an officer can actually get to the alert: this drives the
 * whole chain from the officer's and manager's seats, clicking the real
 * controls, and ends on the rendered alert badge rather than on a JSON field.
 *
 * Deployment happens twice here, which is the SRS's own arrangement rather than
 * a quirk of the fixture: DCS-FR-SM-12 automatically triggers deployment when
 * signing completes, and UC-05's stimulus is separately "a Contract Manager
 * submits a signed contract for deployment". So the contract reaches ACTIVE on
 * its own, and the manager then re-dispatches it by hand — the recovery an
 * operator needs when a target was unreachable, lost its state, or was replaced.
 * The state machine treats that as an idempotent ACTIVE -> ACTIVE re-dispatch.
 *
 * Where it deploys to is chosen through the UI as well: an administrator
 * registers a target system, and the manager points the contract at it before
 * signing (ADR-25). Nothing is seeded behind the UI's back.
 *
 * The one step that is not a click is the KPI report: the reporter is the
 * contract *target system*, an external party posting over the deployment
 * callback channel with its shared secret, quoting the correlation id the
 * manager's deployment returned. No DCS user performs it and no control exists
 * to click.
 *
 * Every action a DCS *user* performs — author, submit, review, approve, sign,
 * deploy, sweep — goes through the real controls as the role that owns it.
 *
 * The threshold is the one buildApprovedContract already authors through the
 * clause editor: an ODRL constraint binding the Payment Amount contract field
 * with "less than or equal to" 500. Reporting 900 against that field breaches
 * it. Nothing here is seeded behind the UI's back.
 */

const CALLBACK_SECRET = process.env.E2E_DEPLOYMENT_CALLBACK_SECRET ?? 'bdd-deployment-callback-secret'

/** The shipped ORCE contract-target flow the kind stack runs (values.bdd.yml). */
const E2E_CONTRACT_TARGET_URL = process.env.E2E_CONTRACT_TARGET_URL ?? 'http://dcs-orce:1880/contract-target/deploy'

/** The bearer token the SPA holds for the currently applied role session. */
async function currentToken(page: import('@playwright/test').Page): Promise<string> {
  const token = await page.evaluate(() => window.localStorage.getItem('access_token'))
  expect(token, 'the applied role session must carry an access token').toBeTruthy()
  return token!
}

test('@DCS-FR-CWE-31 @DCS-IR-PACM-03 a breached KPI raises an underperformance alert the compliance officer can see', async ({
  page,
  loginAs,
}) => {
  // The fixture drives template authoring, negotiation, review, approval and
  // signing through the UI; each leg is slow and this test owns all of them.
  test.setTimeout(20 * 60_000)

  const targetName = `E2E Target ${Date.now()}`

  await test.step('the administrator registers a target system', async () => {
    await gotoAs(page, loginAs, 'Sys. Administrator', '/ui/admin/targets')
    await expect(page.getByTestId('target-admin')).toBeVisible()

    await page.getByTestId('target-name').fill(targetName)
    // The shipped ORCE contract-target flow, the same endpoint the BDD suite
    // registers: it verifies the payload hash and acknowledges over the
    // callback channel, which is what drives SIGNED -> ACTIVE.
    await page.getByTestId('target-url').fill(E2E_CONTRACT_TARGET_URL)
    await page.getByTestId('target-description').fill('Registered by the KPI alert e2e.')

    const registered = page.waitForResponse(
      (r) => r.url().includes('/contract/targets') && r.request().method() === 'POST',
    )
    await page.getByTestId('target-save').click()
    const response = await registered
    expect(response.ok(), `register target ${response.status()}`).toBeTruthy()

    await expect(page.getByTestId('target-row').filter({ hasText: targetName })).toHaveCount(1)
  })

  const contractDid = await test.step('author and approve a contract whose ODRL policy bounds a numeric field', () =>
    buildApprovedContract(page, loginAs))

  await test.step('the contract manager points it at that target system', async () => {
    // Before signing, deliberately: signing completion deploys the contract
    // automatically with nobody present to choose a destination, so a contract
    // that reaches it without one does not deploy at all (ADR-25).
    await gotoAs(page, loginAs, 'Contract Manager', `/ui/contracts/view/${contractDid}`)
    await expect(page.getByTestId('contract-target-unset'), 'no target is set yet').toBeVisible()

    await page.getByTestId('contract-target-select').selectOption({ label: targetName })
    const designated = page.waitForResponse(
      (r) => r.url().includes('/contract/target/designate') && r.request().method() === 'POST',
    )
    await page.getByTestId('contract-target-save').click()
    const response = await designated
    expect(response.ok(), `designate ${response.status()}`).toBeTruthy()

    await expect(page.getByTestId('contract-target-name')).toContainText(targetName)
  })

  await test.step('sign it', () => signApprovedContractViaViewer(page, loginAs, contractDid))

  const boundFieldIri =
    await test.step('signing auto-deploys it and the target acknowledges, taking the contract ACTIVE', async () => {
      // Nothing is simulated: the auto-deploy subscriber dispatched to the
      // shipped ORCE contract-target flow, whose callback drives SIGNED ->
      // ACTIVE. Wait for that transition, then read the @id of the field the
      // ODRL constraint binds — a KPI binds to the constraint by node IRI, not
      // by label.
      await gotoAs(page, loginAs, 'Contract Manager', `/ui/contracts/view/${contractDid}`)
      const token = await currentToken(page)
      let fields: { '@id': string }[] = []
      await expect
        .poll(
          async () => {
            const res = await page.request.get(`/api/contract/retrieve/${contractDid}`, {
              headers: { Authorization: `Bearer ${token}` },
            })
            if (!res.ok()) return `retrieve ${res.status()}`
            const body = (await res.json()) as {
              state?: string
              contract_data?: { 'dcs:contractFields'?: { '@id': string }[] }
            }
            fields = body.contract_data?.['dcs:contractFields'] ?? []
            return String(body.state ?? '').toUpperCase()
          },
          {
            message: 'the automatic deployment is acknowledged, driving SIGNED -> ACTIVE',
            timeout: 180_000,
            intervals: [3_000],
          },
        )
        .toBe('ACTIVE')
      expect(fields.length, 'the contract declares the ODRL-bound field a KPI can report against').toBeGreaterThan(0)
      return fields[0]['@id']
    })

  const correlationId = await test.step('the contract manager re-dispatches it to the target', async () => {
    // UC-05's stimulus is a Contract Manager submitting a contract for
    // deployment, and the state machine treats DEPLOY from ACTIVE as an
    // idempotent re-dispatch — the operator's recovery when a target has to
    // receive the contract again. Clicking it is what a manager would do, and
    // the response carries the correlation id the target's callback must quote.
    await gotoAs(page, loginAs, 'Contract Manager', `/ui/contracts/view/${contractDid}`)
    const deploy = page.getByTestId('deploy-contract')
    await expect(deploy, 'a manager can re-dispatch an active contract to its target').toBeVisible({
      timeout: 30_000,
    })
    await expect(deploy).toHaveText('Redeploy')

    // The click reloads the page on success (router.go(0)), which discards the
    // response body before it can be read off a waitForResponse handle. Route
    // the request instead: route.fetch() performs the real call and hands back
    // the real response, which is then passed through unchanged — the body is
    // captured on the way past rather than substituted.
    let captured: { status: number; text: string } | undefined
    await page.route('**/contract/deploy', async (route) => {
      const response = await route.fetch()
      const text = await response.text()
      captured = { status: response.status(), text }
      await route.fulfill({ response, body: text })
    })

    await deploy.click()
    await expect.poll(() => captured !== undefined, { message: 'the deployment request completes' }).toBe(true)
    await page.unroute('**/contract/deploy')

    expect(captured!.status, `redeploy ${captured!.status}: ${captured!.text}`).toBe(200)
    const body = JSON.parse(captured!.text) as { correlation_id?: string; target_name?: string }
    expect(body.correlation_id, 'the deployment names the correlation id its callback quotes').toBeTruthy()
    expect(body.target_name, 'the deployment names the target system it went to').toBeTruthy()
    return body.correlation_id!
  })

  await test.step('the target reports a value that breaches the contract threshold', async () => {
    // 900 against "Payment Amount <= 500" — the constraint authored in the
    // clause editor by the fixture.
    const res = await page.request.post('/api/contract/deployment/callback', {
      headers: { 'X-Deployment-Callback-Secret': CALLBACK_SECRET },
      data: {
        did: contractDid,
        correlation_id: correlationId,
        kpi: { metric: boundFieldIri, value: '900' },
      },
    })
    expect(res.ok(), `KPI callback ${res.status()}: ${await res.text()}`).toBeTruthy()
  })

  await test.step('the compliance officer sweeps and is alerted', async () => {
    await gotoAs(page, loginAs, 'Compliance Officer', '/ui/non-compliance')

    const swept = page.waitForResponse((r) => r.url().includes('/pac/monitor') && r.ok())
    await page.getByTestId('run-monitoring-sweep').click()
    await swept

    // The sweep is system-wide and other suites drive contracts concurrently,
    // so narrow to this contract rather than assume it is the only finding.
    await page.getByTestId('monitor-search').fill(contractDid)

    const row = page.getByTestId('monitor-risk-row').filter({ hasText: contractDid })
    await expect(row, 'the breached contract is flagged').toHaveCount(1, { timeout: 30_000 })
    await expect(row.getByTestId('monitor-risk-type')).toContainText('CONTRACT_UNDERPERFORMANCE')
    await expect(
      row.getByTestId('monitor-risk-underperformance-badge'),
      'underperformance is highlighted as an alert, not listed as another grey row',
    ).toBeVisible()
    // The officer must be able to act on it: the detail names the observed value.
    await expect(row.getByTestId('monitor-risk-detail')).toContainText('900')
  })
})
