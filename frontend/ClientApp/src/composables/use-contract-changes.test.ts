import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { useAuthTokenStore } from '@/stores/auth-token-store'
import { parseContractChanges, useContractChanges } from './useContractChanges'

const refresh = vi.hoisted(() => vi.fn())
vi.mock('@/services/authentication-service', () => ({ authenticationService: { refresh } }))

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

// A refused stream waits before the next attempt, and longer after each
// further refusal, instead of reconnecting at once.
it('backs off after a refused stream', async () => {
  vi.useFakeTimers()
  setActivePinia(createPinia())
  useAuthTokenStore().setTokens('Bearer', 'header.payload.signature')
  const fetchMock = vi.fn().mockResolvedValue(new Response('', { status: 401 }))
  vi.stubGlobal('fetch', fetchMock)
  refresh.mockResolvedValue(true)
  const wrapper = mount(defineComponent({ setup: () => useContractChanges(() => undefined), render: () => null }))
  await vi.advanceTimersByTimeAsync(100)
  expect(fetchMock).toHaveBeenCalledTimes(1)
  await vi.advanceTimersByTimeAsync(4_800)
  expect(fetchMock).toHaveBeenCalledTimes(1)
  await vi.advanceTimersByTimeAsync(300)
  expect(fetchMock).toHaveBeenCalledTimes(2)
  await vi.advanceTimersByTimeAsync(9_000)
  expect(fetchMock).toHaveBeenCalledTimes(2)
  await vi.advanceTimersByTimeAsync(1_200)
  expect(fetchMock).toHaveBeenCalledTimes(3)
  wrapper.unmount()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})
