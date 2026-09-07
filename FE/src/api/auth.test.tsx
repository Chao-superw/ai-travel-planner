import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useLogin, useBindEmail, useLogout } from './auth'
import { loadToken, loadUser, saveSession } from '../store/auth'

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('useLogin', () => {
  beforeEach(() => { localStorage.clear(); vi.stubEnv('VITE_API_BASE_URL', 'http://127.0.0.1:8080') })
  it('saves token to store on success', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 200, ok: true,
      json: () => Promise.resolve({ data: { token: 'tk', expires_at: 'x', user: { id: 'u', username: 'zoe', role: 'user', email: 'zoe@qq.com', email_verified: true } }, request_id: 'r' }),
    }) as any
    const { result } = renderHook(() => useLogin(), { wrapper })
    result.current.mutate({ email: 'zoe@qq.com', password: 'password1' })
    await waitFor(() => expect(loadToken()).toBe('tk'))
    const init = vi.mocked(fetch).mock.calls[0][1]
    expect(JSON.parse(init?.body as string)).toEqual({ email: 'zoe@qq.com', password: 'password1' })
  })
  it('does not reuse another account query cache after login', async () => {
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const oldUser = { id: 'old', username: 'old', role: 'user' as const, email: 'old@qq.com', email_verified: true }
    saveSession({ token: 'old-token', expires_at: 'x', user: oldUser })
    qc.setQueryData(['me', 'old-token'], oldUser)
    qc.setQueryData(['trips'], { items: [{ id: 'private-old-trip' }] })
    const newUser = { ...oldUser, id: 'new', email: 'new@qq.com' }
    global.fetch = vi.fn().mockResolvedValue({ status: 200, json: async () => ({ data: { token: 'new-token', user: newUser, expires_at: 'x' } }) })
    const { result } = renderHook(() => useLogin(), { wrapper: ({ children }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider> })
    result.current.mutate({ email: 'new@qq.com', password: 'password-123' })
    await waitFor(() => expect(loadToken()).toBe('new-token'))
    expect(loadUser()?.id).toBe('new')
    expect(qc.getQueryData(['trips'])).toBeUndefined()
    expect(qc.getQueryData(['me', 'old-token'])).toBeUndefined()
    expect(qc.getQueryData(['me', 'new-token'])).toEqual(newUser)
  })
})


it('binding cancels an in-flight profile read before publishing verified state', async () => {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const unverified = { id: 'u', username: 'old', role: 'user' as const, email: '', email_verified: false }
  const verified = { ...unverified, email: 'new@qq.com', email_verified: true }
  saveSession({ token: 'token', expires_at: 'x', user: unverified })
  let resolve!: (user: typeof unverified) => void
  const stale = qc.fetchQuery({ queryKey: ['me', 'token'], queryFn: () => new Promise<typeof unverified>(r => { resolve = r }) }).catch(() => undefined)
  global.fetch = vi.fn().mockResolvedValue({ status: 200, json: async () => ({ data: verified }) })
  const { result } = renderHook(() => useBindEmail(), { wrapper: ({ children }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider> })
  result.current.mutate({ email: 'new@qq.com', code: '001234', challenge_id: 'c' })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  resolve(unverified)
  await stale
  expect(qc.getQueryData(['me', 'token'])).toEqual(verified)
})

it('a delayed logout cannot clear a newly signed-in account', async () => {
  const oldUser = { id: 'old', username: 'old', role: 'user' as const, email: 'old@qq.com', email_verified: true }
  saveSession({ token: 'old-token', expires_at: 'x', user: oldUser })
  let resolve!: (result: unknown) => void
  global.fetch = vi.fn().mockImplementation(() => new Promise(r => { resolve = r }))
  const { result } = renderHook(() => useLogout(), { wrapper })
  result.current.mutate()
  await waitFor(() => expect(fetch).toHaveBeenCalled())
  saveSession({ token: 'new-token', expires_at: 'x', user: { ...oldUser, id: 'new' } })
  resolve({ status: 200, json: async () => ({ data: { logged_out: true } }) })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  expect(loadToken()).toBe('new-token')
})
