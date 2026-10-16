import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { expect, type Page, type Route, test } from '@playwright/test'

// Mock HTTP isolates browser contracts; persistence and authorization have separate backend tests.
const did = 'did:web:example.test:contract:archived'
const templateDid = 'did:web:example.test:template:party'
const profile = `@prefix dcs: <https://w3id.org/facis/dcs/ontology/v1#> .
@prefix odrl: <http://www.w3.org/ns/odrl/2/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
<https://w3id.org/facis/dcs/ontology/v1/odrl-profile> a odrl:Profile ; dcs:defaultConstraintAction dcs:provideCompliantValue .
odrl:eq a odrl:Operator ; rdfs:label "Must equal" ; dcs:appliesToParameterType "string" .`
const contractDocument = {
  '@type': 'dcs:Contract',
  'dcs:metadata': { '@type': 'dcs:ContractMetadata', 'dcs:title': 'Frozen contract' },
  'dcs:documentStructure': {
    '@type': 'dcs:DocumentStructure',
    'dcs:blocks': {
      '@list': [{ '@id': 'frozen-text', '@type': 'dcs:TextBlock', 'dcs:text': 'Frozen archived terms, version 2.' }],
    },
    'dcs:layout': {
      '@list': [{ '@id': 'root', 'dcs:isRoot': true, 'dcs:children': { '@list': [{ '@id': 'frozen-text' }] } }],
    },
  },
  'dcs:contractFields': [],
  'dcs:contractData': [],
  'dcs:policies': { '@type': 'odrl:Agreement', 'odrl:profile': { '@id': 'dcs:profile' } },
}
const snapshot = {
  did,
  name: 'Archived agreement',
  description: 'Frozen description',
  contract_version: 2,
  state: 'SIGNED',
  created_by: 'creator',
  created_at: '2026-10-15T00:00:00Z',
  updated_at: '2026-10-16T00:00:00Z',
  contract_data: contractDocument,
}
const template = {
  did: templateDid,
  version: 1,
  template_type: 'CONTRACT_TEMPLATE',
  party_type: 'MULTI_PARTY',
  name: 'Party template',
  description: 'Party classification fixture',
  state: 'DRAFT',
  created_by: 'creator',
  created_at: snapshot.created_at,
  updated_at: snapshot.updated_at,
  template_data: { ...contractDocument, '@type': 'dcs:ContractTemplate' },
}
async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}
async function auth(page: Page, role: string) {
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url')
  const token = `${encode({ alg: 'none' })}.${encode({ sub: 'did:test:actor', exp: Math.floor(Date.now() / 1000) + 3600, roles: [role], ext: { iss: 'did:web:example.test', roles: [role] } })}.unsigned`
  await page.addInitScript((token) => {
    localStorage.setItem('access_token', token)
    localStorage.setItem('token_type', 'Bearer')
  }, token)
  await page.route(
    (url) => url.pathname.startsWith('/api/'),
    (route) => json(route, { message: 'Unexpected API request in mock test' }, 500),
  )
  await page.route('**/auth/refresh', (route) => json(route, { access_token: token, token_type: 'Bearer' }))
  await page.route('**/semantic/schema/list', (route) => json(route, []))
  await page.route('**/semantic/ontology/dcs-odrl-profile', (route) => json(route, { content: profile }))
}
async function noOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(
    true,
  )
}
for (const width of [390, 1280]) {
  test(`P05 Observer searches and reads frozen version without privileged or current-data calls at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 })
    await auth(page, 'Contract Observer')
    const forbiddenCalls: string[] = []
    page.on('request', (request) => {
      if (/archive\/(statistics|erasure-status|annotate|delete)|pac\/|contract\/retrieve|pdf\//.test(request.url()))
        forbiddenCalls.push(request.url())
    })
    await page.route('**/archive/retrieve', (route) => json(route, { contracts: [snapshot] }))
    let searchParams: URLSearchParams | undefined
    await page.route('**/archive/search**', (route) => {
      searchParams = new URL(route.request().url()).searchParams
      return json(route, [snapshot])
    })
    await page.route('**/api/archive/detail**', (route) => {
      expect(new URL(route.request().url()).searchParams.get('contract_version')).toBe('2')
      return json(route, snapshot)
    })
    await page.goto('/ui/archive/dashboard')
    await expect(page.getByRole('link', { name: 'Archived agreement' })).toBeVisible()
    await expect(page.getByTestId('archive-statistics')).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Annotate' })).toHaveCount(0)
    await page.getByLabel('Name', { exact: true }).fill('Archived')
    await page.getByLabel('Version', { exact: true }).fill('2')
    await page.getByLabel('DID', { exact: true }).fill(did)
    await page.getByLabel('Party', { exact: true }).fill('did:test:party')
    await page.getByLabel('State').selectOption('SIGNED')
    await page.getByLabel('Tag', { exact: true }).fill('renewed')
    await page.getByLabel('Valid from').fill('2026-10-01')
    await page.getByLabel('Valid until').fill('2026-10-31')
    await page.getByRole('button', { name: 'Search archive' }).press('Enter')
    await expect.poll(() => searchParams?.get('contract_version')).toBe('2')
    expect(Object.fromEntries(searchParams ?? [])).toEqual({
      name: 'Archived',
      contract_version: '2',
      did,
      party: 'did:test:party',
      state: 'SIGNED',
      tag: 'renewed',
      valid_from: '2026-10-01T00:00:00.000Z',
      valid_until: '2026-10-31T00:00:00.000Z',
    })
    await noOverflow(page)
    await page.getByRole('link', { name: 'Archived agreement' }).click()
    await expect(page.getByRole('heading', { name: 'Archived agreement — Version 2' })).toBeVisible()
    await expect(page.getByText('Frozen archived terms, version 2.', { exact: true })).toBeVisible()
    await page.getByTestId('machine-readable-toggle').press('Enter')
    await expect(page.getByTestId('machine-readable-payload')).toContainText('Frozen archived terms')
    await expect(page.locator('section[aria-label="Archived contract content"] input')).toHaveCount(0)
    await noOverflow(page)
    expect(forbiddenCalls).toEqual([])
  })
}
for (const status of [403, 404, 500]) {
  test(`P05 detail ${status} leaves no snapshot and retry recovers`, async ({ page }) => {
    await auth(page, 'Contract Observer')
    let failed = true
    await page.route('**/api/archive/detail**', (route) =>
      failed ? json(route, { message: `Snapshot error ${status}` }, status) : json(route, snapshot),
    )
    await page.goto(`/ui/archive/detail?did=${encodeURIComponent(did)}&contract_version=2`)
    await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible()
    await expect(page.getByTestId('machine-readable-view')).toHaveCount(0)
    failed = false
    await page.getByRole('button', { name: 'Retry', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Archived agreement — Version 2' })).toBeVisible()
  })
}
test('P09 explicit choice is required, persisted independently, and loaded into editable details', async ({ page }) => {
  await auth(page, 'Template Creator')
  const writes: Record<string, unknown>[] = []
  await page.route('**/template/retrieve**', (route) =>
    json(route, { contract_templates: [], review_tasks: [], approval_tasks: [] }),
  )
  await page.route('**/template/create', (route) => {
    writes.push(route.request().postDataJSON())
    return json(route, { did: templateDid })
  })
  await page.goto('/ui/templates/new')
  await page.getByRole('button', { name: /^Contract/ }).click()
  await page.getByTestId('template-global-name').fill('Party template')
  await page.getByTestId('template-base-description').fill('Party classification fixture')
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(page.getByTestId('template-party-type')).toBeFocused()
  await expect(page.getByText('Party type is required.')).toBeVisible()
  expect(writes).toHaveLength(0)
  await page.getByTestId('template-party-type').selectOption('MULTI_PARTY')
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  await expect.poll(() => writes.length).toBe(1)
  expect(writes[0]).toMatchObject({ template_type: 'CONTRACT_TEMPLATE', party_type: 'MULTI_PARTY' })
  await page.route('**/template/retrieve/**', (route) => json(route, template))
  await page.goto(`/ui/templates/edit/${encodeURIComponent(templateDid)}`)
  await expect(page.getByTestId('template-party-type')).toHaveValue('MULTI_PARTY')
  await page.getByTestId('template-party-type').selectOption('SINGLE_PARTY')
  await page.route('**/template/update', (route) => {
    writes.push(route.request().postDataJSON())
    return json(route, { did: templateDid })
  })
  await page.getByRole('button', { name: 'Update', exact: true }).click()
  await expect.poll(() => writes.length).toBe(2)
  expect(writes[1]).toMatchObject({ party_type: 'SINGLE_PARTY' })
  await page.goto('/ui/templates/new')
  await page.getByRole('button', { name: /^Component/ }).click()
  await expect(page.getByTestId('template-party-type')).toHaveValue('')
})
for (const catalogue of [false, true]) {
  test(`P09 ${catalogue ? 'federated catalogue' : 'repository'} combines visible text and party filters and ignores old responses`, async ({
    page,
  }) => {
    await auth(page, catalogue ? 'Template Manager' : 'Template Creator')
    await page.route('**/template/retrieve**', (route) =>
      json(route, { contract_templates: [], review_tasks: [], approval_tasks: [] }),
    )
    await page.route('**/catalogue/template/retrieve**', (route) => json(route, { totalCount: 0, items: [] }))
    const requests: Record<string, string>[] = []
    let older: Route | undefined
    const response = (item: typeof template) =>
      catalogue ? { totalCount: 1, items: [item] } : [{ ...item, version: '1' }]
    await page.route(catalogue ? '**/catalogue/template/search**' : '**/template/search**', (route) => {
      const params = Object.fromEntries(new URL(route.request().url()).searchParams)
      requests.push(params)
      if (params.party_type === 'SINGLE_PARTY') {
        older = route
        return
      }
      return json(route, response(template))
    })
    await page.goto(catalogue ? '/ui/catalogues/templates' : '/ui/templates')
    const input = page.getByRole('combobox', { name: catalogue ? 'Search catalogue templates' : 'Search templates' })
    await input.fill(templateDid)
    await page.getByRole('option', { name: templateDid, exact: true }).click()
    await page.getByRole('button', { name: 'Search', exact: true }).click()
    await page.getByLabel('Party type', { exact: true }).selectOption('SINGLE_PARTY')
    await expect.poll(() => older !== undefined).toBe(true)
    await page.getByLabel('Party type', { exact: true }).selectOption('MULTI_PARTY')
    await expect.poll(() => requests[requests.length - 1]?.party_type).toBe('MULTI_PARTY')
    expect(requests[requests.length - 1]).toMatchObject({ did: templateDid, party_type: 'MULTI_PARTY' })
    await expect(input).toHaveValue(templateDid)
    const oldResponse = page.waitForResponse(
      (res) => new URL(res.url()).searchParams.get('party_type') === 'SINGLE_PARTY',
    )
    if (!older) throw new Error('Expected deferred party query')
    await json(older, response({ ...template, name: 'Stale template' }))
    await (await oldResponse).finished()
    await page.getByRole('button', { name: 'Search', exact: true }).click()
    await expect(page.getByText(catalogue ? 'Name: Party template' : 'Party template', { exact: true })).toBeVisible()
    await expect(page.getByText('Stale template', { exact: false })).toHaveCount(0)
    await input.fill('')
    await expect.poll(() => requests[requests.length - 1]?.did).toBeUndefined()
    expect(requests[requests.length - 1]?.party_type).toBe('MULTI_PARTY')
    await page.getByLabel('Party type', { exact: true }).selectOption('')
    await expect(page.getByText(catalogue ? 'Name: Party template' : 'Party template', { exact: true })).toHaveCount(0)
  })
}
test('P09 federated details retain party classification when remote template data is unavailable', async ({ page }) => {
  await auth(page, 'Template Manager')
  await page.route('**/template/retrieve**', (route) =>
    json(route, { contract_templates: [], review_tasks: [], approval_tasks: [] }),
  )
  await page.route('**/catalogue/template/retrieve/**', (route) =>
    json(route, { ...template, template_data: undefined }),
  )
  await page.goto(`/ui/catalogues/templates/view/${encodeURIComponent(templateDid)}?version=1`)
  await expect(page.getByText('Remote template source is currently unreachable.')).toBeVisible()
  await expect(page.getByText(/Party type:.*Multi-party/)).toBeVisible()
})
for (const width of [390, 1280]) {
  test(`F10 validates, retries failed upload, prevents duplicates, refreshes and downloads exact bytes at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 })
    let stored = false
    let failed = true
    const bytes = Buffer.from([0, 1, 128, 255, 10, 13, 65])
    const sha256 = createHash('sha256').update(bytes).digest('hex')
    const entry = {
      evidence_id: 'qa-evidence',
      did,
      contract_version: 2,
      recorded_by: 'manager',
      recorded_at: snapshot.updated_at,
      filename: 'proof.bin',
      description: 'Signed evidence',
      media_type: 'application/octet-stream',
      sha256,
    }
    const posts: Record<string, unknown>[] = []
    // Install handlers after auth's catch-all via a page init wrapper below.
    await auth(page, 'Contract Manager')
    await page.route('**/contract/retrieve**', (route) =>
      json(
        route,
        route.request().url().includes('/retrieve/')
          ? snapshot
          : { contracts: [], review_tasks: [], approval_tasks: [], negotiation_tasks: [] },
      ),
    )
    await page.route('**/contract/audit', (route) => json(route, []))
    await page.route('**/contract/*/evidence', (route) => json(route, stored ? [entry] : []))
    await page.route('**/contract/*/evidence/qa-evidence', (route) =>
      json(route, { ...entry, content: bytes.toString('base64') }),
    )
    let pending: Route | undefined
    await page.route('**/contract/store', async (route) => {
      posts.push(route.request().postDataJSON())
      if (failed) return json(route, { message: 'Evidence service unavailable' }, 500)
      pending = route
    })
    await page.goto(`/ui/contracts/view/${encodeURIComponent(did)}`)
    await page.getByRole('tab', { name: 'Audit History' }).press('Enter')
    await page.getByRole('button', { name: 'Store evidence' }).click()
    await expect(page.getByLabel('Evidence file', { exact: true })).toBeFocused()
    await page.getByRole('button', { name: 'Save evidence' }).click()
    await expect(page.getByText('Choose a non-empty evidence file.')).toBeVisible()
    await page
      .getByLabel('Evidence file', { exact: true })
      .setInputFiles({ name: 'proof.bin', mimeType: 'application/octet-stream', buffer: bytes })
    await page.getByRole('button', { name: 'Save evidence' }).click()
    await expect(page.getByLabel('Evidence description')).toBeFocused()
    await page.getByLabel('Evidence description').fill('Signed evidence')
    await page.getByRole('button', { name: 'Save evidence' }).click()
    await expect(page.locator('form [role="alert"]')).toContainText('Evidence service unavailable')
    await expect(page.getByLabel('Evidence description')).toHaveValue('Signed evidence')
    failed = false
    await page.getByRole('button', { name: 'Save evidence' }).click()
    await expect.poll(() => posts.length).toBe(2)
    await expect(page.getByRole('button', { name: 'Storing evidence…' })).toBeDisabled()
    expect(Buffer.from(String(posts[1]?.content), 'base64')).toEqual(bytes)
    expect(posts[1]).toMatchObject({
      did,
      updated_at: snapshot.updated_at,
      description: 'Signed evidence',
      filename: 'proof.bin',
    })
    expect(pending).toBeDefined()
    stored = true
    if (pending) await json(pending, { did, evidence_id: entry.evidence_id })
    await expect(page.getByText(`SHA-256: ${sha256}`)).toBeVisible()
    await noOverflow(page)
    const downloadEvent = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Download proof.bin' }).click()
    const download = await downloadEvent
    const path = await download.path()
    expect(path).not.toBeNull()
    if (!path) throw new Error('Download path unavailable')
    const downloaded = await readFile(path)
    expect(downloaded).toEqual(bytes)
    expect(createHash('sha256').update(downloaded).digest('hex')).toBe(sha256)
    expect(download.suggestedFilename()).toBe('proof.bin')
    await page.getByRole('button', { name: 'Store evidence' }).click()
    await page.getByLabel('Evidence description').fill('Discard me')
    await page.keyboard.press('Escape')
    await expect(page.getByRole('button', { name: 'Store evidence' })).toBeFocused()
    expect(posts).toHaveLength(2)
    await page.reload()
    await page.getByRole('tab', { name: 'Audit History' }).press('Enter')
    await expect(page.getByText(`SHA-256: ${sha256}`)).toBeVisible()
  })
}
test('F10 observers cannot access evidence controls or trigger evidence requests', async ({ page }) => {
  const evidenceRequests: string[] = []
  page.on('request', (req) => {
    if (/\/evidence|\/contract\/store/.test(req.url())) evidenceRequests.push(req.url())
  })
  await auth(page, 'Contract Observer')
  await page.route('**/contract/retrieve**', (route) =>
    json(
      route,
      route.request().url().includes('/retrieve/')
        ? snapshot
        : { contracts: [], review_tasks: [], approval_tasks: [], negotiation_tasks: [] },
    ),
  )
  await page.goto(`/ui/contracts/view/${encodeURIComponent(did)}`)
  await expect(page.getByRole('tab', { name: 'Contract Details' })).toBeVisible()
  await expect(page.getByRole('tab', { name: 'Audit History' })).toHaveCount(0)
  expect(evidenceRequests).toEqual([])
})
