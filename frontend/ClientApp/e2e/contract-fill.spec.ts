import { expect, seededFixtures, test } from './dcs-test'

/**
 * The contract fill flow's semantic boundary, through the real edit UI: a
 * value typed into a placeholder input is carried inline on the typed
 * dcs:Placeholder it fills (dcs:value on the placeholder an ODRL constraint
 * names as its odrl:leftOperand), with the editor-internal (blockId,
 * placeholder @id) tuple never leaking as a separate values array, and the
 * unsigned contract's policy set staying an odrl:Offer.
 */

interface Placeholder {
  '@id'?: string
  'dcs:label'?: string
  'dcs:value'?: unknown
}

test('filling a placeholder writes the value inline on the placeholder of an odrl:Offer', async ({
  page,
  loginAs,
}) => {
  const { contractDid } = seededFixtures()
  await loginAs('Contract Creator')
  await page.goto(`/ui/contracts/edit/${contractDid}`)

  // The fill inputs live under the Contract Content tab; the seeded fixture
  // carries one placeholder for the coverage value, and the input is labeled
  // with the placeholder's dcs:label.
  await page
    .getByRole('tab', { name: /content/i })
    .or(page.getByText('Contract Content', { exact: true }))
    .first()
    .click()
  const input = page.getByRole('textbox', { name: /coverage/i }).first()
  await expect(input).toBeVisible()
  await input.fill('97')

  const updateRequest = page.waitForRequest((r) => r.url().includes('/contract/update') && r.method() === 'PUT')
  await page.getByRole('button', { name: 'Update', exact: true }).click()
  const payload = (await updateRequest).postDataJSON() as {
    contract_data: {
      'dcs:contractData'?: Placeholder[]
      'dcs:policies': { '@type': string }
    }
  }

  const placeholders = payload.contract_data['dcs:contractData'] ?? []
  const coverage = placeholders.find((ph) => /coverage/i.test(ph['dcs:label'] ?? ''))
  expect(coverage, 'the coverage placeholder is in the document').toBeTruthy()
  // The value lives inline on the placeholder, not in a separate values array;
  // the decimal-typed placeholder yields a NUMBER, not a string.
  expect(coverage!['dcs:value'], 'the filled value is carried inline on the placeholder').toBe(97)

  expect(payload.contract_data['dcs:policies']['@type'], 'unsigned contracts stay an odrl:Offer').toBe('odrl:Offer')
})
