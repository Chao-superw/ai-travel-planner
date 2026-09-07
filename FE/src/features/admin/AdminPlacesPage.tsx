import { useEffect, useState } from 'react'
import { Navigate } from 'react-router-dom'
import Header from '../../components/Header'
import { useAuth } from '../../store/auth'
import { useSearchPlaces } from '../../api/places'
import { apiRequest } from '../../lib/http'
import { loadToken } from '../../store/auth'
import { ApiError } from '../../lib/errors'
import { minuteToClock } from '../../lib/time'
import type { PlaceFull, CreatePlaceInput, UpdatePlaceInput } from '../../types/api'

function clockToMinute(v: string): number | null {
  const m = /^(\d{1,2}):(\d{2})$/.exec(v)
  if (!m) return null
  const min = Number(m[1]) * 60 + Number(m[2])
  return min >= 0 && min <= 1440 ? min : null
}

interface FormState {
  duration_minutes: number
  tagsText: string
  open: string
  close: string
  feeYuan: string
  fee_source: string
  active: boolean
}

function toForm(p: PlaceFull): FormState {
  return {
    duration_minutes: p.duration_minutes || 60,
    tagsText: p.tags.join('、'),
    open: p.open_minute != null ? minuteToClock(p.open_minute) : '',
    close: p.close_minute != null ? minuteToClock(p.close_minute) : '',
    feeYuan: p.fee_cents != null ? (p.fee_cents / 100).toString() : '',
    fee_source: p.fee_source ?? '',
    active: p.active,
  }
}

function EditForm({ place, onSaved }: { place: PlaceFull; onSaved: (p: PlaceFull) => void }) {
  const [f, setF] = useState<FormState>(() => toForm(place))
  const [msg, setMsg] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null)
  const [saving, setSaving] = useState(false)
  useEffect(() => { setF(toForm(place)); setMsg(null) }, [place])

  const buildInput = (): { edit: Omit<CreatePlaceInput, 'id'>; error?: string } => {
    const tags = f.tagsText.split(/[、,，\s]+/).map((t) => t.trim()).filter(Boolean)
    if (tags.length > 10) return { edit: {} as any, error: '标签最多 10 个' }
    if (f.duration_minutes < 15 || f.duration_minutes > 480) return { edit: {} as any, error: '游玩时长应为 15–480 分钟' }

    let open: number | null = null
    let close: number | null = null
    if (f.open || f.close) {
      open = clockToMinute(f.open)
      close = clockToMinute(f.close)
      if (open == null || close == null || open >= close) return { edit: {} as any, error: '营业时间需完整且开始早于结束' }
    }

    let fee_cents: number | null = null
    if (f.feeYuan.trim() !== '') {
      const n = Number(f.feeYuan)
      if (!Number.isFinite(n) || n < 0) return { edit: {} as any, error: '费用需为非负数字' }
      fee_cents = Math.round(n * 100)
      if (!f.fee_source.trim()) return { edit: {} as any, error: '填写费用时必须提供费用来源' }
    }

    return {
      edit: {
        duration_minutes: f.duration_minutes,
        tags,
        open_minute: open,
        close_minute: close,
        fee_cents,
        fee_source: f.fee_source.trim(),
        active: f.active,
      },
    }
  }

  const onSave = async () => {
    const { edit, error } = buildInput()
    if (error) { setMsg({ kind: 'err', text: error }); return }
    setSaving(true)
    setMsg(null)
    try {
      // 先尝试首次维护（create），若后端提示已维护则回退为更新（update，带 version 乐观锁）
      let saved: PlaceFull
      try {
        const body: CreatePlaceInput = { id: place.id, ...edit }
        saved = (await apiRequest<PlaceFull>('/api/v1/admin/places', { method: 'POST', body, token: loadToken() })).data
      } catch (e) {
        if (e instanceof ApiError && e.status === 409 && e.code === 'CONFLICT') {
          const body: UpdatePlaceInput = { version: place.version, ...edit }
          saved = (await apiRequest<PlaceFull>(`/api/v1/admin/places/${place.id}`, { method: 'PATCH', body, token: loadToken() })).data
        } else {
          throw e
        }
      }
      setMsg({ kind: 'ok', text: '已保存' })
      onSaved(saved)
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.code === 'VERSION_CONFLICT') { setMsg({ kind: 'err', text: '该景点资料已被他人修改，请重新搜索后再试' }); return }
        setMsg({ kind: 'err', text: e.message })
        return
      }
      setMsg({ kind: 'err', text: '网络异常，请重试' })
    } finally {
      setSaving(false)
    }
  }

  const field = 'w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500'

  return (
    <div className="rounded-2xl border border-gray-100 p-5">
      <div className="flex items-center justify-between mb-1">
        <h3 className="text-base font-medium text-gray-900">{place.name}</h3>
        <span className="text-xs text-gray-400">版本 v{place.version}</span>
      </div>
      {place.address && <p className="text-sm text-gray-500 mb-4">📍 {place.address}</p>}

      {msg && (
        <div className={`mb-3 rounded-lg text-sm px-3 py-2 ${msg.kind === 'ok' ? 'bg-brand-50 text-brand-600' : 'bg-red-50 text-red-600'}`}>{msg.text}</div>
      )}

      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-sm text-gray-700">游玩时长（分钟，15–480）</label>
            <input type="number" min={15} max={480} value={f.duration_minutes}
              onChange={(e) => setF({ ...f, duration_minutes: Number(e.target.value) })} className={field} />
          </div>
          <div className="flex items-end pb-2">
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={f.active} onChange={(e) => setF({ ...f, active: e.target.checked })} />
              对外可用（上架）
            </label>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-sm text-gray-700">开始营业（HH:MM，可空）</label>
            <input type="time" value={f.open} onChange={(e) => setF({ ...f, open: e.target.value })} className={field} />
          </div>
          <div>
            <label className="text-sm text-gray-700">结束营业（HH:MM，可空）</label>
            <input type="time" value={f.close} onChange={(e) => setF({ ...f, close: e.target.value })} className={field} />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-sm text-gray-700">门票（元，可空）</label>
            <input type="number" min={0} step="0.01" value={f.feeYuan} onChange={(e) => setF({ ...f, feeYuan: e.target.value })} className={field} placeholder="留空表示未知" />
          </div>
          <div>
            <label className="text-sm text-gray-700">费用来源（填费用时必填）</label>
            <input value={f.fee_source} onChange={(e) => setF({ ...f, fee_source: e.target.value })} className={field} placeholder="如：官网 / 现场公示" />
          </div>
        </div>

        <div>
          <label className="text-sm text-gray-700">标签（顿号或逗号分隔，最多 10 个）</label>
          <input value={f.tagsText} onChange={(e) => setF({ ...f, tagsText: e.target.value })} className={field} placeholder="如：文化、地标、亲子" />
        </div>

        <button onClick={onSave} disabled={saving} className="w-full bg-brand-500 hover:bg-brand-600 text-white rounded-lg py-2.5 font-medium disabled:opacity-60">
          {saving ? '正在保存…' : '保存'}
        </button>
      </div>
    </div>
  )
}

