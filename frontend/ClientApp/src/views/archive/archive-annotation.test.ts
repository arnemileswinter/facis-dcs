import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import ArchiveDashboardView from './ArchiveDashboardView.vue'

const mocks = vi.hoisted(() => ({
  retrieve: vi.fn(),
  erasureStatus: vi.fn(),
  statistics: vi.fn(),
  annotate: vi.fn(),
  reveal: vi.fn(),
}))
vi.mock('@/services/archive-service', () => ({ archiveService: mocks }))
vi.mock('@/services/archive-statistics-service', () => ({ archiveStatisticsService: mocks }))
vi.mock('@/stores/auth-store', () => ({ useAuthStore: () => ({ user: { roles: ['ARCHIVE_MANAGER'] } }) }))
vi.mock('@/stores/error-store', () => ({ useErrorStore: () => ({ add: vi.fn() }) }))
vi.mock('@/router/router', () => ({ ROUTES: { ARCHIVE: { DETAIL: 'detail' }, AUDIT: { LIST: 'audit' } } }))
const entry = { did: 'did:test:archive', contract_version: 1, name: 'Example', state: 'ARCHIVED' }
const modal = defineComponent({
  setup(_, { expose }) {
    expose({ reveal: mocks.reveal })
  },
  template: '<div />',
})
function render() {
  return shallowMount(ArchiveDashboardView, {
    global: { stubs: { ConfirmationModal: modal, RouterLink: { template: '<a><slot /></a>' } } },
  })
}
beforeEach(() => {
  vi.resetAllMocks()
  mocks.statistics.mockResolvedValue(null)
  mocks.erasureStatus.mockResolvedValue({ did: entry.did, local_status: 'live', peers: [] })
})
it.each([false, true])(
  'prefills annotations when present=%s and writes only after the second confirmation',
  async (present) => {
    mocks.retrieve.mockResolvedValue([
      { ...entry, ...(present ? { archive_summary: 'Original summary', archive_tags: ['original', 'qa'] } : {}) },
    ])
    let finishTags!: (value: { isCanceled: boolean; data: string }) => void
    mocks.reveal.mockResolvedValueOnce({ isCanceled: false, data: '  Revised summary  ' }).mockReturnValueOnce(
      new Promise((resolve) => {
        finishTags = resolve
      }),
    )
    const wrapper = render()
    await flushPromises()
    await wrapper.get(`[data-testid="archive-annotate-${entry.did}"]`).trigger('click')
    await flushPromises()
    expect(mocks.reveal.mock.calls[0]![0]).toMatchObject({
      title: 'Edit summary – 1 of 2',
      confirmLabel: 'Next',
      editor: { label: 'Summary', initialValue: present ? 'Original summary' : undefined },
    })
    expect(mocks.reveal.mock.calls[1]![0]).toMatchObject({
      title: 'Tags – 2 of 2',
      confirmLabel: 'Save annotation',
      editor: { label: 'Tags', initialValue: present ? 'original, qa' : undefined },
    })
    expect(mocks.annotate).not.toHaveBeenCalled()
    finishTags({ isCanceled: false, data: ' supply, renewed ' })
    await flushPromises()
    expect(mocks.annotate).toHaveBeenCalledExactlyOnceWith(entry.did, 'Revised summary', ['supply', 'renewed'])
    wrapper.unmount()
  },
)
it.each([1, 2])('does not write when step %s is canceled', async (step) => {
  mocks.retrieve.mockResolvedValue([entry])
  if (step === 2) mocks.reveal.mockResolvedValueOnce({ isCanceled: false, data: 'Summary' })
  mocks.reveal.mockResolvedValueOnce({ isCanceled: true })
  const wrapper = render()
  await flushPromises()
  await wrapper.get(`[data-testid="archive-annotate-${entry.did}"]`).trigger('click')
  await flushPromises()
  expect(mocks.annotate).not.toHaveBeenCalled()
  wrapper.unmount()
})
it('preserves omission semantics for empty summary and tags', async () => {
  mocks.retrieve.mockResolvedValue([entry])
  mocks.reveal.mockResolvedValue({ isCanceled: false, data: '  ' })
  const wrapper = render()
  await flushPromises()
  await wrapper.get(`[data-testid="archive-annotate-${entry.did}"]`).trigger('click')
  await flushPromises()
  expect(mocks.annotate).toHaveBeenCalledExactlyOnceWith(entry.did, undefined, undefined)
  wrapper.unmount()
})
