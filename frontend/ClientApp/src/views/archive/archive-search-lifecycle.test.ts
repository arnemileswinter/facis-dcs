import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ArchiveDashboardView from './ArchiveDashboardView.vue'

const mocks = vi.hoisted(() => ({ retrieve: vi.fn(), search: vi.fn(), erasureStatus: vi.fn(), statistics: vi.fn() }))
vi.mock('@/services/archive-service', () => ({ archiveService: mocks }))
vi.mock('@/services/archive-statistics-service', () => ({ archiveStatisticsService: mocks }))
vi.mock('@/stores/auth-store', () => ({ useAuthStore: () => ({ user: { roles: ['ARCHIVE_MANAGER'] } }) }))
vi.mock('@/stores/error-store', () => ({ useErrorStore: () => ({ add: vi.fn() }) }))
vi.mock('@/router/router', () => ({ ROUTES: { ARCHIVE: { DETAIL: 'detail' }, AUDIT: { LIST: 'audit' } } }))
const entry = { did: 'did:test:archive', contract_version: 2, name: 'Current entry', state: 'SIGNED' }
const live = { did: entry.did, local_status: 'live', peers: [] }
const shredded = { ...live, local_status: 'shredded' }
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((yes) => {
    resolve = yes
  })
  return { promise, resolve }
}
function render() {
  return shallowMount(ArchiveDashboardView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
}
describe('P05 archive search and erasure lifecycle', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mocks.statistics.mockResolvedValue(null)
    mocks.retrieve.mockResolvedValue([entry])
    mocks.search.mockResolvedValue([entry])
    mocks.erasureStatus.mockResolvedValue(live)
  })
  it.each([entry.did, 'did:test:other'])(
    'reset clears expanded data and ignores old erasure results for %s',
    async (did) => {
      const wrapper = render()
      await flushPromises()
      await wrapper.get(`[data-testid="erasure-badge-${entry.did}"]`).trigger('click')
      expect(wrapper.find(`[data-testid="erasure-details-${entry.did}"]`).exists()).toBe(true)
      const old = deferred<typeof live>()
      mocks.erasureStatus.mockReturnValueOnce(old.promise).mockResolvedValueOnce({ ...shredded, did })
      await wrapper.get('form').trigger('submit')
      await flushPromises()
      expect(wrapper.find(`[data-testid="erasure-details-${entry.did}"]`).exists()).toBe(false)
      expect(wrapper.find(`[data-testid="erasure-badge-${entry.did}"]`).exists()).toBe(false)
      mocks.retrieve.mockResolvedValueOnce([{ ...entry, did }])
      const reset = wrapper.findAll('button').find((button) => button.text() === 'Reset search')
      await reset?.trigger('click')
      await flushPromises()
      expect(wrapper.get(`[data-testid="erasure-badge-${did}"]`).text()).toBe('Keys destroyed')
      old.resolve(live)
      await flushPromises()
      expect(wrapper.get(`[data-testid="erasure-badge-${did}"]`).text()).toBe('Keys destroyed')
      wrapper.unmount()
    },
  )
  it('reports failed search, retries, and ignores old entry replies after a reset', async () => {
    mocks.retrieve.mockRejectedValueOnce(new Error('Archive unavailable'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="archive-entries-error"]').text()).toContain('Archive unavailable')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-testid="archive-entries-error"]').exists()).toBe(false)
    const old = deferred<(typeof entry)[]>()
    mocks.retrieve.mockReturnValueOnce(old.promise).mockResolvedValueOnce([{ ...entry, name: 'Newest entry' }])
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((button) => button.text() === 'Reset search')
      ?.trigger('click')
    await flushPromises()
    old.resolve([{ ...entry, name: 'Stale entry' }])
    await flushPromises()
    expect(wrapper.text()).toContain('Newest entry')
    expect(wrapper.text()).not.toContain('Stale entry')
    expect(wrapper.find('[data-testid="archive-entries-error"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
