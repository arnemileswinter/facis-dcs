import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ContractState } from '@/types/contract-state'
import ContractHistoryDiffView from './ContractHistoryDiffView.vue'

/**
 * Each version in the history picker names the actor whose save replaced it,
 * so an edit is attributed to the editor and not to the contract's creator.
 */

vi.mock('@/services/contract-workflow-service', () => ({
  contractWorkflowService: {
    retrieveHistoryByDid: () =>
      Promise.resolve([
        {
          did: 'did:web:example.com:contract',
          contract_version: 1,
          state: 'DRAFT',
          created_by: 'Contract Creator',
          created_at: '2026-10-27T08:00:00Z',
          updated_at: '2026-10-27T08:00:00Z',
          modified_by: 'Editing User',
          modified_at: '2026-10-28T09:30:00Z',
        },
        {
          did: 'did:web:example.com:contract',
          contract_version: 1,
          state: 'DRAFT',
          created_by: 'Contract Creator',
          created_at: '2026-10-27T08:00:00Z',
          updated_at: '2026-10-26T08:00:00Z',
        },
      ]),
  },
}))

describe('ContractHistoryDiffView', () => {
  it('labels each version with the actor of the save that replaced it', async () => {
    const wrapper = shallowMount(ContractHistoryDiffView, {
      props: { contractDid: 'did:web:example.com:contract', contractState: ContractState.draft },
    })
    await flushPromises()

    const labels = wrapper.findAll('#diff-left-pick option').map((option) => option.text())
    expect(labels).toHaveLength(2)
    expect(labels[0]).toContain('replaced by Editing User at')
    expect(labels[0]).not.toContain('Contract Creator')
    expect(labels[1]).not.toContain('replaced by')
  })
})
