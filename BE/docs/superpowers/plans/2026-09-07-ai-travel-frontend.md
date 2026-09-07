# AI 旅行行程规划师 · 前端 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从零搭建 `FE/` 真实 React 前端，对接现有 Go 后端，实现 MVP 全流程：注册登录 → 首页/向导 → 创建生成任务并轮询 → 行程详情（时间线 + 高德地图 + 预算）→ 我的行程列表。

**Architecture:** Vite + React SPA，react-router 多路由。`lib/http.ts` 为唯一出网入口，`api/*` 用 @tanstack/react-query 组织查询/轮询/变更并归一化数据，`features/*` 只消费 hooks。向导用 react-hook-form + zod 校验，UI 用 Tailwind + shadcn/ui，视觉沿用已确认 demo。

**Tech Stack:** Vite、React 18、TypeScript、react-router-dom、@tanstack/react-query、react-hook-form、zod、Tailwind CSS、shadcn/ui、Vitest + @testing-library/react、高德 JS API v2。

**Spec:** `BE/docs/superpowers/specs/2026-09-07-ai-travel-frontend-design.md`

## Global Constraints

- 后端地址：`http://127.0.0.1:8080`；前端固定端口 **5173**（后端 CORS 白名单已放行 `localhost:5173`、`127.0.0.1:5173`）。
- 工程位置：`小学期/FE/`，与 `BE/` 平级。
- 成功响应统一取 `body.data`，保留 `body.request_id`；所有 JSON 字段为 **snake_case**；请求体 < 1 MiB，不发未知字段。
- 受保护请求带 `Authorization: Bearer <token>`；token 存 **localStorage**；token 不写入 URL、日志、分析事件。收到 **401** 一律登出并跳登录。
- 写操作（创建生成、重试）必带 1–128 字节 `Idempotency-Key`：用户每次提交生成新键，网络重试复用同键。
- 金额单位是 **分**（分→元展示）；`complete=false` 时**不显示「未超预算」**。坐标为 **GCJ-02**；路线按每段实际 `route.mode` 渲染，不用请求偏好。
- 异步任务失败不表现为 HTTP 4xx/5xx：轮询本身返回 200，失败信息在 `data.error.code/message/status`。终态：`succeeded/failed/conflicted/interrupted`；中间态 `queued/running`；轮询每 2s，最长容忍接近 600s。
- 品牌视觉令牌（来自已确认 demo）：brand 主色 `#F97316`（500）、`#EA5A0B`（600）；字体 `Noto Sans SC` / `Noto Serif SC`；容器最大宽 `1140px`；品牌名「行迹 Wandr」。
- 环境变量：`VITE_API_BASE_URL=http://127.0.0.1:8080`、`VITE_AMAP_KEY=`（占位）。真实值放 `.env.local`，**不入库**；不提交任何 Key/token/凭据。
- **git 前置**：当前项目尚不是 git 仓库。计划中的 `git commit` 步骤要求先 `git init`（Task 1 处理）；按用户约定，实际提交/推送需用户明确同意，执行时确认。

---

### Task 1: 脚手架与基础配置

**Files:**
- Create: `FE/package.json`、`FE/vite.config.ts`、`FE/tsconfig.json`、`FE/tailwind.config.ts`、`FE/postcss.config.js`、`FE/index.html`、`FE/.gitignore`、`FE/.env.example`
- Create: `FE/src/main.tsx`、`FE/src/App.tsx`、`FE/src/index.css`
- Create: `FE/vitest.config.ts`、`FE/src/test/setup.ts`

**Interfaces:**
- Produces: 可 `npm run dev`（5173）与 `npm run test` 的工程；`App` 导出根路由骨架；Tailwind 含 brand 配色与字体。

- [ ] **Step 1: 初始化工程与依赖**

在 `FE/` 执行（若目录不存在先建）：

```bash
cd 小学期/FE
npm create vite@latest . -- --template react-ts
npm i react-router-dom @tanstack/react-query react-hook-form zod @hookform/resolvers
npm i -D tailwindcss postcss autoprefixer vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom
npx tailwindcss init -p
git init
```

- [ ] **Step 2: 写 Tailwind 配置（brand 令牌）**

`FE/tailwind.config.ts`：

```ts
import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#FFF4ED', 100: '#FFE6D5', 200: '#FECDAA', 300: '#FDAC74',
          400: '#FB8B3C', 500: '#F97316', 600: '#EA5A0B', 700: '#C2440C',
        },
      },
      fontFamily: {
        sans: ['"Noto Sans SC"', 'system-ui', 'sans-serif'],
        serif: ['"Noto Serif SC"', 'serif'],
      },
      maxWidth: { container: '1140px' },
    },
  },
  plugins: [],
} satisfies Config
```

- [ ] **Step 3: 写 index.css / vite.config / vitest.config / env**

`FE/src/index.css`：

```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

`FE/vite.config.ts`：

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: { port: 5173, strictPort: true },
})
```

`FE/vitest.config.ts`：

```ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: { environment: 'jsdom', globals: true, setupFiles: ['./src/test/setup.ts'] },
})
```

`FE/src/test/setup.ts`：

```ts
import '@testing-library/jest-dom'
```

`FE/.env.example`：

```
VITE_API_BASE_URL=http://127.0.0.1:8080
VITE_AMAP_KEY=
```

`FE/.gitignore` 追加：`node_modules`、`dist`、`.env.local`。
`FE/package.json` 的 `scripts` 增加：`"test": "vitest run"`、`"test:watch": "vitest"`。

