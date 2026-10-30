import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import ArchiveDashboardView from './ArchiveDashboardView.vue'

/**
 * The Archive Manager stores contracts and evidence (DCS-IR-CSA-02) and
 * terminates archived contracts (DCS-IR-CSA-03) from the dashboard.
 */

const archive = vi.hoisted(() => ({
  retrieve: vi.fn(),
  erasureStatus: vi.fn(),
  compliance: vi.fn(),
  storeContract: vi.fn(),
  storeEvidence: vi.fn(),
  evidence: vi.fn(),
  search: vi.fn(),
  annotate: vi.fn(),
  delete: vi.fn(),
}))
const workflow = vi.hoisted(() => ({ terminate: vi.fn() }))

vi.mock('@/services/archive-service', () => ({ archiveService: archive }))
vi.mock('@/services/contract-workflow-service', () => ({ contractWorkflowService: workflow }))
vi.mock('@/services/archive-statistics-service', () => ({
  archiveStatisticsService: {
    statistics: vi.fn().mockResolvedValue({
      archived_total: 1,
      archived_contracts: 1,
      deleted_total: 0,
      storage_bytes: 10,
      compliant_total: 0,
      flagged_total: 1,
      recent_actions: [],
      expiring_contracts: [],
    }),
  },
}))

const signed = {
  did: 'did:web:example.com:contract:1',
  contract_version: 1,
  state: 'SIGNED',
  created_by: 'did:web:example.com',
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-02T00:00:00Z',
}

async function mountDashboard() {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().user = { issuer: 'did:web:example.com', holder: 'user', roles: ['ARCHIVE_MANAGER'] }
  const wrapper = mount(ArchiveDashboardView, {
    global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } } },
  })
  await flushPromises()
  return wrapper
}

describe('archive dashboard actions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    archive.retrieve.mockResolvedValue([signed])
    archive.erasureStatus.mockResolvedValue({ did: signed.did, local_status: 'live', peers: [] })
    archive.compliance.mockResolvedValue([
      {
        did: signed.did,
        contract_version: 1,
        flagged: true,
        checks: [{ rule: 'evidence_completeness', passed: false, reason: 'no TSA receipt' }],
      },
    ])
  })

  it('archives a contract by DID', async () => {
    archive.storeContract.mockResolvedValue({
      did: signed.did,
      kind: 'contract',
      contract_version: 1,
      content_hash: 'h',
    })
    const wrapper = await mountDashboard()
    await wrapper.get('[data-testid="archive-store-did"]').setValue(signed.did)
    await wrapper.get('[data-testid="archive-store-contract"]').trigger('click')
    await flushPromises()
    expect(archive.storeContract).toHaveBeenCalledWith(signed.did)
  })

  it('flags a non-compliant entry', async () => {
    const wrapper = await mountDashboard()
    expect(wrapper.get(`[data-testid="archive-compliance-${signed.did}"]`).text()).toContain('Flagged')
  })

  it('offers termination only for a contract that can still end', async () => {
    archive.retrieve.mockResolvedValue([
      signed,
      { ...signed, did: 'did:web:example.com:contract:2', state: 'TERMINATED' },
    ])
    const wrapper = await mountDashboard()
    expect(wrapper.find(`[data-testid="archive-terminate-${signed.did}"]`).exists()).toBe(true)
    expect(wrapper.find('[data-testid="archive-terminate-did:web:example.com:contract:2"]').exists()).toBe(false)
  })
})
