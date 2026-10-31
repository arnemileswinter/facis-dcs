import { expect, test } from './dcs-test'
import { buildApprovedContract, gotoAs, signApprovedContractWithWallet } from './lifecycle-helpers'

/**
 * C2PA verifier UI (SRS DCS-OR-C2PA-006, -008) against the real backend: a
 * Contract Manager verifies the stored PDF of a signed contract and sees the
 * Active banner and the four named checks, with the PAdES signature checked by
 * the EU DSS; then verifies a copy whose page text was changed and sees the
 * mismatch.
 */
test('the C2PA verifier shows the banner and four checks, and refuses a tampered copy', async ({ page, loginAs }) => {
  // DCS-OR-C2PA-006 // DCS-OR-C2PA-008
  test.setTimeout(420_000)
  page.setDefaultTimeout(30_000)

  const contractDid = await buildApprovedContract(page, loginAs)
  await signApprovedContractWithWallet(page, loginAs, contractDid)

  await gotoAs(page, loginAs, 'Contract Manager', `/ui/contracts/view/${contractDid}`)
  const verifier = page.getByTestId('pdf-verifier')

  await test.step('the stored PDF of the signed contract verifies as Active', async () => {
    await verifier.getByRole('button', { name: 'Verify stored PDF' }).click()
    await expect(verifier.getByTestId('pdf-verifier-banner')).toHaveText('Active', { timeout: 120_000 })
    await expect(verifier.getByTestId('pdf-verifier-match')).toHaveText('The PDF matches the contract.')
    const checks = verifier.getByTestId('pdf-verifier-check')
    await expect(checks).toHaveCount(4)
    await expect(checks.filter({ hasText: 'PDF signature' })).toContainText('Valid')
    await expect(checks.filter({ hasText: 'PDF signature' })).toContainText('eu-dss')
  })

  await test.step('a copy with changed page text is refused', async () => {
    const token = await page.evaluate(() => window.localStorage.getItem('access_token'))
    const exported = await page.request.get(`/api/pdf/export/contract/${encodeURIComponent(contractDid)}`, {
      headers: { Authorization: `Bearer ${token}` },
      timeout: 90_000,
    })
    expect(exported.ok()).toBeTruthy()
    const pdf = Buffer.from(await exported.body())
    // The page content streams are not compressed: change one letter of the
    // first text-showing operator, keeping the length so the xref stays valid.
    const at = pdf.indexOf(') Tj')
    expect(at).toBeGreaterThan(0)
    pdf[at - 1] = pdf[at - 1] === 0x58 ? 0x59 : 0x58

    await verifier.getByLabel('PDF copy to verify').setInputFiles({
      name: 'tampered.pdf',
      mimeType: 'application/pdf',
      buffer: pdf,
    })
    await expect(verifier.getByTestId('pdf-verifier-match')).toContainText('Mismatch', { timeout: 120_000 })
  })
})
