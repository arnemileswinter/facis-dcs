import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import AuditView from './AuditView.vue'
import type { AuditFinding } from '@/models/responses/auditing-response'

const audit = vi.hoisted(() => vi.fn())
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('@/services/archive-service', () => ({ archiveService: { erasureStatus: vi.fn().mockResolvedValue(null) } }))
vi.mock('@/services/auditing-service', () => ({ auditingService: { audit } }))
const finding: AuditFinding = {
  id: 'result-a',
  category: 'compliance_check',
  title: 'Contract A checked',
  status: 'PASSED',
  did: 'did:test:a',
  created_at: '2026-10-26T00:00:00Z',
}
function render() {
  return mount(AuditView, {
    global: {
      mocks: { $route: { meta: { name: 'Audit' } } },
      stubs: { ActionHistoryPanel: true, AccessAttempts: true },
    },
  })
}
function button(wrapper: ReturnType<typeof render>, text: string) {
  return wrapper.findAll('button').find((button) => button.text() === text)!
}
function expectNoResult(wrapper: ReturnType<typeof render>) {
  expect(wrapper.text()).not.toContain('Contract A checked')
  expect(wrapper.text()).not.toContain('Audit passed.')
  expect(wrapper.text()).toContain('Select a scope and execute an audit.')
  expect(wrapper.findAll('tbody tr[tabindex]')).toHaveLength(0)
  expect(wrapper.findAll('.dropdown input[type="checkbox"]')).toHaveLength(0)
  expect(wrapper.findAll('.dropdown .badge').map((badge) => badge.text())).toEqual(['0/0', '0/0', '0/0', '0/0'])
  for (const format of ['JSON', 'CSV', 'PDF']) expect(button(wrapper, format).attributes('disabled')).toBeDefined()
}

describe('P06 audit execution context', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useAuthStore().user = { issuer: 'org', holder: 'user', roles: ['AUDITOR'] }
    audit.mockReset()
  })
  it('clears findings, filter values and opened details on DID or scope changes', async () => {
    audit.mockResolvedValue([finding])
    const wrapper = render()
    await wrapper.get('#input-did').setValue('did:test:a')
    await wrapper.get('#input-justification').setValue('Review context')
    await button(wrapper, 'Execute Audit').trigger('click')
    await flushPromises()
    await wrapper.get('tbody tr[tabindex]').trigger('keydown', { key: 'Enter' })
    expect(wrapper.get('aside').text()).toContain('Contract A checked')
    await wrapper.get('#input-did').setValue('did:test:b')
    await flushPromises()
    expectNoResult(wrapper)
    await wrapper.get('#input-did').setValue('did:test:a')
    await flushPromises()
    expect(wrapper.findAll('tbody tr[tabindex]')).toHaveLength(1)
    expect(wrapper.get('aside').text()).not.toContain('Contract A checked')
    await wrapper.get('#select-scope').setValue('templates')
    await flushPromises()
    expectNoResult(wrapper)
    wrapper.unmount()
  })
  it.each(['did', 'scope'])('does not attribute a late response to a different %s', async (context) => {
    let resolve!: (findings: AuditFinding[]) => void
    audit.mockReturnValue(new Promise<AuditFinding[]>((res) => (resolve = res)))
    const wrapper = render()
    await wrapper.get('#input-did').setValue('did:test:a')
    await wrapper.get('#input-justification').setValue('Review context')
    await button(wrapper, 'Execute Audit').trigger('click')
    expect(wrapper.get('#input-did').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#select-scope').attributes('disabled')).toBeDefined()
    // Simulate an external context change while the disabled form awaits its response.
    const state = wrapper.vm as unknown as { didFilter: string; selectedScope: string }
    if (context === 'did') state.didFilter = 'did:test:b'
    else state.selectedScope = 'templates'
    resolve([finding])
    await flushPromises()
    expectNoResult(wrapper)
    wrapper.unmount()
  })
})
