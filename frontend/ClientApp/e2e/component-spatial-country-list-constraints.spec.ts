import type { Locator, Page } from '@playwright/test'
import { expect, test } from './dcs-test'
const COUNTRY_OPTIONS = ['Germany (DEU)', 'Austria (AUT)', 'Switzerland (CHE)', 'United States (USA)'] as const

async function openSpatialConstraint(page: Page, loginAs: (role: 'Template Creator') => Promise<void>) {
  await loginAs('Template Creator')
  await page.goto('/ui/templates/new')
  await page.getByRole('button', { name: /Component/ }).click()
  await page.getByRole('tab', { name: /Clauses/ }).click()

  const editor = page.getByTestId('split-clause-editor')
  await expect(editor.getByText('Machine-readable meaning (ODRL)')).toBeVisible()
  await editor.getByRole('button', { name: '+ constraint' }).click()

  const row = editor.locator('.flex.flex-wrap.items-center.gap-1').last()
  await row.locator('select').nth(0).selectOption({ label: 'access region (spatial)' })
  return row
}
async function expectCountryOptions(valueSelect: Locator, includesPlaceholder = false) {
  const options = valueSelect.getByRole('option')
  await expect(options).toHaveCount(COUNTRY_OPTIONS.length + Number(includesPlaceholder))
  if (includesPlaceholder) await expect(options.first()).toHaveText('choose value')
  for (const label of COUNTRY_OPTIONS)
    await expect(valueSelect.getByRole('option', { name: label, exact: true })).toHaveCount(1)
}

test('a Template Creator authors spatial country list constraints in a Component', async ({ page, loginAs }) => {
  page.setDefaultTimeout(15_000)

  let constraintRow: Locator
  await test.step('@REQ-component-spatial-country-list-constraints-AC1 opens the Component clause constraint editor', async () => {
    constraintRow = await openSpatialConstraint(page, loginAs)
  })

  await test.step('@REQ-component-spatial-country-list-constraints-AC2 spatial offers the ontology-backed countries and alpha-3 codes', async () => {
    await expectCountryOptions(constraintRow.locator('select').nth(3), true)
  })

  await test.step('@REQ-component-spatial-country-list-constraints-AC3 must equal uses a single-select country value', async () => {
    await constraintRow.locator('select').nth(1).selectOption({ label: 'must equal' })
    const valueSelect = constraintRow.locator('select').nth(3)
    await expect(valueSelect).not.toHaveAttribute('multiple', '')
    await valueSelect.selectOption({ label: 'Germany (DEU)' })
    await expect(valueSelect.locator('option:checked')).toHaveText('Germany (DEU)')
  })

  const assertCountryMultiSelect = async (operator: string, selectedLabels: [string, string]) => {
    await constraintRow.locator('select').nth(1).selectOption({ label: operator })
    const valueSelect = constraintRow.locator('select').nth(3)
    await expect(valueSelect).toHaveAttribute('multiple', '')
    await expectCountryOptions(valueSelect)
    await valueSelect.selectOption(selectedLabels.map((label) => ({ label })))
    await expect(valueSelect.locator('option:checked')).toHaveText(selectedLabels)
  }

  await test.step('@REQ-component-spatial-country-list-constraints-AC4 must be one of selects multiple countries', async () => {
    await assertCountryMultiSelect('must be one of', ['Germany (DEU)', 'Austria (AUT)'])
  })

  await test.step('@REQ-component-spatial-country-list-constraints-AC5 must not be one of selects multiple countries', async () => {
    await assertCountryMultiSelect('must not be one of', ['Switzerland (CHE)', 'United States (USA)'])
  })

  await test.step('@REQ-component-spatial-country-list-constraints-AC6 must be all of selects multiple countries', async () => {
    await assertCountryMultiSelect('must be all of', ['Germany (DEU)', 'Switzerland (CHE)'])
  })
})