export default function AdminPlacesPage() {
  const { user } = useAuth()
  const [city, setCity] = useState('')
  const [keyword, setKeyword] = useState('')
  const [query, setQuery] = useState<{ city: string; keyword: string }>({ city: '', keyword: '' })
  const [selected, setSelected] = useState<PlaceFull | null>(null)

  const { data, isFetching, error, refetch } = useSearchPlaces({ city: query.city, keyword: query.keyword, pageSize: 25 }, query.city !== '')

  if (user && user.role !== 'admin') return <Navigate to="/" replace />

  const onSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setSelected(null)
    setQuery({ city: city.trim(), keyword: keyword.trim() })
  }

  return (
    <div className="min-h-screen bg-white font-sans">
      <Header />
      <main className="pt-16 mx-auto max-w-container px-4 py-8">
        <h1 className="text-2xl font-serif text-gray-900 mb-2">景点资料维护</h1>
        <p className="text-gray-500 mb-6">先搜索地点（会自动入库），再精修其游玩时长、营业时间、门票与标签。</p>

        <form onSubmit={onSearch} className="flex flex-col sm:flex-row gap-3 mb-6">
          <input value={city} onChange={(e) => setCity(e.target.value)} placeholder="城市（必填）"
            className="flex-1 border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500" />
          <input value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="关键词（可选）"
            className="flex-1 border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500" />
          <button type="submit" disabled={!city.trim()} className="rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-6 py-2 text-sm font-medium disabled:opacity-50">搜索</button>
        </form>

        {error && <p className="text-red-600">搜索失败：{error instanceof Error ? error.message : '未知错误'}</p>}
        {isFetching && <p className="text-gray-500">正在搜索…</p>}

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div>
            {data && data.items.length === 0 && !isFetching && <p className="text-gray-500">没有结果</p>}
            <ul className="space-y-2">
              {data?.items.map((p) => (
                <li key={p.id}>
                  <button
                    onClick={() => setSelected(p)}
                    className={`w-full text-left rounded-xl border px-3 py-2 transition ${selected?.id === p.id ? 'border-brand-500 bg-brand-50' : 'border-gray-100 hover:bg-gray-50'}`}
                  >
                    <div className="font-medium text-gray-900 text-sm">{p.name}</div>
                    {p.address && <div className="text-xs text-gray-500 line-clamp-1">{p.address}</div>}
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div>
            {selected
              ? <EditForm place={selected} onSaved={() => { void refetch() }} />
              : <p className="text-gray-400 text-sm">从左侧选择一个地点开始维护。</p>}
          </div>
        </div>
      </main>
    </div>
  )
}
