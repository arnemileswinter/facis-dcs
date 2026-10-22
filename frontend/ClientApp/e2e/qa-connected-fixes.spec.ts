import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { expect, test } from './dcs-test'
import { buildApprovedContract, FIXTURE_CLAUSE_PROSE, gotoAs, signApprovedContractViaViewer } from './lifecycle-helpers'

test('P04 P05 P06 F10 persist archive annotations, exact snapshots, evidence and complete reports through real services', async ({
  page,
  loginAs,
}) => {
  test.setTimeout(900_000)
  page.setDefaultTimeout(30_000)
  const did = await buildApprovedContract(page, loginAs)
  await signApprovedContractViaViewer(page, loginAs, did)

  await test.step('F10 manager stores and downloads the exact evidence after reload', async () => {
    await gotoAs(page, loginAs, 'Contract Manager', `/ui/contracts/view/${encodeURIComponent(did)}`)
    await page.getByRole('tab', { name: 'Audit History' }).click()
    await page.getByRole('button', { name: 'Store evidence', exact: true }).click()
    const bytes = Buffer.from([0, 1, 128, 255, 10, 13, 65])
    const hash = createHash('sha256').update(bytes).digest('hex')
    await page.getByLabel('Evidence file', { exact: true }).setInputFiles({
      name: 'connected-proof.bin',
      mimeType: 'application/octet-stream',
      buffer: bytes,
    })
    await page.getByLabel('Evidence description').fill('Connected QA evidence')
    const stored = page.waitForResponse((response) => response.url().endsWith('/contract/store'))
    await page.getByRole('button', { name: 'Save evidence', exact: true }).click()
    const response = await stored
    expect(response.ok(), await response.text()).toBeTruthy()
    await page.reload()
    await page.getByRole('tab', { name: 'Audit History' }).click()
    await expect(page.getByText(`SHA-256: ${hash}`)).toBeVisible()
    await expect(page.getByText('Connected QA evidence', { exact: true })).toBeVisible()
    const downloading = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Download connected-proof.bin' }).click()
    const download = await downloading
    expect(download.suggestedFilename()).toBe('connected-proof.bin')
    const file = await download.path()
    expect(file).not.toBeNull()
    expect(await readFile(file)).toEqual(bytes)
  })

  let version = ''
  await test.step('P04 archive summary survives reload and a tags-only update', async () => {
    await gotoAs(page, loginAs, 'Archive Manager', '/ui/archive/dashboard')
    await page.getByRole('button', { name: 'Additional filters' }).click()
    await page.getByLabel('DID', { exact: true }).fill(did)
    await page.getByRole('button', { name: 'Search archive', exact: true }).click()
    const row = page.getByTestId(`archive-entry-${did}`).last()
    await expect(row).toBeVisible()
    version = (await row.getByRole('cell').nth(1).innerText()).trim()
    const summary = `Connected QA summary ${Date.now()}`
    await row.getByRole('button', { name: 'Annotate', exact: true }).click()
    await page.getByPlaceholder('Summary', { exact: true }).fill(summary)
    await page.getByRole('dialog').getByRole('button', { name: 'Submit', exact: true }).click()
    const annotated = page.waitForResponse((response) => response.url().endsWith('/archive/annotate'))
    await page.getByRole('dialog').getByRole('button', { name: 'Submit', exact: true }).click()
    const response = await annotated
    expect(response.ok(), await response.text()).toBeTruthy()
    await page.reload()
    await expect(page.getByTestId(`archive-annotation-${did}`).last()).toContainText(summary)
    await page.getByTestId(`archive-annotate-${did}`).last().click()
    await page.getByRole('dialog').getByRole('button', { name: 'Submit', exact: true }).click()
    await page.getByPlaceholder('e.g. supply, 2026, renewed').fill('connected-qa')
    const tagged = page.waitForResponse((response) => response.url().endsWith('/archive/annotate'))
    await page.getByRole('dialog').getByRole('button', { name: 'Submit', exact: true }).click()
    const tagsResponse = await tagged
    expect(tagsResponse.ok(), await tagsResponse.text()).toBeTruthy()
    expect(tagsResponse.request().postDataJSON()).not.toHaveProperty('summary')
    await page.reload()
    await expect(page.getByTestId(`archive-annotation-${did}`).last()).toContainText(summary)
    await expect(page.getByTestId(`archive-annotation-${did}`).last()).toContainText('connected-qa')
  })

  await test.step('P05 observer searches and opens the immutable exact archive version', async () => {
    await gotoAs(page, loginAs, 'Contract Observer', '/ui/archive/dashboard')
    await expect(page.getByTestId('archive-statistics')).toHaveCount(0)
    await page.getByRole('button', { name: 'Additional filters' }).click()
    await page.getByLabel('DID', { exact: true }).fill(did)
    await page.getByLabel('Version', { exact: true }).fill(version)
    await page.getByLabel('Tag', { exact: true }).fill('connected-qa')
    await page.getByRole('button', { name: 'Search archive', exact: true }).click()
    const row = page.getByTestId(`archive-entry-${did}`)
    await expect(row).toHaveCount(1)
    await expect(row.getByRole('button', { name: 'Annotate', exact: true })).toHaveCount(0)
    const detail = page.waitForResponse((response) => new URL(response.url()).pathname.endsWith('/archive/detail'))
    await row.getByRole('link').first().click()
    const response = await detail
    expect(response.ok(), await response.text()).toBeTruthy()
    const snapshot = await response.json()
    expect(snapshot.did).toBe(did)
    expect(snapshot.contract_version).toBe(Number(version))
    await expect(page.getByText('This is the immutable archived version.')).toBeVisible()
    await expect(page.getByText(FIXTURE_CLAUSE_PROSE, { exact: true })).toBeVisible()
    const archivedContent = page.getByRole('region', { name: 'Archived contract content' })
    const amount = archivedContent.getByRole('textbox', { name: 'Payment Amount', exact: true })
    await expect(amount).toHaveValue('250')
    await expect(amount).toBeDisabled()
    await expect(amount).not.toBeEditable()
    await expect(page.getByRole('button', { name: /^(Save|Update)$/ })).toHaveCount(0)
    await page.getByTestId('machine-readable-toggle').click()
    await expect(page.getByTestId('machine-readable-payload')).toContainText(FIXTURE_CLAUSE_PROSE)
  })

  await test.step('P06 auditor downloads JSON, CSV and PDF containing the lifecycle', async () => {
    await gotoAs(page, loginAs, 'Auditor', '/ui/audit')
    await page.locator('#select-scope').selectOption('contracts')
    await page.getByLabel('DID (optional)').fill(did)
    await page.getByLabel('Audit justification').fill('Connected QA report acceptance')
    const audited = page.waitForResponse((response) => response.url().endsWith('/pac/audit'), { timeout: 120_000 })
    await page.getByRole('button', { name: 'Execute Audit', exact: true }).click()
    const response = await audited
    expect(response.ok(), await response.text()).toBeTruthy()
    for (const format of ['json', 'csv', 'pdf']) {
      const downloading = page.waitForEvent('download')
      await page.getByRole('button', { name: format.toUpperCase(), exact: true }).click()
      const download = await downloading
      expect(download.suggestedFilename()).toBe(`audit-report-contracts.${format}`)
      const file = await download.path()
      expect(file).not.toBeNull()
      const bytes = await readFile(file)
      await test.info().attach(`connected-report.${format}`, { path: file })
      if (format === 'json') {
        const report = JSON.parse(bytes.toString())
        expect(report.did).toBe(did)
        expect(report.resources).toEqual(expect.arrayContaining([expect.objectContaining({ did })]))
        expect(report.events.length).toBeGreaterThan(0)
        expect(report.events).toEqual(expect.arrayContaining([expect.objectContaining({ did })]))
        expect(report.summary.totalEvents).toBe(report.events.length)
        expect(report.executor.id).toBeTruthy()
      } else if (format === 'csv') {
        expect(bytes.toString().split(/\r?\n/, 1)[0]?.split(',')).toContain('eventType')
        expect(bytes.toString()).toContain(did)
      } else {
        expect(bytes.subarray(0, 5).toString()).toBe('%PDF-')
        expect(bytes.length).toBeGreaterThan(1000)
      }
    }
  })
})