- [ ] **Step 4: 写 main.tsx / App.tsx 根骨架**

`FE/src/main.tsx`：

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter><App /></BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
)
```

`FE/src/App.tsx`（先占位路由，后续任务填充页面）：

```tsx
import { Routes, Route } from 'react-router-dom'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<div className="p-8 font-sans">首页占位</div>} />
    </Routes>
  )
}
```

`FE/index.html` 的 `<head>` 加入字体预连接与 Noto 字体链接（复用 demo）。

- [ ] **Step 5: 验证工程可跑**

Run: `cd 小学期/FE && npm run build && npm run test`
Expected: build 成功；test 无用例时 Vitest 报 "No test files found" 视为通过（下一任务起补测试）。

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "chore(fe): scaffold vite react ts with tailwind and vitest"
```

---

### Task 2: HTTP 客户端与统一响应/错误解析

**Files:**
- Create: `FE/src/lib/http.ts`、`FE/src/lib/errors.ts`
- Test: `FE/src/lib/http.test.ts`

**Interfaces:**
- Produces:
  - `class ApiError extends Error { code: string; status: number; requestId?: string; fieldErrors?: Record<string,string> }`
  - `async function apiRequest<T>(path: string, opts?: { method?: string; body?: unknown; token?: string | null; idempotencyKey?: string }): Promise<{ data: T; requestId: string }>`
  - `getApiBaseUrl(): string`（读 `import.meta.env.VITE_API_BASE_URL`）

- [ ] **Step 1: 写失败测试**

`FE/src/lib/http.test.ts`：

```ts
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
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd 小学期/FE && npx vitest run src/lib/http.test.ts`
Expected: FAIL（`http.ts` 未实现）。

- [ ] **Step 3: 写最小实现**

`FE/src/lib/errors.ts`：

```ts
export class ApiError extends Error {
  code: string
  status: number
  requestId?: string
  fieldErrors?: Record<string, string>
  constructor(status: number, code: string, message: string, requestId?: string, fieldErrors?: Record<string, string>) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.requestId = requestId
    this.fieldErrors = fieldErrors
  }
}
```

`FE/src/lib/http.ts`：

```ts
import { ApiError } from './errors'

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
  })

  const payload = await res.json().catch(() => ({}))
  if (res.status < 200 || res.status >= 300) {
    throw new ApiError(
      res.status,
      payload.code ?? 'UNKNOWN',
      payload.message ?? `请求失败 (${res.status})`,
      payload.request_id,
      payload.field_errors,
    )
  }
  return { data: payload.data as T, requestId: payload.request_id }
}

export { ApiError }
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd 小学期/FE && npx vitest run src/lib/http.test.ts`
Expected: PASS（3 项）。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(fe): add http client with unified data/error parsing"
```

---

### Task 3: 工具函数（money / time / idempotency）

**Files:**
- Create: `FE/src/lib/money.ts`、`FE/src/lib/time.ts`、`FE/src/lib/idempotency.ts`
- Test: `FE/src/lib/money.test.ts`、`FE/src/lib/time.test.ts`

**Interfaces:**
- Produces:
  - `centsToYuan(cents: number): string`（保留 2 位，如 `2000` → `"20.00"`）
  - `budgetStatusText(complete: boolean, withinBudget?: boolean, unknownCount?: number): string`
  - `minuteToClock(minute: number): string`（`540` → `"09:00"`）
  - `newIdempotencyKey(): string`

- [ ] **Step 1: 写失败测试**

`FE/src/lib/money.test.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { centsToYuan, budgetStatusText } from './money'

describe('money', () => {
  it('converts cents to yuan with 2 decimals', () => {
    expect(centsToYuan(2000)).toBe('20.00')
    expect(centsToYuan(199)).toBe('1.99')
  })
  it('never shows within-budget when incomplete', () => {
    expect(budgetStatusText(false, undefined, 3)).toBe('含 3 项待确认费用')
  })
  it('shows within/over budget only when complete', () => {
    expect(budgetStatusText(true, true, 0)).toBe('未超预算')
    expect(budgetStatusText(true, false, 0)).toBe('已超预算')
  })
})
```

`FE/src/lib/time.test.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { minuteToClock } from './time'

