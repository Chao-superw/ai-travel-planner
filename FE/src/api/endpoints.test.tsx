import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useReplan, useManualEdit, useTripVersions, useTripVersion } from './trips'
import { useSearchPlaces } from './places'
import { useMe } from './me'
import { useCreatePlace, useUpdatePlace } from './admin'
import type { ReplanInput, ManualEditInput, CreatePlaceInput, UpdatePlaceInput } from '../types/api'

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

function mockOk(json: unknown) {
  const f = vi.fn().mockResolvedValue({ status: 200, ok: true, json: () => Promise.resolve(json) })
  global.fetch = f as any
  return f
}
function lastCall(f: ReturnType<typeof vi.fn>) {
  const [url, init] = f.mock.calls[f.mock.calls.length - 1] as [string, RequestInit]
  return { url, init, headers: (init.headers ?? {}) as Record<string, string>, body: init.body ? JSON.parse(init.body as string) : undefined }
}

describe('trip iteration + versions hooks', () => {
  beforeEach(() => { localStorage.clear(); localStorage.setItem('wandr.token', 'tok'); vi.stubEnv('VITE_API_BASE_URL', 'http://127.0.0.1:8080') })

  it('useReplan POSTs scope+instruction with idempotency key', async () => {
    const f = mockOk({ data: { id: 'j1', status_url: '/api/v1/planning-jobs/j1' }, request_id: 'r' })
    const input: ReplanInput = {
      expected_version: 3,
      scope: { date: '2026-10-01', start_minute: 0, end_minute: 1440, editable_item_ids: ['a1'], locked_item_ids: [] },
      instruction: '把下午换成博物馆',
    }
    const { result } = renderHook(() => useReplan('t1'), { wrapper })
    result.current.mutate({ input, idempotencyKey: 'k1' })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const c = lastCall(f)
    expect(c.url).toBe('http://127.0.0.1:8080/api/v1/trips/t1/replanning-jobs')
    expect(c.init.method).toBe('POST')
    expect(c.headers['Idempotency-Key']).toBe('k1')
    expect(c.headers['Authorization']).toBe('Bearer tok')
    expect(c.body).toEqual(input)
  })

  it('useManualEdit PATCHes full plan', async () => {
    const f = mockOk({ data: { id: 'j2', status_url: '/api/v1/planning-jobs/j2' }, request_id: 'r' })
    const input: ManualEditInput = {
      expected_version: 2,
      plan: { title: '新标题', summary: '概述', activities: [{ id: 'a1', date: '2026-10-01', start_minute: 540, end_minute: 660, kind: 'sightseeing', place_id: 'p1', title: '西湖', reason: '' }] },
    }
    const { result } = renderHook(() => useManualEdit('t1'), { wrapper })
    result.current.mutate({ input, idempotencyKey: 'k2' })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const c = lastCall(f)
    expect(c.url).toBe('http://127.0.0.1:8080/api/v1/trips/t1')
    expect(c.init.method).toBe('PATCH')
    expect(c.headers['Idempotency-Key']).toBe('k2')
    expect(c.body).toEqual(input)
  })

  it('useTripVersions GETs version list', async () => {
    const f = mockOk({ data: { items: [], page: 1, page_size: 20, has_more: false }, request_id: 'r' })
    const { result } = renderHook(() => useTripVersions('t1', 2, 10), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(lastCall(f).url).toBe('http://127.0.0.1:8080/api/v1/trips/t1/versions?page=2&page_size=10')
  })

  it('useTripVersion is disabled when version is null', async () => {
    const f = mockOk({ data: {}, request_id: 'r' })
    const { result } = renderHook(() => useTripVersion('t1', null), { wrapper })
    await waitFor(() => expect(result.current.fetchStatus).toBe('idle'))
    expect(f).not.toHaveBeenCalled()
  })

  it('useTripVersion GETs a specific version when set', async () => {
    const f = mockOk({ data: { id: 't1', version: 2 }, request_id: 'r' })
    const { result } = renderHook(() => useTripVersion('t1', 2), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(lastCall(f).url).toBe('http://127.0.0.1:8080/api/v1/trips/t1/versions/2')
  })
})

describe('places + me + admin hooks', () => {
  beforeEach(() => { localStorage.clear(); localStorage.setItem('wandr.token', 'tok'); vi.stubEnv('VITE_API_BASE_URL', 'http://127.0.0.1:8080') })

  it('useSearchPlaces requires city and builds query', async () => {
    const f = mockOk({ data: { items: [], page: 1, page_size: 20, has_more: false }, request_id: 'r' })
    const { result } = renderHook(() => useSearchPlaces({ city: '杭州市', keyword: '西湖', page: 1, pageSize: 20 }), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const url = new URL(lastCall(f).url)
    expect(url.pathname).toBe('/api/v1/places')
    expect(url.searchParams.get('city')).toBe('杭州市')
    expect(url.searchParams.get('keyword')).toBe('西湖')
    expect(url.searchParams.get('page_size')).toBe('20')
  })

  it('useSearchPlaces is disabled without city', async () => {
    const f = mockOk({ data: {}, request_id: 'r' })
    const { result } = renderHook(() => useSearchPlaces({ city: '' }), { wrapper })
    await waitFor(() => expect(result.current.fetchStatus).toBe('idle'))
    expect(f).not.toHaveBeenCalled()
  })

  it('useMe GETs /me', async () => {
    const f = mockOk({ data: { id: 'u', username: 'wcy', role: 'user' }, request_id: 'r' })
    const { result } = renderHook(() => useMe(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(lastCall(f).url).toBe('http://127.0.0.1:8080/api/v1/me')
  })

  it('useCreatePlace POSTs create input', async () => {
    const f = mockOk({ data: { id: 'p1', version: 1 }, request_id: 'r' })
    const body: CreatePlaceInput = { id: 'p1', duration_minutes: 90, tags: ['景点'], open_minute: null, close_minute: null, fee_cents: 6000, fee_source: '官网', active: true }
    const { result } = renderHook(() => useCreatePlace(), { wrapper })
    result.current.mutate(body)
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const c = lastCall(f)
    expect(c.url).toBe('http://127.0.0.1:8080/api/v1/admin/places')
    expect(c.init.method).toBe('POST')
    expect(c.body).toEqual(body)
  })

  it('useUpdatePlace PATCHes with version', async () => {
    const f = mockOk({ data: { id: 'p1', version: 2 }, request_id: 'r' })
    const body: UpdatePlaceInput = { version: 1, duration_minutes: 120, tags: [], open_minute: 540, close_minute: 1080, fee_cents: null, fee_source: '', active: false }
    const { result } = renderHook(() => useUpdatePlace('p1'), { wrapper })
    result.current.mutate(body)
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const c = lastCall(f)
    expect(c.url).toBe('http://127.0.0.1:8080/api/v1/admin/places/p1')
    expect(c.init.method).toBe('PATCH')
    expect(c.body).toEqual(body)
  })
})
