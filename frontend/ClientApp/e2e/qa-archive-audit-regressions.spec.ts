import { readFile } from 'node:fs/promises'
import { expect, type Page, type Route, test } from '@playwright/test'

// HTTP is mocked: these checks prove browser behavior, not backend persistence or authorization.
const did = 'did:web:example.test:contract:qa'
const profile = `@prefix dcs: <https://w3id.org/facis/dcs/ontology/v1#> .
@prefix odrl: <http://www.w3.org/ns/odrl/2/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
<https://w3id.org/facis/dcs/ontology/v1/odrl-profile> a odrl:Profile ; dcs:defaultConstraintAction dcs:provideCompliantValue .
odrl:eq a odrl:Operator ; rdfs:label "Must equal" ; dcs:appliesToParameterType "string" .`
async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}
async function authenticate(page: Page, role: string) {
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url')
  const token = `${encode({ alg: 'none', typ: 'JWT' })}.${encode({ sub: 'did:web:auditor.test', exp: Math.floor(Date.now() / 1000) + 3600, roles: [role], ext: { iss: 'did:web:example.test', roles: [role] } })}.unsigned`
  await page.addInitScript((token) => {
    localStorage.setItem('token_type', 'Bearer')
    localStorage.setItem('access_token', token)
  }, token)
  await page.route('**/auth/refresh', (route) => json(route, { token_type: 'Bearer', access_token: token }))
  await page.route('**/semantic/schema/list', (route) => json(route, []))
  await page.route('**/semantic/ontology/dcs-odrl-profile', (route) => json(route, { content: profile }))
}
async function archive(page: Page) {
  await authenticate(page, 'Archive Manager')
  await page.route('**/archive/statistics', (route) =>
    json(route, {
      archived_total: 1,
      archived_contracts: 1,
      deleted_total: 0,
      storage_bytes: 100,
      compliant_total: 1,
      flagged_total: 0,
      recent_actions: [],
      expiring_contracts: [],
    }),
  )
  await page.route('**/archive/erasure-status**', (route) => json(route, { did, local_status: 'live', peers: [] }))
}
async function enterSummary(page: Page) {
  await page.getByTestId(`archive-annotate-${did}`).click()
  await page.getByPlaceholder('Summary', { exact: true }).fill('  Retained QA archive summary  ')
  await page.getByRole('dialog').getByRole('button', { name: 'Submit' }).click()
  await expect(page.getByPlaceholder('e.g. supply, 2026, renewed')).toBeVisible()
}
for (const failed of [false, true]) {
  test(`P04 annotation ${failed ? 'failure is visible' : 'survives confirmation and reload; cancellation sends no POST'}`, async ({
    page,
  }) => {
    await archive(page)
    let storedSummary = 'Original summary'
    const posts: unknown[] = []
    await page.route('**/archive/retrieve', (route) =>
      json(route, {
        contracts: [
          { did, name: 'QA archive contract', state: 'ARCHIVED', contract_version: 1, archive_summary: storedSummary },
        ],
      }),
    )
    await page.route('**/archive/annotate', async (route) => {
      const body = route.request().postDataJSON() as { summary: string }
      posts.push(body)
      if (failed) return json(route, { message: 'Annotation storage unavailable' }, 500)
      storedSummary = body.summary
      await json(route, { did, summary: storedSummary, tags: [] })
    })
    await page.goto('/ui/archive/dashboard')
    await enterSummary(page)
    await page.getByRole('dialog').getByRole('button', { name: 'Submit' }).click()
    await expect.poll(() => posts.length).toBe(1)
    expect(posts[0]).toEqual({ did, summary: 'Retained QA archive summary' })
    if (failed) await expect(page.getByRole('alert')).toContainText('Annotation storage unavailable')
    const expected = failed ? 'Original summary' : 'Retained QA archive summary'
    await expect(page.getByTestId(`archive-annotation-${did}`)).toHaveText(expected)
    await page.reload()
    await expect(page.getByTestId(`archive-annotation-${did}`)).toHaveText(expected)
    for (const cancel of ['button', 'escape']) {
      await page.getByTestId(`archive-annotate-${did}`).click()
      if (cancel === 'button') await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click()
      else await page.keyboard.press('Escape')
      await expect(page.getByRole('dialog')).not.toBeVisible()
      expect(posts).toHaveLength(1)
      await enterSummary(page)
      await page.keyboard.press('Escape')
      await expect(page.getByRole('dialog')).not.toBeVisible()
      expect(posts).toHaveLength(1)
    }
  })
}
async function audit(page: Page) {
  await authenticate(page, 'Auditor')
  await page.route('**/pac/audit', (route) =>
    json(route, {
      audit_id: 'qa-audit',
      correlation_id: 'qa-correlation',
      executed_at: '2026-10-15T10:00:00Z',
      scope: 'contracts',
      executor: { id: 'policy-engine', version: '1' },
      findings: [],
    }),
  )
  await page.goto('/ui/audit')
  await page.getByLabel('Audit justification').fill('Investigate QA report')
  await page.getByRole('button', { name: 'Execute Audit' }).click()
  await expect(page.getByRole('button', { name: 'JSON', exact: true })).toBeEnabled()
}
for (const [format, contentType, bytes] of [
  ['json', 'application/json', '{"reportId":"qa-audit","events":[{"eventType":"ARCHIVED"}]}'],
  ['csv', 'text/csv', 'eventType,occurredAt\r\nARCHIVED,2026-10-15T10:00:00Z\r\n'],
  ['pdf', 'application/pdf', '%PDF-1.4\nQA transport fixture\n%%EOF\n'],
] as const) {
  test(`P06 ${format} download preserves server bytes and filename`, async ({ page }) => {
    await audit(page)
    await page.route('**/pac/report?**', async (route) => {
      const params = new URL(route.request().url()).searchParams
      expect(params.get('format')).toBe(format)
      expect(params.get('scope')).toBe('contracts')
      expect(params.get('justification')).toBe('Investigate QA report')
      await route.fulfill({ contentType, body: bytes })
    })
    const promise = page.waitForEvent('download')
    await page.getByRole('button', { name: format.toUpperCase(), exact: true }).click()
    const download = await promise
    expect(download.suggestedFilename()).toBe(`audit-report-contracts.${format}`)
    expect(await readFile(await download.path(), 'utf8')).toBe(bytes)
  })
}
for (const status of [404, 500]) {
  test(`P06 JSON error ${status} displays once without downloading an error document`, async ({ page }) => {
    await audit(page)
    let downloads = 0
    page.on('download', () => downloads++)
    await page.route('**/pac/report?**', (route) =>
      json(
        route,
        { message: status === 404 ? 'Report unavailable after erasure' : 'Report generation unavailable' },
        status,
      ),
    )
    await page.getByRole('button', { name: 'PDF', exact: true }).click()
    await expect(page.getByRole('alert')).toHaveText('Audit report could not be generated.')
    await expect(page.getByRole('button', { name: 'PDF', exact: true })).toBeEnabled()
    expect(downloads).toBe(0)
  })
}
const denied = {
  id: 9,
  attempted_at: '2026-10-15T10:00:00Z',
  attempt_by: '<img src=x onerror=alert(1)>',
  roles: ['ARCHIVE_MANAGER'],
  service: 'ProcessAuditAndCompliance',
  method: 'Audit',
  scope: 'contracts',
  did,
  justification: 'Denied audit request',
  outcome: 'DENIED',
}
for (const width of [390, 1280]) {
  test(`F02 access logs load, page, clear on failure and retry at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await authenticate(page, 'Auditor')
    const requests: URLSearchParams[] = []
    let pending: Route | undefined
    await page.route('**/pac/access-attempts?**', (route) => {
      requests.push(new URL(route.request().url()).searchParams)
      pending = route
    })
    await page.goto('/ui/audit')
    const summary = page.locator('summary').filter({ hasText: /^Access attempts$/ })
    await summary.focus()
    await page.keyboard.press('Enter')
    const section = page.locator('details').filter({ has: summary })
    const load = section.getByRole('button', { name: 'Load access attempts' })
    await expect(load).toBeDisabled()
    expect(requests).toHaveLength(0)
    await page.getByLabel('Access log justification').fill('  Investigate access denials  ')
    await load.click()
    await expect(section.getByRole('status')).toHaveText('Loading access attempts…')
    await expect(load).toBeDisabled()
    await expect.poll(() => requests.length).toBe(1)
    expect(requests[0].get('justification')).toBe('Investigate access denials')
    await json(pending!, { attempts: [denied], next_cursor: 'opaque:first/+' })
    const region = section.getByRole('region', { name: 'Access attempts table' })
    await expect(region).toContainText('DENIED')
    await expect(region).toContainText(denied.attempt_by)
    await expect(region.locator('img')).toHaveCount(0)
    await region.focus()
    await expect(region).toBeFocused()
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth),
    ).toBe(true)
    await page.getByLabel('Access log justification').fill('Changed purpose')
    await section.getByRole('button', { name: 'Go to next page' }).click()
    await expect.poll(() => requests.length).toBe(2)
    expect(requests[1].get('cursor')).toBe('opaque:first/+')
    expect(requests[1].get('justification')).toBe('Investigate access denials')
    await json(pending!, { attempts: [{ ...denied, id: 8, outcome: 'FAILED' }] })
    await expect(region).toContainText('FAILED')
    await section.getByRole('button', { name: 'Go to previous page' }).click()
    await expect.poll(() => requests.length).toBe(3)
    expect(requests[2].has('cursor')).toBe(false)
    await json(pending!, { attempts: [denied], next_cursor: 'opaque:first/+' })
    await expect(region).toContainText('DENIED')
    await load.click()
    await expect.poll(() => requests.length).toBe(4)
    expect(requests[3].get('justification')).toBe('Changed purpose')
    expect(requests[3].has('cursor')).toBe(false)
    await json(pending!, { message: 'Access unavailable' }, 500)
    await expect(page.getByRole('alert')).toHaveText('Access attempts could not be loaded. Please try again.')
    await expect(region).toHaveCount(0)
    await expect(section.getByText('No access attempts found.')).toHaveCount(0)
    await load.click()
    await expect.poll(() => requests.length).toBe(5)
    await json(pending!, { attempts: [] })
    await expect(section.getByRole('status')).toHaveText('No access attempts found.')
    await expect(page.getByRole('alert')).toHaveCount(0)
  })
}
test('F02 Archive Manager cannot see or request the access log panel', async ({ page }) => {
  await authenticate(page, 'Archive Manager')
  let requests = 0
  await page.route('**/pac/access-attempts**', (route) => {
    requests++
    return json(route, { attempts: [] })
  })
  await page.goto('/ui/audit')
  await expect(page.getByRole('heading', { name: 'Audit', exact: true })).toBeVisible()
  await expect(page.locator('summary').filter({ hasText: /^Access attempts$/ })).toHaveCount(0)
  expect(requests).toBe(0)
})
