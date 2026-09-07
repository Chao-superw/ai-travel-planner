import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import App from '../../App'
import { clearSession, loadToken, saveSession } from '../../store/auth'

const email = 'Traveler+trip@qq.com'
const user = { id: 'user1', username: '旅行者_user1', role: 'user' as const, email, email_verified: true }
const calls: { path: string; body: Record<string, string>; headers: Record<string, string> }[] = []
let currentUser = user
let sendResult: () => Promise<Response>
function response(data: unknown, status = 200): Response {
  return { status, headers: new Headers(), json: async () => status < 400 ? { data, request_id: 'test' } : data } as Response
}
function mount(path: string, from = '/trips') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[{ pathname: path, state: { from } }]}><App /></MemoryRouter></QueryClientProvider>)
  return client
}
async function fillRegistration() {
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('邮箱'), email)
  await u.type(screen.getByLabelText('密码', { exact: true }), 'password-123')
  await u.type(screen.getByLabelText('确认密码'), 'password-123')
  return u
}
beforeEach(() => {
  clearSession(); calls.length = 0; currentUser = user
  sendResult = async () => response({ challenge_id: 'challenge-1', expires_in: 600, retry_after: 60 })
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
    const path = new URL(url, window.location.origin).pathname
    calls.push({ path, body: init.body ? JSON.parse(String(init.body)) : {}, headers: init.headers as Record<string, string> })
    if (path.endsWith('/code')) return sendResult()
    if (path === '/api/v1/auth/register') return response(user, 201)
    if (path === '/api/v1/auth/login' || path === '/api/v1/auth/legacy/login') return response({ token: 'session-token', user: currentUser, expires_at: '2030-01-01T00:00:00Z' })
    if (path === '/api/v1/me/email') { currentUser = { ...currentUser, email, email_verified: true }; return response(currentUser) }
    if (path === '/api/v1/me') return response(currentUser)
    return response({ items: [], page: 1, page_size: 20, has_more: false })
  }))
})
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); vi.unstubAllEnvs() })

