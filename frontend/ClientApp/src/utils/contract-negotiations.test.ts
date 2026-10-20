import { describe, expect, it } from 'vitest'
import {
  acceptedNegotiations,
  contractWithAcceptedProposals,
  hasUnresolvedProposals,
  isAcceptedForEditorBaseline,
  isFullyAcceptedNegotiation,
  proposalComparisonBaseline,
} from '@/utils/contract-negotiations'
import type { Contract } from '@/models/contract/contract'
import type { ContractNegotiation } from '@/models/contract/contract-negotiation'

const baseContract = (overrides: Partial<Contract> = {}): Contract =>
  ({
    did: 'did:web:example:contract',
    contract_version: 1,
    state: 'NEGOTIATION',
    created_by: 'did:web:example:org',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    name: 'Formal name',
    description: 'Formal description',
    start_date: '2026-01-01T00:00:00Z',
    ...overrides,
  })

const negotiation = (overrides: Partial<ContractNegotiation>): ContractNegotiation => ({
  id: 'n1',
  created_by: 'did:web:example:org',
  created_at: '2026-01-02T00:00:00Z',
  contract_version: 1,
  change_request: {},
  negotiation_decisions: [{ negotiator: 'did:web:peer', decision: 'ACCEPTED' }],
  ...overrides,
})

describe('hasUnresolvedProposals', () => {
  const local = 'did:web:local'
  const peer = 'did:web:peer'
  const author = 'did:web:example:org'

  it('blocks Submit while our proposal waits on the peer', () => {
    const contract = baseContract({
      negotiations: [
        negotiation({
          created_by: author,
          negotiation_decisions: [
            { negotiator: local, decision: undefined },
            { negotiator: peer, decision: undefined },
          ],
        }),
      ],
    })
    expect(hasUnresolvedProposals(contract, local, author)).toBe(true)
  })

  it('allows Submit once the peer has decided on our proposal', () => {
    const contract = baseContract({
      negotiations: [
        negotiation({
          created_by: author,
          negotiation_decisions: [
            { negotiator: local, decision: undefined },
            { negotiator: peer, decision: 'ACCEPTED' },
          ],
        }),
      ],
    })
    expect(hasUnresolvedProposals(contract, local, author)).toBe(false)
  })

  it('blocks Submit while we still owe Accept/Reject on a peer proposal', () => {
    const contract = baseContract({
      negotiations: [
        negotiation({
          created_by: 'did:web:other',
          negotiation_decisions: [
            { negotiator: local, decision: undefined },
            { negotiator: peer, decision: undefined },
          ],
        }),
      ],
    })
    expect(hasUnresolvedProposals(contract, local, author)).toBe(true)
  })
})

describe('accepted negotiation fold', () => {
  it('treats peer-accepted proposals as foldable even when the author row is still null', () => {
    const partial = negotiation({
      negotiation_decisions: [
        { negotiator: 'a', decision: 'ACCEPTED' },
        { negotiator: 'b', decision: undefined },
      ],
    })
    expect(isFullyAcceptedNegotiation(partial)).toBe(true)
    expect(isAcceptedForEditorBaseline(partial)).toBe(true)
    expect(
      isFullyAcceptedNegotiation(
        negotiation({
          negotiation_decisions: [{ negotiator: 'a', decision: 'ACCEPTED' }],
        }),
      ),
    ).toBe(true)
    expect(
      isAcceptedForEditorBaseline(
        negotiation({
          negotiation_decisions: [
            { negotiator: 'a', decision: 'ACCEPTED' },
            { negotiator: 'b', decision: 'REJECTED' },
          ],
        }),
      ),
    ).toBe(false)
  })

  it('orders accepted proposals oldest-first then by id', () => {
    const contract = baseContract({
      negotiations: [
        negotiation({ id: 'b', created_at: '2026-01-03T00:00:00Z', change_request: { name: 'Second' } }),
        negotiation({ id: 'a', created_at: '2026-01-02T00:00:00Z', change_request: { name: 'First' } }),
        negotiation({
          id: 'c',
          created_at: '2026-01-02T00:00:00Z',
          change_request: { name: 'Same-time' },
        }),
      ],
    })
    expect(acceptedNegotiations(contract).map((n) => n.id)).toEqual(['a', 'c', 'b'])
  })

  it('folds metadata last-accepted-wins and replaces contract_data whole-document', () => {
    const formalData = { '@id': 'formal' } as Contract['contract_data']
    const firstData = { '@id': 'first' } as Contract['contract_data']
    const secondData = { '@id': 'second' } as Contract['contract_data']
    const folded = contractWithAcceptedProposals(
      baseContract({
        contract_data: formalData,
        negotiations: [
          negotiation({
            id: 'a',
            created_at: '2026-01-02T00:00:00Z',
            change_request: { name: 'Accepted name', description: 'Keep me', contract_data: firstData },
          }),
          negotiation({
            id: 'b',
            created_at: '2026-01-03T00:00:00Z',
            change_request: { name: 'Latest name', contract_data: secondData },
          }),
        ],
      }),
    )
    expect(folded.name).toBe('Latest name')
    expect(folded.description).toBe('Keep me')
    expect(folded.contract_data).toEqual(secondData)
  })

  it('returns the formal contract when nothing is fully accepted', () => {
    const contract = baseContract({
      negotiations: [
        negotiation({
          negotiation_decisions: [{ negotiator: 'a', decision: undefined }],
          change_request: { name: 'Pending' },
        }),
      ],
    })
    expect(contractWithAcceptedProposals(contract)).toBe(contract)
  })

  it('Show baseline chains from prior accepted proposals, excluding the selected row', () => {
    const firstData = { '@id': 'first' } as Contract['contract_data']
    const secondData = { '@id': 'second' } as Contract['contract_data']
    const first = negotiation({
      id: 'a',
      created_at: '2026-01-02T00:00:00Z',
      change_request: { name: 'Accepted name', contract_data: firstData },
    })
    const second = negotiation({
      id: 'b',
      created_at: '2026-01-03T00:00:00Z',
      change_request: { name: 'Later proposal', contract_data: secondData },
      negotiation_decisions: [{ negotiator: 'a', decision: undefined }],
    })
    const contract = baseContract({
      negotiations: [first, second],
    })

    expect(proposalComparisonBaseline(contract, first).name).toBe('Formal name')
    expect(proposalComparisonBaseline(contract, second).name).toBe('Accepted name')
    expect(proposalComparisonBaseline(contract, second).contract_data).toEqual(firstData)
  })
})
