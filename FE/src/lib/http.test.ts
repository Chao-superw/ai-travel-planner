import { describe, it, expect, vi, beforeEach } from 'vitest'
import { apiRequest, ApiError } from './http'

function mockFetch(status: number, json: unknown) {
  return vi.fn().mockResolvedValue({
    status, ok: status >= 200 && status < 300,
    json: () => Promise.resolve(json),
  })
}

describe('apiRequest', () => {
  beforeEach(() => { vi.stubEnv('VITE_API_BASE_URL', 'http://127.0.0.1:8080') })

  it('unwraps data and request_id on success', async () => {
    global.fetch = mockFetch(200, { data: { token: 't1' }, request_id: 'r1' }) as any
    const res = await apiRequest<{ token: string }>('/api/v1/auth/login', { method: 'POST', body: { username: 'u', password: 'p' } })
    expect(res.data.token).toBe('t1')
    expect(res.requestId).toBe('r1')
  })

  it('throws ApiError with code/status/fieldErrors on 400', async () => {
    global.fetch = mockFetch(400, { code: 'INVALID', message: 'bad', request_id: 'r2', field_errors: { username: 'required' } }) as any
    await expect(apiRequest('/x', { method: 'POST', body: {} })).rejects.toMatchObject({
      code: 'INVALID', status: 400, requestId: 'r2', fieldErrors: { username: 'required' },
    } satisfies Partial<ApiError>)
  })

  it('adds Authorization and Idempotency-Key headers', async () => {
    const f = mockFetch(200, { data: {}, request_id: 'r3' })
    global.fetch = f as any
    await apiRequest('/x', { method: 'POST', body: {}, token: 'tok', idempotencyKey: 'key1' })
    const headers = (f.mock.calls[0][1] as RequestInit).headers as Record<string, string>
    expect(headers['Authorization']).toBe('Bearer tok')
    expect(headers['Idempotency-Key']).toBe('key1')
  })
  it('preserves retry_after and disables caching on auth requests', async () => {
    const f = mockFetch(429, { code: 'AUTH_RATE_LIMITED', message: 'later', retry_after: 60 })
    global.fetch = f as any
    await expect(apiRequest('/api/v1/auth/login', { method: 'POST' })).rejects.toMatchObject({ retryAfter: 60 })
    expect(f.mock.calls[0][1].cache).toBe('no-store')
  })
})

it('clears an expired current session on protected HTTP 401', async () => {
  localStorage.setItem('wandr.token', 'expired')
  global.fetch = mockFetch(401, { code: 'UNAUTHENTICATED', message: 'expired' }) as any
  await expect(apiRequest('/api/v1/trips', { token: 'expired' })).rejects.toMatchObject({ status: 401 })
  expect(localStorage.getItem('wandr.token')).toBeNull()
})
it('does not let a stale 401 clear a newer login', async () => {
  localStorage.setItem('wandr.token', 'new-token')
  global.fetch = mockFetch(401, { code: 'UNAUTHENTICATED', message: 'expired' }) as any
  await expect(apiRequest('/api/v1/trips', { token: 'old-token' })).rejects.toMatchObject({ status: 401 })
  expect(localStorage.getItem('wandr.token')).toBe('new-token')
})