describe('minuteToClock', () => {
  it('formats minutes since midnight', () => {
    expect(minuteToClock(540)).toBe('09:00')
    expect(minuteToClock(0)).toBe('00:00')
    expect(minuteToClock(1080)).toBe('18:00')
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd 小学期/FE && npx vitest run src/lib/money.test.ts src/lib/time.test.ts`
Expected: FAIL（未实现）。

- [ ] **Step 3: 写最小实现**

`FE/src/lib/money.ts`：

```ts
export function centsToYuan(cents: number): string {
  return (cents / 100).toFixed(2)
}

export function budgetStatusText(complete: boolean, withinBudget?: boolean, unknownCount = 0): string {
  if (!complete) return `含 ${unknownCount} 项待确认费用`
  return withinBudget ? '未超预算' : '已超预算'
}
```

`FE/src/lib/time.ts`：

```ts
export function minuteToClock(minute: number): string {
  const h = Math.floor(minute / 60)
  const m = minute % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}
```

`FE/src/lib/idempotency.ts`：

```ts
export function newIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `key-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd 小学期/FE && npx vitest run src/lib/money.test.ts src/lib/time.test.ts`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(fe): add money/time/idempotency helpers"
```

---

### Task 4: 类型定义与 auth 状态

**Files:**
- Create: `FE/src/types/api.ts`、`FE/src/store/auth.ts`
- Test: `FE/src/store/auth.test.ts`

**Interfaces:**
- Consumes: `apiRequest`（Task 2）。
- Produces:
  - `FE/src/types/api.ts`：`User`、`Session`、`Constraints`、`ConstraintsInput`、`Activity`、`Route`、`Plan`、`Trip`、`TripSummary`、`Job`、`BudgetSummary`、`AmountGroup`、`Cost`、`Place` 与后端 schema 同构（snake_case 字段名）。
  - `store/auth.ts`：`saveSession(s: Session)`、`loadToken(): string | null`、`loadUser(): User | null`、`clearSession()`、`useAuth(): { token, user, isAuthed, login, logout }`（基于 `useSyncExternalStore` 或简单事件订阅）。

- [ ] **Step 1: 写失败测试**

`FE/src/store/auth.test.ts`：

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { saveSession, loadToken, loadUser, clearSession } from './auth'

describe('auth store', () => {
  beforeEach(() => localStorage.clear())
  it('persists and reads session token+user', () => {
    saveSession({ token: 'abc', expires_at: '2026-10-01T00:00:00Z', user: { id: 'u1', username: 'zoe', role: 'user' } })
    expect(loadToken()).toBe('abc')
    expect(loadUser()?.username).toBe('zoe')
  })
  it('clears session', () => {
    saveSession({ token: 'abc', expires_at: 'x', user: { id: 'u1', username: 'zoe', role: 'user' } })
    clearSession()
    expect(loadToken()).toBeNull()
    expect(loadUser()).toBeNull()
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd 小学期/FE && npx vitest run src/store/auth.test.ts`
Expected: FAIL。

- [ ] **Step 3: 写类型与实现**

`FE/src/types/api.ts`（关键片段，字段与 `BE/docs/openapi.yaml` 一致）：

```ts
export interface User { id: string; username: string; role: 'user' | 'admin' }
export interface Session { token: string; expires_at: string; user: User }

export interface ConstraintsInput {
  city: string
  start_date: string
  end_date: string
  party_size: number
  budget_cents: number
  budget_scope: 'per_person' | 'total'
  interests: string[]
  pace: string
  transport: 'walking' | 'transit'
}
export interface Constraints extends ConstraintsInput { budget_total_cents?: number }

export interface Cost { category: string; amount_cents: number | null; unit: string; source: string; estimated: boolean }
export interface Place { id: string; name: string; location: string; coordinate_system?: 'GCJ-02'; address?: string; fee_cents?: number | null }
export interface Activity {
  id: string; date: string; start_minute: number; end_minute: number
  kind: string; place_id: string; title: string; reason: string
  place?: Place; costs?: Cost[]
}
export interface Route {
  from_item_id: string; to_item_id: string; date: string
  mode: 'walking' | 'transit'
  distance_m: number; duration_s: number; fee_cents: number | null
  summary: string; polyline: string; from_location: string; to_location: string
}
export interface Plan { title: string; summary: string; activities: Activity[]; routes: Route[]; warnings: string[] }
export interface Trip { id: string; version: number; constraints: Constraints; plan: Plan; created_at: string }
export interface TripSummary { id: string; version: number; constraints: Constraints; title: string; summary: string; created_at: string }

export interface JobError { code: string; message: string; status: number; field_errors?: Record<string, string> }
export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'conflicted' | 'interrupted'
export interface Job {
  id: string; kind: 'generate' | 'replan' | 'manual_edit'; status: JobStatus; stage: string
  trip_id?: string; result_version?: number; created_at: string; updated_at: string; error?: JobError
}
export interface AcceptedJob extends Job { status_url: string }

export interface AmountGroup { key: string; known_cents: number; unknown_count: number }
export interface BudgetSummary {
  trip_id: string; version: number; budget_total_cents: number; known_total_cents: number
  unknown_count: number; complete: boolean; within_budget?: boolean
  by_day: AmountGroup[]; by_category: AmountGroup[]
}
export interface Paged<T> { items: T[]; page: number; page_size: number; has_more: boolean }
```

`FE/src/store/auth.ts`：

```ts
import { useSyncExternalStore } from 'react'
import type { Session, User } from '../types/api'

const TOKEN_KEY = 'wandr.token'
const USER_KEY = 'wandr.user'
const listeners = new Set<() => void>()
function emit() { listeners.forEach((l) => l()) }

export function loadToken(): string | null { return localStorage.getItem(TOKEN_KEY) }
export function loadUser(): User | null {
  const raw = localStorage.getItem(USER_KEY)
  return raw ? (JSON.parse(raw) as User) : null
}
export function saveSession(s: Session) {
  localStorage.setItem(TOKEN_KEY, s.token)
  localStorage.setItem(USER_KEY, JSON.stringify(s.user))
  emit()
}
export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  emit()
}

function subscribe(cb: () => void) { listeners.add(cb); return () => listeners.delete(cb) }

export function useAuth() {
  const token = useSyncExternalStore(subscribe, loadToken)
  const user = useSyncExternalStore(subscribe, () => loadUser())
  return { token, user, isAuthed: !!token, login: saveSession, logout: clearSession }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd 小学期/FE && npx vitest run src/store/auth.test.ts`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(fe): add api types and localStorage auth store"
```

---

### Task 5: auth API hooks + 登录/注册页 + 受保护路由

**Files:**
- Create: `FE/src/api/auth.ts`、`FE/src/features/auth/LoginPage.tsx`、`FE/src/features/auth/RegisterPage.tsx`、`FE/src/components/RequireAuth.tsx`
- Modify: `FE/src/App.tsx`
- Test: `FE/src/api/auth.test.ts`

**Interfaces:**
- Consumes: `apiRequest`（Task 2）、auth store（Task 4）、`Session`/`User` 类型。
- Produces:
  - `useLogin()`、`useRegister()`、`useLogout()`（react-query mutations）。
  - `<RequireAuth>`：无 token 时 `<Navigate to="/login" state={{ from }}/>`，否则渲染 children。

- [ ] **Step 1: 写失败测试（登录 mutation 存 session）**

`FE/src/api/auth.test.ts`：

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useLogin } from './auth'
import { loadToken } from '../store/auth'

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('useLogin', () => {
  beforeEach(() => { localStorage.clear(); vi.stubEnv('VITE_API_BASE_URL', 'http://127.0.0.1:8080') })
  it('saves token to store on success', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 200, ok: true,
      json: () => Promise.resolve({ data: { token: 'tk', expires_at: 'x', user: { id: 'u', username: 'zoe', role: 'user' } }, request_id: 'r' }),
    }) as any
    const { result } = renderHook(() => useLogin(), { wrapper })
    result.current.mutate({ username: 'zoe', password: 'password1' })
    await waitFor(() => expect(loadToken()).toBe('tk'))
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd 小学期/FE && npx vitest run src/api/auth.test.ts`
Expected: FAIL。

- [ ] **Step 3: 写 api/auth.ts + 页面 + 受保护路由**

`FE/src/api/auth.ts`：

```ts
import { useMutation } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { saveSession, clearSession, loadToken } from '../store/auth'
import type { Session, User } from '../types/api'

interface Credentials { username: string; password: string }

export function useLogin() {
  return useMutation({
    mutationFn: async (c: Credentials) => (await apiRequest<Session>('/api/v1/auth/login', { method: 'POST', body: c })).data,
    onSuccess: (s) => saveSession(s),
  })
}

export function useRegister() {
  return useMutation({
    mutationFn: async (c: Credentials) => (await apiRequest<User>('/api/v1/auth/register', { method: 'POST', body: c })).data,
  })
}

export function useLogout() {
  return useMutation({
    mutationFn: async () => { await apiRequest<{ logged_out: boolean }>('/api/v1/auth/logout', { method: 'POST', token: loadToken() }) },
    onSettled: () => clearSession(),
  })
}
```

`FE/src/components/RequireAuth.tsx`：

```tsx
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../store/auth'

export default function RequireAuth({ children }: { children: React.ReactNode }) {
  const { isAuthed } = useAuth()
  const loc = useLocation()
  if (!isAuthed) return <Navigate to="/login" state={{ from: loc.pathname }} replace />
  return <>{children}</>
}
```

`FE/src/features/auth/LoginPage.tsx` 与 `RegisterPage.tsx`：react-hook-form + zod（username 3–32、password 8–72），提交调用 `useLogin`/`useRegister`；登录成功 `navigate(state?.from ?? '/')`；注册成功跳 `/login`。捕获 `ApiError`：401 提示凭据错误，400 用 `fieldErrors` 回填，409 提示用户名已存在。表单容器用 brand 配色与「行迹 Wandr」标识。

`FE/src/App.tsx` 增加路由：`/login`、`/register`，其余受保护路由在后续任务加入 `<RequireAuth>`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd 小学期/FE && npx vitest run src/api/auth.test.ts`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(fe): auth hooks, login/register pages, protected route"
```

---

### Task 6: 顶栏 + 首页（Hero + 热门模板 + 快速偏好入口）

**Files:**
- Create: `FE/src/components/Header.tsx`、`FE/src/features/home/HomePage.tsx`、`FE/src/features/home/templates.ts`
- Modify: `FE/src/App.tsx`

**Interfaces:**
- Consumes: `useAuth`（Task 4）、`useLogout`（Task 5）。
- Produces:
  - `TEMPLATES: TripTemplate[]`，`TripTemplate = { city: string; region: string; theme: string; days: number; budget_cents: number; interests: string[]; imgPrompt: string }`
  - `Header`：fixed 顶栏、药丸入口、登录态切换（未登录=「登录」；已登录=「我的行程」+「退出」）。
  - `HomePage`：Hero、模板卡片网格、横滑快速偏好条；点击模板/偏好触发 `onStartWizard(prefill)`（Task 7 接入向导）。

- [ ] **Step 1: 写模板数据与组件（无 TDD，纯展示）**

`templates.ts` 提供 6 条热门线路（杭州/成都/西安/厦门/大理/北京 等），图片用规范要求的 `tool_text_to_image` URL：`https://copilot-cn.bytedance.net/api/ide/v1/tool_text_to_image?prompt={encoded}&image_size=square`。

`Header.tsx`、`HomePage.tsx` 复刻 demo 结构：药丸搜索触发器、`一键生成` 角标、模板卡片（城市·地区 / 主题·N天 / 预算参考）、横滑偏好条（全部 + 文化/美食/自然/亲子/摄影…）。已登录时药丸/模板点击打开向导，未登录点击跳 `/login`。

- [ ] **Step 2: 挂路由并手动验证**

`App.tsx` 加 `/`（HomePage）。
Run: `cd 小学期/FE && npm run dev`，浏览器开 `http://127.0.0.1:5173`
Expected: 顶栏 + Hero + 模板网格 + 偏好条正常渲染，未登录点击跳登录。

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "feat(fe): header and home page with templates and quick prefs"
```

---

### Task 7: 需求向导 Modal + zod 校验 + 创建生成任务

**Files:**
- Create: `FE/src/features/wizard/wizardSchema.ts`、`FE/src/features/wizard/WizardModal.tsx`、`FE/src/api/jobs.ts`
- Modify: `FE/src/features/home/HomePage.tsx`（接入向导开关）、`FE/src/App.tsx`
- Test: `FE/src/features/wizard/wizardSchema.test.ts`

**Interfaces:**
- Consumes: `ConstraintsInput` 类型、`apiRequest`、`newIdempotencyKey`、`loadToken`。
- Produces:
  - `wizardSchema`（zod）+ `WizardValues` 类型；`toConstraintsInput(v: WizardValues): ConstraintsInput`（元→分换算：`budget_cents = budget_yuan * 100`）。
  - `useCreatePlanningJob()`：mutation，入参 `{ constraints, idempotencyKey }`，返回 `AcceptedJob`；成功后由调用方 `navigate('/planning/' + job.id)`。

- [ ] **Step 1: 写失败测试（schema 校验 + 元转分）**

`FE/src/features/wizard/wizardSchema.test.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { wizardSchema, toConstraintsInput } from './wizardSchema'

const valid = {
  city: '杭州市', start_date: '2026-10-01', end_date: '2026-10-03',
  party_size: 2, budget_yuan: 2000, budget_scope: 'per_person',
  interests: ['文化', '美食'], pace: 'balanced', transport: 'transit',
}

describe('wizardSchema', () => {
  it('accepts valid input', () => {
    expect(wizardSchema.safeParse(valid).success).toBe(true)
  })
  it('rejects end_date before start_date', () => {
    const r = wizardSchema.safeParse({ ...valid, end_date: '2026-09-30' })
    expect(r.success).toBe(false)
  })
  it('rejects party_size out of 1..8', () => {
    expect(wizardSchema.safeParse({ ...valid, party_size: 9 }).success).toBe(false)
  })
  it('converts yuan to cents', () => {
    expect(toConstraintsInput(valid as any).budget_cents).toBe(200000)
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd 小学期/FE && npx vitest run src/features/wizard/wizardSchema.test.ts`
Expected: FAIL。

- [ ] **Step 3: 写 schema + api/jobs.ts + WizardModal**

`FE/src/features/wizard/wizardSchema.ts`：

```ts
import { z } from 'zod'
import type { ConstraintsInput } from '../../types/api'

export const wizardSchema = z.object({
  city: z.string().min(1, '请填写目的地城市'),
  start_date: z.string().min(1),
  end_date: z.string().min(1),
  party_size: z.number().int().min(1).max(8),
  budget_yuan: z.number().positive('预算需大于 0'),
  budget_scope: z.enum(['per_person', 'total']),
  interests: z.array(z.string()).min(1, '至少选择一个兴趣'),
  pace: z.string().min(1),
  transport: z.enum(['walking', 'transit']),
}).refine((v) => v.end_date >= v.start_date, { path: ['end_date'], message: '结束日期不能早于开始日期' })
  .refine((v) => {
    const days = (new Date(v.end_date).getTime() - new Date(v.start_date).getTime()) / 86400000 + 1
    return days >= 1 && days <= 7
  }, { path: ['end_date'], message: '行程天数需在 1–7 天' })

export type WizardValues = z.infer<typeof wizardSchema>

export function toConstraintsInput(v: WizardValues): ConstraintsInput {
  return {
    city: v.city, start_date: v.start_date, end_date: v.end_date,
    party_size: v.party_size, budget_cents: Math.round(v.budget_yuan * 100),
    budget_scope: v.budget_scope, interests: v.interests, pace: v.pace, transport: v.transport,
  }
}
```

`FE/src/api/jobs.ts`：

```ts
import { useMutation } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { loadToken } from '../store/auth'
import type { AcceptedJob, ConstraintsInput } from '../types/api'

export function useCreatePlanningJob() {
  return useMutation({
    mutationFn: async (args: { constraints: ConstraintsInput; idempotencyKey: string }) =>
      (await apiRequest<AcceptedJob>('/api/v1/planning-jobs', {
        method: 'POST', body: { constraints: args.constraints },
        token: loadToken(), idempotencyKey: args.idempotencyKey,
      })).data,
  })
}
```

`WizardModal.tsx`：react-hook-form + `zodResolver(wizardSchema)`，字段：城市、开始/结束日期、人数、预算(元)+口径、兴趣多选、节奏(relaxed/balanced/packed)、交通(transit/walking)。支持 `prefill`（模板/偏好带入）。提交时 `key = newIdempotencyKey()`（存在 ref 里，失败重试复用同键），调用 `useCreatePlanningJob`，成功 `navigate('/planning/' + job.id)`。捕获 `ApiError`：400 回填 `fieldErrors`，401 跳登录，429 提示配额，422 提示业务约束（展示 message）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd 小学期/FE && npx vitest run src/features/wizard/wizardSchema.test.ts`
Expected: PASS（4 项）。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(fe): wizard modal with zod validation and create job"
```

---

### Task 8: 生成中页（轮询 + stage + 错误分类 + 重试）

**Files:**
- Create: `FE/src/features/planning/PlanningPage.tsx`、`FE/src/api/jobPolling.ts`
- Modify: `FE/src/App.tsx`
- Test: `FE/src/api/jobPolling.test.ts`

**Interfaces:**
- Consumes: `apiRequest`、`loadToken`、`Job`/`JobStatus` 类型、`newIdempotencyKey`。
- Produces:
  - `useJobPolling(jobId: string)`：react-query query，`refetchInterval` 在非终态返回 2000、终态返回 `false`；返回 `Job`。
  - `isTerminal(status: JobStatus): boolean`。
  - `useRetryJob()`：mutation `POST /api/v1/planning-jobs/{id}/retries`（新幂等键）。

- [ ] **Step 1: 写失败测试（终态判定 + 轮询间隔逻辑）**

`FE/src/api/jobPolling.test.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { isTerminal, pollInterval } from './jobPolling'

describe('job polling helpers', () => {
  it('marks terminal statuses', () => {
    expect(isTerminal('succeeded')).toBe(true)
    expect(isTerminal('failed')).toBe(true)
    expect(isTerminal('conflicted')).toBe(true)
    expect(isTerminal('interrupted')).toBe(true)
    expect(isTerminal('queued')).toBe(false)
    expect(isTerminal('running')).toBe(false)
  })
  it('polls every 2s until terminal', () => {
    expect(pollInterval({ status: 'running' } as any)).toBe(2000)
    expect(pollInterval({ status: 'succeeded' } as any)).toBe(false)
    expect(pollInterval(undefined)).toBe(2000)
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd 小学期/FE && npx vitest run src/api/jobPolling.test.ts`
Expected: FAIL。

- [ ] **Step 3: 写 jobPolling.ts + PlanningPage**

`FE/src/api/jobPolling.ts`：

```ts
import { useQuery, useMutation } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { loadToken } from '../store/auth'
import type { Job, JobStatus, AcceptedJob } from '../types/api'

const TERMINAL: JobStatus[] = ['succeeded', 'failed', 'conflicted', 'interrupted']
export function isTerminal(s: JobStatus): boolean { return TERMINAL.includes(s) }
export function pollInterval(job: Job | undefined): number | false {
  if (!job) return 2000
  return isTerminal(job.status) ? false : 2000
}

export function useJobPolling(jobId: string) {
  return useQuery({
    queryKey: ['job', jobId],
    queryFn: async () => (await apiRequest<Job>(`/api/v1/planning-jobs/${jobId}`, { token: loadToken() })).data,
    refetchInterval: (q) => pollInterval(q.state.data),
  })
}

export function useRetryJob() {
  return useMutation({
    mutationFn: async (args: { jobId: string; idempotencyKey: string }) =>
      (await apiRequest<AcceptedJob>(`/api/v1/planning-jobs/${args.jobId}/retries`, {
        method: 'POST', token: loadToken(), idempotencyKey: args.idempotencyKey,
      })).data,
  })
}
```

`PlanningPage.tsx`（受保护，路由 `/planning/:jobId`）：
- 用 `useJobPolling(jobId)`。中间态展示骨架屏 + 真实 `job.stage` 文案 + 已等待计时；不在 60s/300s 自行判失败。
- `succeeded`：`navigate('/trips/' + job.trip_id, { replace: true })`。
- `failed`/`interrupted`：展示 `job.error.message`，「重试」按钮调用 `useRetryJob({ jobId, idempotencyKey: newIdempotencyKey() })`，成功后跳 `/planning/newJob.id`。
- `conflicted`：提示需重新提交，给「返回首页重填」按钮。
- 组件维护一个软上限计时（≈600s）：超过后展示「仍在处理，可稍后在我的行程查看」并给「去我的行程」出口，但不停止已在跑的任务。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd 小学期/FE && npx vitest run src/api/jobPolling.test.ts`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(fe): planning page with 2s polling, stage display, retry"
```

---

### Task 9: 行程详情 —— 数据 hook + 时间线 + 预算面板

**Files:**
- Create: `FE/src/api/trips.ts`、`FE/src/features/trip/TripPage.tsx`、`FE/src/features/trip/Timeline.tsx`、`FE/src/features/trip/BudgetPanel.tsx`
- Modify: `FE/src/App.tsx`
- Test: `FE/src/features/trip/BudgetPanel.test.tsx`

**Interfaces:**
- Consumes: `apiRequest`、`loadToken`、`Trip`/`BudgetSummary`/`Activity`/`Route` 类型、`centsToYuan`、`budgetStatusText`、`minuteToClock`。
- Produces:
  - `useTrip(id)`：`GET /api/v1/trips/{id}` → `Trip`。
  - `useTripBudget(id)`：`GET /api/v1/trips/{id}/budget` → `BudgetSummary`。
  - `Timeline`：按 date 分组、按 start_minute 排序渲染活动（时刻、title、reason、place、costs）。
  - `BudgetPanel`：总额/已知/未知项，`by_day`/`by_category`，状态文案走 `budgetStatusText`；渲染 `plan.warnings`。

- [ ] **Step 1: 写失败测试（预算面板不误报未超预算）**

`FE/src/features/trip/BudgetPanel.test.tsx`：

```tsx
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import BudgetPanel from './BudgetPanel'
import type { BudgetSummary } from '../../types/api'

const incomplete: BudgetSummary = {
  trip_id: 't', version: 1, budget_total_cents: 400000, known_total_cents: 250000,
  unknown_count: 3, complete: false, by_day: [], by_category: [],
}

describe('BudgetPanel', () => {
  it('shows pending-cost text and never "未超预算" when incomplete', () => {
    render(<BudgetPanel budget={incomplete} warnings={[]} />)
    expect(screen.getByText('含 3 项待确认费用')).toBeInTheDocument()
    expect(screen.queryByText('未超预算')).toBeNull()
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd 小学期/FE && npx vitest run src/features/trip/BudgetPanel.test.tsx`
Expected: FAIL。

- [ ] **Step 3: 写 api/trips.ts + 组件**

`FE/src/api/trips.ts`：

```ts
import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { loadToken } from '../store/auth'
import type { Trip, BudgetSummary, TripSummary, Paged } from '../types/api'

export function useTrip(id: string) {
  return useQuery({
    queryKey: ['trip', id],
    queryFn: async () => (await apiRequest<Trip>(`/api/v1/trips/${id}`, { token: loadToken() })).data,
  })
}
export function useTripBudget(id: string) {
  return useQuery({
    queryKey: ['trip-budget', id],
    queryFn: async () => (await apiRequest<BudgetSummary>(`/api/v1/trips/${id}/budget`, { token: loadToken() })).data,
  })
}
export function useMyTrips(page = 1, pageSize = 20) {
  return useQuery({
    queryKey: ['trips', page, pageSize],
    queryFn: async () => (await apiRequest<Paged<TripSummary>>(`/api/v1/trips?page=${page}&page_size=${pageSize}`, { token: loadToken() })).data,
  })
}
```

`BudgetPanel.tsx`：props `{ budget: BudgetSummary; warnings: string[] }`，用 `centsToYuan` 展示金额，状态文案用 `budgetStatusText(budget.complete, budget.within_budget, budget.unknown_count)`；`by_day`/`by_category` 列表；warnings 以告警样式列出。

`Timeline.tsx`：props `{ activities: Activity[]; routes: Route[] }`，按 `date` 分天，天内按 `start_minute` 升序；每个活动展示 `minuteToClock(start)-minuteToClock(end)`、title、reason、place.name、costs（分→元，未知价显示「待确认」）；活动之间插入对应 route 段：`mode` 徽章（transit/walking 不同色）+ `distance_m`/`duration_s`/`summary`。

`TripPage.tsx`（受保护，`/trips/:id`）：`useTrip` + `useTripBudget`，加载态骨架、404 提示、渲染 `plan.title`/`summary` + `Timeline` + 地图区（Task 10 接入）+ `BudgetPanel`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd 小学期/FE && npx vitest run src/features/trip/BudgetPanel.test.tsx`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(fe): trip detail with timeline and budget panel"
```

---

### Task 10: 高德地图集成（点位 + 路线折线，含降级）

**Files:**
- Create: `FE/src/lib/amap.ts`、`FE/src/features/trip/TripMap.tsx`
- Modify: `FE/src/features/trip/TripPage.tsx`
- Test: `FE/src/lib/amap.test.ts`

**Interfaces:**
- Consumes: `Activity`/`Route` 类型、`import.meta.env.VITE_AMAP_KEY`。
- Produces:
  - `parseGcj02(location: string): [number, number]`（"经度,纬度" → `[lng, lat]`）。
  - `parsePolyline(polyline: string): [number, number][]`（分号分隔的 "lng,lat" 串）。
  - `loadAmap(key: string): Promise<any>`（懒加载 JS SDK，单例）。
  - `TripMap`：props `{ activities, routes }`，下点 + 画每段折线（按 `mode` 着色）；无 Key 或加载失败时渲染「地图不可用」占位。

- [ ] **Step 1: 写失败测试（坐标/折线解析）**

`FE/src/lib/amap.test.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { parseGcj02, parsePolyline } from './amap'

describe('amap parsing', () => {
  it('parses "lng,lat" into [lng, lat] numbers', () => {
    expect(parseGcj02('120.155,30.274')).toEqual([120.155, 30.274])
  })
  it('parses polyline into coordinate pairs', () => {
    expect(parsePolyline('120.1,30.2;120.3,30.4')).toEqual([[120.1, 30.2], [120.3, 30.4]])
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd 小学期/FE && npx vitest run src/lib/amap.test.ts`
Expected: FAIL。

- [ ] **Step 3: 写 amap.ts + TripMap**

`FE/src/lib/amap.ts`（解析函数 + SDK 懒加载）：

```ts
export function parseGcj02(location: string): [number, number] {
  const [lng, lat] = location.split(',').map(Number)
  return [lng, lat]
}

export function parsePolyline(polyline: string): [number, number][] {
  if (!polyline) return []
  return polyline.split(';').filter(Boolean).map((p) => {
    const [lng, lat] = p.split(',').map(Number)
    return [lng, lat] as [number, number]
  })
}

let amapPromise: Promise<any> | null = null
export function loadAmap(key: string): Promise<any> {
  if (!key) return Promise.reject(new Error('NO_AMAP_KEY'))
  if (amapPromise) return amapPromise
  amapPromise = new Promise((resolve, reject) => {
    if ((window as any).AMap) return resolve((window as any).AMap)
    const s = document.createElement('script')
    s.src = `https://webapi.amap.com/maps?v=2.0&key=${key}`
    s.onload = () => resolve((window as any).AMap)
    s.onerror = () => reject(new Error('AMAP_LOAD_FAILED'))
    document.head.appendChild(s)
  })
  return amapPromise
}
```

`TripMap.tsx`：`useEffect` 里 `loadAmap(import.meta.env.VITE_AMAP_KEY)`。成功后建图，对每个有 `place.location` 的活动下 `AMap.Marker`；对每段 route 用 `parsePolyline(route.polyline)` 画 `AMap.Polyline`，`mode==='transit'` 用 brand 蓝、`walking` 用虚线灰；`setFitView`。失败（含 `INVALID_USER_KEY`、`NO_AMAP_KEY`、加载失败）渲染灰底占位「地图不可用，可参考下方文字路线」，不阻塞页面。地图容器高度固定（如 `h-80`）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd 小学期/FE && npx vitest run src/lib/amap.test.ts`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(fe): amap integration for points and routes with fallback"
```

---

### Task 11: 我的行程列表 + 路由收口 + 端到端手动验证

**Files:**
- Create: `FE/src/features/trips/MyTripsPage.tsx`
- Modify: `FE/src/App.tsx`（收口全部路由 + `<RequireAuth>` 包裹受保护路由）

**Interfaces:**
- Consumes: `useMyTrips`（Task 9）、`centsToYuan`、`Header`。
- Produces: `MyTripsPage`（`/trips`），摘要卡片列表 + 分页（`has_more` 控制「加载更多/下一页」），点进 `/trips/:id`。

- [ ] **Step 1: 写 MyTripsPage + 路由收口**

`MyTripsPage.tsx`：`useMyTrips(page)`，展示 `title`/`summary`/`constraints.city`/天数/`created_at`；空态提示「还没有行程，去首页生成一条」；分页按 `has_more`。

`App.tsx` 最终路由：

```tsx
import { Routes, Route } from 'react-router-dom'
import RequireAuth from './components/RequireAuth'
import HomePage from './features/home/HomePage'
import LoginPage from './features/auth/LoginPage'
import RegisterPage from './features/auth/RegisterPage'
import PlanningPage from './features/planning/PlanningPage'
import TripPage from './features/trip/TripPage'
import MyTripsPage from './features/trips/MyTripsPage'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/planning/:jobId" element={<RequireAuth><PlanningPage /></RequireAuth>} />
      <Route path="/trips" element={<RequireAuth><MyTripsPage /></RequireAuth>} />
      <Route path="/trips/:id" element={<RequireAuth><TripPage /></RequireAuth>} />
    </Routes>
  )
}
```

- [ ] **Step 2: 全量单测**

Run: `cd 小学期/FE && npm run test`
Expected: 所有测试 PASS。

- [ ] **Step 3: 端到端手动验证（需后端已起）**

前置：`cd 小学期/BE` 按 README 启动 planner/travel/api（Key 已配齐），确认 `curl http://127.0.0.1:8080/readyz` 返回 ready。
在 `FE/` 建 `.env.local`（填 `VITE_API_BASE_URL`、`VITE_AMAP_KEY`），`npm run dev`。
走查：注册→登录→首页选模板/填向导→创建任务跳生成中→轮询到 succeeded→行程详情（时间线+地图+预算）→我的行程列表→退出。
Expected: 全流程连通；预算未完整时不显示「未超预算」；地图无 Key 时降级为占位。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat(fe): my trips list and finalize routes"
```

---

## Self-Review

**Spec coverage：**
- §1 决策 → Task 1（脚手架/配置/端口/env/品牌令牌）✓
- §3 路由表 → Task 5/6/7/8/9/10/11 全覆盖（`/`、`/login`、`/register`、`/planning/:jobId`、`/trips`、`/trips/:id`）✓
- §4 目录结构 → 各任务 Files 与目录一致 ✓
- §5.1 统一约定 → Task 2（data/error 解析、状态码）✓
- §5.2 鉴权 → Task 4/5（token 存取、401 登出、Bearer、logout）✓
- §5.3 创建任务 + 幂等 → Task 7 ✓
- §5.4 轮询/终态/重试 → Task 8 ✓
- §5.5 详情（时间线/地图/预算，mode 渲染、GCJ-02、complete=false）→ Task 9/10 ✓
- §5.6 我的行程 → Task 9(hook)/11(页面) ✓
- §6 错误与边界 → Task 5/7/8（fieldErrors 回填、401、429、503/504）、Task 10（地图降级）✓
- §7 视觉基调 → Task 1（令牌）、Task 6（顶栏/首页复刻 demo）✓
- §8 环境运行 → Task 1（.env.example、5173）、Task 11（.env.local 验证）✓
- §9 显式排除 → 计划不含 replan/manual_edit/版本历史/管理员页 ✓

**Placeholder scan：** 无 TBD/TODO；展示型任务（6、11）无强测试用例但给了明确结构与手动验证步骤；带逻辑的任务均含 TDD 与实际代码。

**Type consistency：** `ConstraintsInput`/`Trip`/`Job`/`BudgetSummary`/`AmountGroup` 等类型在 Task 4 定义，后续任务引用一致；`isTerminal`/`pollInterval`/`parseGcj02`/`parsePolyline`/`budgetStatusText`/`minuteToClock`/`centsToYuan`/`newIdempotencyKey`/`toConstraintsInput`/`apiRequest` 命名在定义与消费处一致。
