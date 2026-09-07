import { describe, it, expect, beforeEach } from 'vitest'
import { saveSession, loadToken, loadUser, clearSession } from './auth'

describe('auth store', () => {
  beforeEach(() => localStorage.clear())
  it('persists and reads session token+user', () => {
    saveSession({ token: 'abc', expires_at: '2026-10-01T00:00:00Z', user: { id: 'u1', username: 'zoe', role: 'user', email: 'zoe@qq.com', email_verified: true } })
    expect(loadToken()).toBe('abc')
    expect(loadUser()?.username).toBe('zoe')
  })
  it('clears session', () => {
    saveSession({ token: 'abc', expires_at: 'x', user: { id: 'u1', username: 'zoe', role: 'user', email: 'zoe@qq.com', email_verified: true } })
    clearSession()
    expect(loadToken()).toBeNull()
    expect(loadUser()).toBeNull()
  })
})
