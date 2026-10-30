import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import { type ContractTemplateState, TemplateState } from '@/types/contract-template-state'
import TemplateManagerActions from './TemplateManagerActions.vue'
import type { PartialContractTemplate } from '@/models/contract-template/contract-template'

vi.mock('@/router/router', () => ({ ROUTES: { TEMPLATES: { LIST: 'templates' } } }))

/** DCS-FR-TR-18: the Template Manager can delete a deprecated template. */
function archiveButton(state: ContractTemplateState) {
  setActivePinia(createPinia())
  useAuthStore().user = { issuer: 'did:web:example.com:org', holder: 'user', roles: ['TEMPLATE_MANAGER'] }
  const template: PartialContractTemplate = {
    did: 'did:web:example.com:template',
    name: 'Supply template',
    state,
    template_type: 'CONTRACT_TEMPLATE',
    party_type: 'SINGLE_PARTY',
    version: 1,
    created_by: 'did:web:example.com:creator',
    created_at: '2026-07-31T00:00:00Z',
    updated_at: '2026-07-31T00:00:00Z',
  }
  const wrapper = mount(TemplateManagerActions, { props: { template } })
  return wrapper.findAll('button').find((button) => ['Archive', 'Delete'].includes(button.text()))
}

describe('template manager archive action', () => {
  it('offers Delete for a deprecated template', () => {
    expect(archiveButton(TemplateState.deprecated)?.text()).toBe('Delete')
  })

  it('offers Archive for a registered template', () => {
    expect(archiveButton(TemplateState.registered)?.text()).toBe('Archive')
  })

  it('offers no action for a deleted template', () => {
    expect(archiveButton(TemplateState.deleted)).toBeUndefined()
  })
})