describe('email authentication pages', () => {
  it('only exposes email login and registration, with no legacy login entry', () => {
    mount('/login')
    expect(screen.getByRole('heading', { name: '邮箱登录' })).toBeInTheDocument()
    expect(screen.queryByText(/迁移旧账号|之前使用用户名/)).not.toBeInTheDocument()
    expect(screen.queryByLabelText('用户名')).not.toBeInTheDocument()
  })
  it('retains login errors and follows the server retry countdown', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => response({ code: 'AUTH_RATE_LIMITED', message: '操作频繁', retry_after: 90 }, 429)))
    mount('/login'); const u = userEvent.setup()
    await u.type(screen.getByLabelText('邮箱'), email)
    await u.type(screen.getByLabelText('密码'), 'password-123')
    await u.click(screen.getByRole('button', { name: '登录' }))
    expect(await screen.findByRole('button', { name: '90 秒后重试' })).toBeDisabled()
    expect(loadToken()).toBeNull()
    expect(fetch).toHaveBeenCalledTimes(1)
  })
  it('checks password byte length before registration', async () => {
    mount('/register'); const u = await fillRegistration()
    await u.clear(screen.getByLabelText('密码', { exact: true }))
    await u.type(screen.getByLabelText('密码', { exact: true }), '中'.repeat(25))
    await u.clear(screen.getByLabelText('确认密码'))
    await u.type(screen.getByLabelText('确认密码'), '中'.repeat(25))
    await u.click(screen.getByRole('button', { name: '获取验证码' })); await screen.findByText(/验证码已发送/)
    await u.type(screen.getByLabelText('邮箱验证码'), '001234')
    await u.click(screen.getByRole('button', { name: '注册' }))
    await screen.findByText(/密码长度须为 8–72 字节/)
    expect(calls.some(c => c.path === '/api/v1/auth/register')).toBe(false)
  })
  it.each(['', 'http://127.0.0.1:8080'])('registers only after receiving a challenge and returns to email login (API base: %j)', async (apiBase) => {
    vi.stubEnv('VITE_API_BASE_URL', apiBase)
    mount('/register')
    const u = await fillRegistration()
    expect(screen.getByRole('button', { name: '注册' })).toBeDisabled()
    await u.click(screen.getByRole('button', { name: '获取验证码' }))
    await screen.findByText(/验证码已发送/)
    expect(calls.filter(c => c.path === '/api/v1/auth/register')).toHaveLength(0)
    await u.type(screen.getByLabelText('邮箱验证码'), '001234')
    await u.click(screen.getByRole('button', { name: '注册' }))
    await screen.findByRole('heading', { name: '邮箱登录' })
    expect(calls.find(c => c.path === '/api/v1/auth/register')?.body).toEqual({ email, password: 'password-123', code: '001234', challenge_id: 'challenge-1' })
    expect(loadToken()).toBeNull()
    expect(screen.getByLabelText('邮箱')).toHaveValue(email)
    await u.type(screen.getByLabelText('密码', { exact: true }), 'password-123')
    await u.click(screen.getByRole('button', { name: '登录' }))
    await waitFor(() => expect(loadToken()).toBe('session-token'))
    expect(calls.find(c => c.path === '/api/v1/auth/login')?.body).toEqual({ email, password: 'password-123' })
    await screen.findByRole('heading', { name: '我的行程' })
  })
  it('clears the proof and code when the email changes', async () => {
    mount('/register'); const u = await fillRegistration()
    await u.click(screen.getByRole('button', { name: '获取验证码' })); await screen.findByText(/验证码已发送/)
    await u.type(screen.getByLabelText('邮箱验证码'), '001234')
    await u.clear(screen.getByLabelText('邮箱')); await u.type(screen.getByLabelText('邮箱'), 'different@qq.com')
    expect(screen.getByLabelText('邮箱验证码')).toHaveValue('')
    expect(screen.getByRole('button', { name: '注册' })).toBeDisabled()
  })
  it('discards an in-flight send result for an edited email', async () => {
    let resolve!: (r: Response) => void
    sendResult = () => new Promise(r => { resolve = r })
    mount('/register'); const u = await fillRegistration()
    await u.click(screen.getByRole('button', { name: '获取验证码' }))
    await u.clear(screen.getByLabelText('邮箱')); await u.type(screen.getByLabelText('邮箱'), 'different@qq.com')
    await act(async () => resolve(response({ challenge_id: 'old', expires_in: 600, retry_after: 60 })))
    expect(screen.queryByText(/验证码已发送/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '注册' })).toBeDisabled()
  })
  it('honors server send throttling without automatic retries', async () => {
    sendResult = async () => response({ code: 'AUTH_RATE_LIMITED', message: '操作频繁', retry_after: 120 }, 429)
    mount('/register'); const u = await fillRegistration()
    await u.click(screen.getByRole('button', { name: '获取验证码' }))
    expect(await screen.findByRole('button', { name: /120 秒后重发/ })).toBeDisabled()
    expect(calls.filter(c => c.path.endsWith('/code'))).toHaveLength(1)
  })
  it('keeps the old proof when a resend fails and prevents expired proof submission', async () => {
    vi.useFakeTimers()
    sendResult = async () => response({ challenge_id: 'old-proof', expires_in: 100, retry_after: 30 })
    mount('/register')
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: email } })
    fireEvent.change(screen.getByLabelText('密码', { exact: true }), { target: { value: 'password-123' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'password-123' } })
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '获取验证码' })))
    fireEvent.change(screen.getByLabelText('邮箱验证码'), { target: { value: '001234' } })
    await act(async () => vi.advanceTimersByTime(31_000))
    sendResult = async () => response({ code: 'MAIL_UNAVAILABLE', message: '暂时无法发送' }, 503)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '重新发送' })))
    expect(screen.getByRole('button', { name: '注册' })).not.toBeDisabled()
    await act(async () => vi.advanceTimersByTime(70_000))
    expect(screen.getByRole('button', { name: '注册' })).toBeDisabled()
    expect(screen.getByText(/验证码已过期/)).toBeInTheDocument()
  })
  it('clears the old typed code after a successful resend', async () => {
    vi.useFakeTimers()
    sendResult = async () => response({ challenge_id: 'first', expires_in: 600, retry_after: 30 })
    mount('/register')
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: email } })
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '获取验证码' })))
    fireEvent.change(screen.getByLabelText('邮箱验证码'), { target: { value: '001234' } })
    await act(async () => vi.advanceTimersByTime(31_000))
    sendResult = async () => response({ challenge_id: 'second', expires_in: 600, retry_after: 30 })
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '重新发送' })))
    expect(screen.getByLabelText('邮箱验证码')).toHaveValue('')
  })
  it('requires verified email before letting a legacy session access trips', async () => {
    currentUser = { ...user, username: 'legacy-user', email: '', email_verified: false }
    saveSession({ token: 'old-token', expires_at: '2030-01-01T00:00:00Z', user: currentUser })
    mount('/trips')
    await screen.findByRole('heading', { name: '绑定邮箱' })
    expect(calls.some(c => c.path === '/api/v1/trips')).toBe(false)
    const u = userEvent.setup()
    await u.type(screen.getByLabelText('邮箱'), email)
    await u.click(screen.getByRole('button', { name: '获取验证码' })); await screen.findByText(/验证码已发送/)
    await u.type(screen.getByLabelText('邮箱验证码'), '001234')
    await u.click(screen.getByRole('button', { name: '验证并绑定' }))
    await screen.findByRole('heading', { name: '我的行程' })
    expect(calls.find(c => c.path === '/api/v1/me/email')?.headers.Authorization).toBe('Bearer old-token')
    expect(calls.find(c => c.path === '/api/v1/me/email')?.body).toEqual({ email, code: '001234', challenge_id: 'challenge-1' })
    expect(loadToken()).toBe('old-token')
  })
})
