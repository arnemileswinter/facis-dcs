import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it } from 'vitest'
import { useDcsDraftStore } from '@template-repository/store/dcsDraftStore'
import { CONTRACT_PARTY_ROLE_OPTIONS } from './ontology-domain-fields'

describe('Semantic Hub contract party vocabulary', () => {
  it('derives the role picker from ContractPartyRoleCode and its value constraint', () => {
    expect(CONTRACT_PARTY_ROLE_OPTIONS).toEqual([
      { value: 'https://w3id.org/facis/dcs/taxonomy/v1#role-provider', label: 'Provider' },
      { value: 'https://w3id.org/facis/dcs/taxonomy/v1#role-customer', label: 'Customer' },
      { value: 'https://w3id.org/facis/dcs/taxonomy/v1#role-supplier', label: 'Supplier' },
      { value: 'https://w3id.org/facis/dcs/taxonomy/v1#role-client', label: 'Client' },
    ])
  })

  it('uses percent-encoded URI fragments while retaining labels and URI values', () => {
    setActivePinia(createPinia())
    const store = useDcsDraftStore()
    store.did = 'urn:uuid:template'

    expect(store.partyAnchors[0]).toEqual({
      id: `urn:uuid:template#party-${encodeURIComponent(CONTRACT_PARTY_ROLE_OPTIONS[0]!.value)}`,
      label: 'Provider',
    })
  })
})
