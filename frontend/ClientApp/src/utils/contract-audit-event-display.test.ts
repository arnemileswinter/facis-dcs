import { describe, expect, it } from 'vitest'
import { contractAuditEventTransition } from './contract-audit-event-display'

describe('contractAuditEventTransition', () => {
  it('formats previous → new from event_data', () => {
    expect(
      contractAuditEventTransition({
        previous_state: 'DRAFT',
        new_state: 'NEGOTIATION',
      }),
    ).toBe('Draft → Negotiation')
  })

  it('returns undefined when the API omits either state', () => {
    expect(contractAuditEventTransition({ previous_state: 'DRAFT' })).toBeUndefined()
    expect(contractAuditEventTransition({ new_state: 'APPROVED' })).toBeUndefined()
    expect(contractAuditEventTransition({ approved_by: 'did:example' })).toBeUndefined()
  })

  it('returns undefined for no-op transitions', () => {
    expect(
      contractAuditEventTransition({
        previous_state: 'NEGOTIATION',
        new_state: 'NEGOTIATION',
      }),
    ).toBeUndefined()
  })
})
