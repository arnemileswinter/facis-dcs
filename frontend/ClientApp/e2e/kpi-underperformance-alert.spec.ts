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
 * The one step that is not a click is the KPI report itself, and deliberately
 * so — the reporter is the contract *target system*, an external party posting
 * over the deployment callback channel with its shared secret. There is no UI
 * for it because no DCS user performs it. Every action a DCS user does perform
 * (author, submit, review, approve, sign, deploy, sweep) goes through the real
 * controls as the role that owns it.
 *
 * The threshold is the one buildApprovedContract already authors through the
 * clause editor: an ODRL constraint binding the Payment Amount contract field
 * with "less than or equal to" 500. Reporting 900 against that field breaches
 * it. Nothing here is seeded behind the UI's back.
 */

const CALLBACK_SECRET = process.env.E2E_DEPLOYMENT_CALLBACK_SECRET ?? 'bdd-deployment-callback-secret'

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

  const contractDid =
    await test.step('author, approve and sign a contract whose ODRL policy bounds a numeric field', async () => {
      const did = await buildApprovedContract(page, loginAs)
      await signApprovedContractViaViewer(page, loginAs, did)
      return did
    })

  const correlationId = await test.step('the contract manager deploys it to the contract target', async () => {
    await gotoAs(page, loginAs, 'Contract Manager', `/ui/contracts/view/${contractDid}`)
    const deploy = page.getByRole('button', { name: 'Deploy', exact: true })
    await expect(deploy, 'Deploy is offered to the manager once the contract is SIGNED').toBeVisible({
      timeout: 30_000,
    })
    const deployed = page.waitForResponse(
      (r) => r.url().includes('/contract/deploy') && r.request().method() === 'POST',
    )
    await deploy.click()
    const response = await deployed
    expect(response.ok(), `deploy ${response.status()}: ${await response.text()}`).toBeTruthy()
    const body = (await response.json()) as { correlation_id?: string }
    expect(body.correlation_id, 'deploy returns the correlation id the callback quotes').toBeTruthy()
    return body.correlation_id!
  })

  const boundFieldIri =
    await test.step('the target acknowledges the deployment, taking the contract ACTIVE', async () => {
      // Nothing is simulated: the backend dispatched to the shipped ORCE
      // contract-target flow, whose callback drives SIGNED -> ACTIVE. Wait for
      // that transition, then read the @id of the field the ODRL constraint
      // binds — a KPI binds to the constraint by node IRI, not by label.
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
          { message: 'the target acknowledgement drives SIGNED -> ACTIVE', timeout: 120_000, intervals: [2_000] },
        )
        .toBe('ACTIVE')
      expect(fields.length, 'the contract declares the ODRL-bound field a KPI can report against').toBeGreaterThan(0)
      return fields[0]['@id']
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
