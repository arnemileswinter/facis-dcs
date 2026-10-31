import { expect, it } from 'vitest'
import { parseContractChanges } from './useContractChanges'

// DCS-FR-CWE-24: the stream parser keeps an event split across chunks and
// drops the events that name no contract.
it('parses complete events and keeps the unfinished rest', () => {
  const first = parseContractChanges(
    'data: {"event_type":"CONNECTED","did":""}\n\ndata: {"did":"a","event_type":"SUBMIT_CONTRACT"}\n\ndata: {"did":"b",',
  )
  expect(first.changes).toEqual([{ did: 'a', event_type: 'SUBMIT_CONTRACT' }])

  const second = parseContractChanges(first.rest + '"event_type":"TERMINATE_CONTRACT"}\n\n')
  expect(second.changes).toEqual([{ did: 'b', event_type: 'TERMINATE_CONTRACT' }])
  expect(second.rest).toBe('')
})
