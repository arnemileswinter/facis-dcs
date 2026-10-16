import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { useDcsDraftStore } from '@template-repository/store/dcsDraftStore'
import ArchiveDetailView from './ArchiveDetailView.vue'
import type { Contract } from '@/models/contract/contract'

const detail = vi.hoisted(() => vi.fn())
const route = reactive({ query: { did: 'did:test:archive', contract_version: '2' } })
vi.mock('vue-router', () => ({ useRoute: () => route, RouterLink: { template: '<a><slot /></a>' } }))
vi.mock('@/router/router', () => ({ ROUTES: { ARCHIVE: { DASHBOARD: 'archive' } } }))
vi.mock('@/services/archive-service', () => ({ archiveService: { detail } }))
function snapshot(): Contract {
  const doc = useDcsDraftStore().templateDocument
  return {
    did: 'did:test:archive',
    contract_version: 2,
    state: 'SIGNED',
    name: 'Frozen snapshot',
    created_by: 'creator',
    created_at: '',
    updated_at: '',
    contract_data: { ...doc, '@type': 'dcs:Contract' },
  }
}
function render() {
  return mount(ArchiveDetailView, { global: { stubs: { TemplatePreview: true } } })
}
describe('P05 exact immutable snapshot lifecycle', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    detail.mockReset()
    route.query = { did: 'did:test:archive', contract_version: '2' }
  })
  it('removes the previous snapshot while another version loads and ignores stale replies', async () => {
    detail.mockResolvedValueOnce(snapshot())
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('Frozen snapshot')
    let resolve!: (value: Contract) => void
    detail
      .mockReturnValueOnce(
        new Promise<Contract>((res) => {
          resolve = res
        }),
      )
      .mockRejectedValueOnce(new Error('Deleted'))
    route.query.contract_version = '3'
    await flushPromises()
    expect(wrapper.text()).not.toContain('Frozen snapshot')
    route.query.contract_version = '4'
    await flushPromises()
    resolve({ ...snapshot(), contract_version: 3 })
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Deleted')
    expect(wrapper.find('[data-testid="machine-readable-view"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it.each(['identity', 'missing content'])('rejects %s instead of showing a misleading document', async (problem) => {
    detail.mockResolvedValue(
      problem === 'identity' ? { ...snapshot(), contract_version: 99 } : { ...snapshot(), contract_data: undefined },
    )
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('incomplete or does not match')
    expect(wrapper.find('[data-testid="machine-readable-view"]').exists()).toBe(false)
  })
})
