import { ApiError } from './errors'
import { clearSession, loadToken } from '../store/auth'

export function getApiBaseUrl(): string {
  return import.meta.env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8080'
}

export interface ApiOptions {
  method?: string
  body?: unknown
  token?: string | null
  idempotencyKey?: string
}

export async function apiRequest<T>(path: string, opts: ApiOptions = {}): Promise<{ data: T; requestId: string }> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.token) headers['Authorization'] = `Bearer ${opts.token}`
  if (opts.idempotencyKey) headers['Idempotency-Key'] = opts.idempotencyKey

  const res = await fetch(`${getApiBaseUrl()}${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    cache: path.startsWith('/api/v1/auth/') || path.startsWith('/api/v1/me') ? 'no-store' : 'default',
  })

  const payload = await res.json().catch(() => ({}))
  if (res.status < 200 || res.status >= 300) {
    // An expired protected request must not revoke a newer account's session.
    if (res.status === 401 && opts.token && opts.token === loadToken()) clearSession()
    const retry = Number(payload.retry_after ?? res.headers?.get('Retry-After'))
    throw new ApiError(
      res.status,
      payload.code ?? 'UNKNOWN',
      payload.message ?? `请求失败 (${res.status})`,
      payload.request_id,
      payload.field_errors,
      Number.isFinite(retry) && retry > 0 ? Math.ceil(retry) : undefined,
    )
  }
  return { data: payload.data as T, requestId: payload.request_id }
}

export { ApiError }
