import { AxiosError, AxiosHeaders, type InternalAxiosRequestConfig } from 'axios'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, expect, it, vi } from 'vitest'

const refresh = vi.hoisted(() => vi.fn())
vi.mock('@/services/authentication-service', () => ({ authenticationService: { refresh } }))

import http from './http'

beforeEach(() => {
  setActivePinia(createPinia())
  refresh.mockReset()
})

// A request refused with 401 is retried once with a refreshed token; a second
// 401 is final, so a server that keeps refusing is not called in a loop.
it('retries a refused request once after a refresh', async () => {
  refresh.mockResolvedValue(true)
  let calls = 0
  http.defaults.adapter = (config: InternalAxiosRequestConfig) => {
    calls++
    const response = { data: { message: 'missing JWT token' }, status: 401, statusText: '', headers: {}, config }
    return Promise.reject(new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, null, response))
  }
  await expect(http.get('/contract/retrieve/x', { headers: new AxiosHeaders() })).rejects.toThrow('missing JWT token')
  expect(calls).toBe(2)
  expect(refresh).toHaveBeenCalledTimes(1)
})

// UC-IF-19: a lockout refusal carries Retry-After. No refresh is tried, since
// the lock refuses every credential, and the message says when to try again.
it('shows the lockout instead of refreshing', async () => {
  let calls = 0
  http.defaults.adapter = (config: InternalAxiosRequestConfig) => {
    calls++
    const response = {
      data: { message: 'Credential invalid or access revoked.' },
      status: 401,
      statusText: '',
      headers: { 'retry-after': '600' },
      config,
    }
    return Promise.reject(new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, null, response))
  }
  await expect(http.get('/contract/retrieve/x', { headers: new AxiosHeaders() })).rejects.toThrow(
    'Credential invalid or access revoked. Too many invalid credentials came from this address. Try again in 10 minutes.',
  )
  expect(calls).toBe(1)
  expect(refresh).not.toHaveBeenCalled()
})
