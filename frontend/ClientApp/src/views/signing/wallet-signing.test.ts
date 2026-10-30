import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import SigningCeremonyDialog from '@/components/signing/SigningCeremonyDialog.vue'
import { useAuthStore } from '@/stores/auth-store'
import SecureContractViewerView from './SecureContractViewerView.vue'

/**
 * DCS-FR-SM-06/-23: after the PID ceremony, the signer signs in the wallet. The
 * viewer publishes the OID4VP Document-Retrieval request, shows its deep link
 * and waits until the DCS accepted the signature the wallet posted back.
 */

const CONTRACT_DID = 'did:web:example.com:contract:1'
const WALLET_URI = 'openid4vp://?client_id=x509_san_dns%3Adcs&request_uri=https%3A%2F%2Fdcs%2Fobject'

const service = vi.hoisted(() => ({
  retrieveById: vi.fn(),
  getProvenanceChain: vi.fn(() => Promise.resolve({ chain: [] })),
  verifySignature: vi.fn(() => Promise.resolve({ match: true, findings: [] })),
  publishSignatureRequest: vi.fn(),
  startCeremony: vi.fn(),
  getCeremonyStatus: vi.fn(),
}))

vi.mock('@/services/signature-management-service', () => ({
  signatureManagementService: service,
  signingErrorMessage: (e: unknown) => String(e),
}))
vi.mock('@/services/did-service', () => ({ getLocalDIDFile: () => Promise.resolve({ id: 'did:web:org' }) }))
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRoute: () => ({ params: { did: CONTRACT_DID }, meta: {} }),
  useRouter: () => ({ push: vi.fn() }),
}))

const reveal = vi.fn()
const DialogStub = defineComponent({
  name: 'SigningCeremonyDialog',
  setup(_, { expose }) {
    expose({ reveal })
    return () => null
  },
})

beforeEach(() => {
  setActivePinia(createPinia())
  useAuthStore().user = { holder: 'did:example:holder', issuer: 'did:web:org', roles: ['CONTRACT_SIGNER'] }
  vi.clearAllMocks()
})

describe('Secure Contract Viewer wallet signing', () => {
  it('publishes the signing request and records the signature the wallet posted back', async () => {
    const contract = { did: CONTRACT_DID, name: 'Supply agreement', state: 'APPROVED', contract_data: null }
    service.retrieveById
      .mockResolvedValueOnce({ contract })
      .mockResolvedValueOnce({ contract, signature_envelope: { status: 'SIGNED', signer_did: 'did:jwk:signer' } })
    service.publishSignatureRequest.mockResolvedValue({ ceremony_id: 'ceremony-1', wallet_uri: WALLET_URI })
    reveal
      .mockResolvedValueOnce({ isCanceled: false, data: { signerDid: 'did:jwk:signer', ceremonyId: 'ceremony-1' } })
      .mockResolvedValueOnce({ isCanceled: false, data: { signerDid: 'did:jwk:signer', ceremonyId: 'ceremony-1' } })

    const wrapper = shallowMount(SecureContractViewerView, {
      global: { stubs: { SigningCeremonyDialog: DialogStub } },
    })
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((b) => b.text() === 'Verify')!
      .trigger('click')
    await flushPromises()
    const sign = wrapper.findAll('button').find((b) => b.text() === 'Sign with wallet')
    expect(sign, 'Sign with wallet button').toBeDefined()
    await sign!.trigger('click')
    await flushPromises()

    expect(service.publishSignatureRequest).toHaveBeenCalledWith('ceremony-1')
    expect(reveal).toHaveBeenLastCalledWith(
      expect.objectContaining({ signingRequest: { ceremonyId: 'ceremony-1', walletUri: WALLET_URI } }),
    )
    expect(service.retrieveById).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('Signed document accepted and recorded.')
  })
})

describe('Signing ceremony dialog for a published signing request', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    HTMLDialogElement.prototype.showModal = vi.fn()
    HTMLDialogElement.prototype.close = vi.fn()
  })
  afterEach(() => vi.useRealTimers())

  it('shows the deep link and resolves once the wallet signature is accepted', async () => {
    service.getCeremonyStatus
      .mockResolvedValueOnce({ ceremony_id: 'ceremony-1', status: 'verified' })
      .mockResolvedValueOnce({ ceremony_id: 'ceremony-1', status: 'verified', signed_at: '2026-10-31T10:00:00Z' })
    const wrapper = mount(SigningCeremonyDialog, { attachTo: document.body })
    const outcome = wrapper.vm.reveal({
      contractDid: CONTRACT_DID,
      fieldName: 'did:web:org',
      signingRequest: { ceremonyId: 'ceremony-1', walletUri: WALLET_URI },
    })
    await flushPromises()
    expect(service.startCeremony).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Waiting for the wallet signature')

    await vi.advanceTimersByTimeAsync(2500)
    expect(document.body.textContent).toContain('Waiting for the wallet signature')
    await vi.advanceTimersByTimeAsync(2500)

    await expect(outcome).resolves.toMatchObject({ isCanceled: false, data: { ceremonyId: 'ceremony-1' } })
    wrapper.unmount()
  })
})
