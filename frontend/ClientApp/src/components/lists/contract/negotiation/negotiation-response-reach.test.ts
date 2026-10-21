import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import NegotiationList from './NegotiationList.vue'
import type { Contract } from '@/models/contract/contract'
import type { UserRole } from '@/types/user-role'

/**
 * Accept/Reject on a change proposal. The gate has to admit every role
 * design/contract_workflow_engine.go respond() scopes, and the "have I already
 * decided" lookup has to key on this party's DCS instance did:web — decision
 * rows carry that, never the logged-in user's credential issuer.
 */

vi.mock('@/services/contract-workflow-service', () => ({
  contractWorkflowService: { respond: vi.fn(() => Promise.resolve({ id: 'n1' })) },
}))

const LOCAL_INSTANCE = 'did:web:local.example'
const USER_ISSUER = 'did:web:issuer.example:org'
const PEER_AUTHOR = 'did:web:peer.example'

const contract = (opts: {
  decision?: string | null
  createdBy?: string
  decisions?: { negotiator: string; decision: string | null }[]
}): Contract =>
  ({
    did: 'did:web:local.example:contract',
    state: 'NEGOTIATION',
    negotiations: [
      {
        id: 'n1',
        created_by: opts.createdBy ?? PEER_AUTHOR,
        created_at: '2026-07-31T00:00:00Z',
        state: 'OPEN',
        negotiation_decisions: opts.decisions ?? [{ negotiator: LOCAL_INSTANCE, decision: opts.decision ?? null }],
      },
    ],
  }) as unknown as Contract

function mountList(
  roles: UserRole[],
  opts: {
    decision?: string | null
    createdBy?: string
    decisions?: { negotiator: string; decision: string | null }[]
  } = {},
) {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().user = { issuer: USER_ISSUER, holder: 'user', roles }
  return mount(NegotiationList, {
    props: {
      contract: contract(opts),
      localInstanceDid: LOCAL_INSTANCE,
      participantId: USER_ISSUER,
    },
    global: { plugins: [pinia] },
  })
}

async function actionsFor(
  roles: UserRole[],
  opts: {
    decision?: string | null
    createdBy?: string
    decisions?: { negotiator: string; decision: string | null }[]
  } = {},
) {
  const wrapper = mountList(roles, opts)
  await wrapper
    .findAll('button')
    .find((button) => button.text().includes('Show'))
    ?.trigger('click')
  return wrapper.findAll('button')
}

describe('responding to a change proposal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it.each<UserRole>(['CONTRACT_CREATOR', 'CONTRACT_REVIEWER', 'CONTRACT_NEGOTIATOR', 'CONTRACT_MANAGER'])(
    'shows Accept and Reject for %s when this instance owes a decision',
    async (role) => {
      const buttons = await actionsFor([role])

      for (const label of ['Accept', 'Reject']) {
        const button = buttons.find((candidate) => candidate.text().includes(label))
        expect(button, label).toBeDefined()
        expect(button?.attributes('disabled'), label).toBeUndefined()
      }
    },
  )

  it('hides Accept and Reject for a role respond() refuses', async () => {
    const buttons = await actionsFor(['CONTRACT_OBSERVER'])

    expect(buttons.find((button) => button.text().includes('Accept'))).toBeUndefined()
    expect(buttons.find((button) => button.text().includes('Reject'))).toBeUndefined()
  })

  it('hides Accept and Reject once this instance has decided', async () => {
    const buttons = await actionsFor(['CONTRACT_NEGOTIATOR'], { decision: 'ACCEPTED' })

    expect(buttons.find((button) => button.text().includes('Accept'))).toBeUndefined()
    expect(buttons.find((button) => button.text().includes('Reject'))).toBeUndefined()
  })

  it('shows Withdraw on our own pending proposal and hides Accept/Reject', async () => {
    const buttons = await actionsFor(['CONTRACT_NEGOTIATOR'], {
      createdBy: USER_ISSUER,
      decisions: [
        { negotiator: LOCAL_INSTANCE, decision: null },
        { negotiator: 'did:web:peer.example', decision: null },
      ],
    })

    expect(buttons.find((button) => button.text().includes('Withdraw'))).toBeDefined()
    expect(buttons.find((button) => button.text().includes('Accept'))).toBeUndefined()
    expect(buttons.find((button) => button.text().includes('Reject'))).toBeUndefined()
  })

  it('hides Withdraw once a counterparty has accepted', async () => {
    const buttons = await actionsFor(['CONTRACT_NEGOTIATOR'], {
      createdBy: USER_ISSUER,
      decisions: [
        { negotiator: LOCAL_INSTANCE, decision: null },
        { negotiator: 'did:web:peer.example', decision: 'ACCEPTED' },
      ],
    })

    expect(buttons.find((button) => button.text().includes('Withdraw'))).toBeUndefined()
  })

  it('hides Withdraw once a counterparty has rejected', async () => {
    const buttons = await actionsFor(['CONTRACT_NEGOTIATOR'], {
      createdBy: USER_ISSUER,
      decisions: [
        { negotiator: LOCAL_INSTANCE, decision: null },
        { negotiator: 'did:web:peer.example', decision: 'REJECTED' },
      ],
    })

    expect(buttons.find((button) => button.text().includes('Withdraw'))).toBeUndefined()
  })

  it('disables Accept with a reason while negotiation sync is pending', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useAuthStore().user = { issuer: USER_ISSUER, holder: 'user', roles: ['CONTRACT_NEGOTIATOR'] }
    const wrapper = mount(NegotiationList, {
      props: {
        contract: {
          ...contract({}),
          pending_negotiation_sync: true,
        },
        localInstanceDid: LOCAL_INSTANCE,
        participantId: USER_ISSUER,
      },
      global: { plugins: [pinia] },
    })
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('Show'))
      ?.trigger('click')

    const accept = wrapper.find('[data-testid="accept-negotiation"]')
    expect(accept.exists()).toBe(true)
    expect(accept.attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="accept-sync-pending"]').text()).toContain(
      'Waiting for earlier negotiation messages',
    )
    expect(
      wrapper
        .findAll('button')
        .find((button) => button.text().includes('Reject'))
        ?.attributes('disabled'),
    ).toBeUndefined()
  })
})
