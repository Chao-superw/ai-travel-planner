import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useManualEdit } from '../../api/trips'
import { newIdempotencyKey } from '../../lib/idempotency'
import { minuteToClock } from '../../lib/time'
import { ApiError } from '../../lib/errors'
import PlacePicker from '../places/PlacePicker'
import type { Plan, ActivityEditInput, PlanEditInput, ManualEditInput, PlaceFull } from '../../types/api'

const KIND_OPTIONS: { value: string; label: string }[] = [
  { value: 'sightseeing', label: '景点' },
  { value: 'meal', label: '用餐' },
  { value: 'hotel', label: '住宿' },
  { value: 'rest', label: '休息' },
]
const KIND_LABEL: Record<string, string> = Object.fromEntries(KIND_OPTIONS.map((k) => [k.value, k.label]))

// 编辑器内部工作态：在活动上额外挂 place_name 以便展示，提交前剥离
interface EditActivity extends ActivityEditInput {
  place_name?: string
}

function clockToMinute(v: string): number {
  const m = /^(\d{1,2}):(\d{2})$/.exec(v)
  if (!m) return 0
  return Number(m[1]) * 60 + Number(m[2])
}

function toEditActivities(plan: Plan): EditActivity[] {
  return plan.activities
    .slice()
    .sort((a, b) => (a.date === b.date ? a.start_minute - b.start_minute : a.date < b.date ? -1 : 1))
    .map((a) => ({
      id: a.id, date: a.date, start_minute: a.start_minute, end_minute: a.end_minute,
      kind: a.kind, place_id: a.place_id, title: a.title, reason: a.reason,
      place_name: a.place?.name,
    }))
}

let seq = 0
function newActivityId(): string {
  seq += 1
  return `edit-${Date.now().toString(36)}-${seq}`
}

// 完整可视化手动编辑器：改标题/简介、逐项调整活动、增删、挂载地点，提交完整 plan。
export default function ManualEditor({
  tripId,
  version,
  plan,
  city,
  onClose,
}: {
  tripId: string
  version: number
  plan: Plan
  city: string
  onClose: () => void
}) {
  const nav = useNavigate()
  const edit = useManualEdit(tripId)

  const [title, setTitle] = useState(plan.title)
  const [summary, setSummary] = useState(plan.summary)
  const [acts, setActs] = useState<EditActivity[]>(() => toEditActivities(plan))
  const [pickerFor, setPickerFor] = useState<string | null>(null)
  const [topError, setTopError] = useState<string | null>(null)

  const dates = useMemo(() => [...new Set(acts.map((a) => a.date))].sort(), [acts])

  const patch = (id: string, next: Partial<EditActivity>) =>
    setActs((prev) => prev.map((a) => (a.id === id ? { ...a, ...next } : a)))
  const remove = (id: string) => setActs((prev) => prev.filter((a) => a.id !== id))

  const move = (id: string, dir: -1 | 1) =>
    setActs((prev) => {
      const sameDay = prev.filter((a) => a.date === prev.find((x) => x.id === id)!.date)
        .sort((a, b) => a.start_minute - b.start_minute)
      const idx = sameDay.findIndex((a) => a.id === id)
      const swap = sameDay[idx + dir]
      if (!swap) return prev
      // 交换两者时间段，实现顺序调整
      const cur = sameDay[idx]
      return prev.map((a) => {
        if (a.id === cur.id) return { ...a, start_minute: swap.start_minute, end_minute: swap.end_minute }
        if (a.id === swap.id) return { ...a, start_minute: cur.start_minute, end_minute: cur.end_minute }
        return a
      })
    })

  const addActivity = (date: string) => {
    const dayActs = acts.filter((a) => a.date === date).sort((a, b) => a.start_minute - b.start_minute)
    const last = dayActs[dayActs.length - 1]
    const start = last ? Math.min(last.end_minute + 30, 1380) : 540
    setActs((prev) => [
      ...prev,
      { id: newActivityId(), date, start_minute: start, end_minute: Math.min(start + 60, 1440), kind: 'sightseeing', place_id: '', title: '新活动', reason: '' },
    ])
  }

  const validate = (): string | null => {
    if (!title.trim()) return '请填写行程标题'
    if (acts.length === 0) return '至少保留一个活动'
    for (const a of acts) {
      if (!a.title.trim()) return '每个活动都需要标题'
      if (a.start_minute < 0 || a.end_minute > 1440 || a.end_minute <= a.start_minute) return `「${a.title}」的时间段无效`
      if (a.kind === 'sightseeing' && !a.place_id) return `景点「${a.title}」需要选择一个地点`
    }
    return null
  }

  const onSubmit = () => {
    setTopError(null)
    const err = validate()
    if (err) { setTopError(err); return }

    const plan: PlanEditInput = {
      title: title.trim(),
      summary: summary.trim(),
      activities: acts.map<ActivityEditInput>((a) => ({
        id: a.id, date: a.date, start_minute: a.start_minute, end_minute: a.end_minute,
        kind: a.kind, place_id: a.kind === 'sightseeing' ? a.place_id : '', title: a.title.trim(), reason: a.reason.trim(),
      })),
    }
    const input: ManualEditInput = { expected_version: version, plan }
    edit.mutate(
      { input, idempotencyKey: newIdempotencyKey() },
      {
        onSuccess: (job) => { onClose(); nav('/planning/' + job.id) },
        onError: (e) => {
          if (e instanceof ApiError) {
            if (e.status === 409) { setTopError('行程已被修改，请关闭后刷新页面再试'); return }
            setTopError(e.message)
            return
          }
          setTopError('网络异常，请重试')
        },
      },
    )
  }

  const field = 'border border-gray-200 rounded-lg px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500'

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 px-4" onClick={onClose}>
      <div className="w-full max-w-3xl bg-white rounded-2xl shadow-xl p-6 font-sans max-h-[92vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-medium text-gray-900">手动编辑行程</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600" aria-label="关闭">✕</button>
        </div>

        {topError && <div className="mb-3 rounded-lg bg-red-50 text-red-600 text-sm px-3 py-2">{topError}</div>}
        <div className="mb-4 rounded-lg bg-amber-50 border border-amber-100 text-amber-700 text-xs px-3 py-2">
          提交后系统会重新校验并核价、查询路线，生成一个新版本。若不满足「每天 1–5 个景点、含用餐、非末日含住宿」等规则会提示失败。
        </div>

        <div className="space-y-3 mb-6">
          <div>
            <label className="text-sm text-gray-700">标题</label>
            <input value={title} onChange={(e) => setTitle(e.target.value)} className={`${field} w-full`} />
          </div>
          <div>
            <label className="text-sm text-gray-700">简介</label>
            <textarea value={summary} onChange={(e) => setSummary(e.target.value)} rows={2} className={`${field} w-full`} />
          </div>
        </div>

        {dates.map((date) => {
          const dayActs = acts.filter((a) => a.date === date).sort((a, b) => a.start_minute - b.start_minute)
          return (
            <section key={date} className="mb-6">
              <div className="flex items-center justify-between mb-2">
                <h3 className="text-sm font-medium text-gray-900">{date}</h3>
                <button onClick={() => addActivity(date)} className="text-sm text-brand-600 hover:text-brand-700">+ 添加活动</button>
              </div>
              <ul className="space-y-3">
                {dayActs.map((a, idx) => (
                  <li key={a.id} className="rounded-xl border border-gray-100 p-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <input type="time" value={minuteToClock(a.start_minute)} onChange={(e) => patch(a.id, { start_minute: clockToMinute(e.target.value) })} className={field} />
                      <span className="text-gray-400">–</span>
                      <input type="time" value={minuteToClock(a.end_minute)} onChange={(e) => patch(a.id, { end_minute: clockToMinute(e.target.value) })} className={field} />
                      <select value={a.kind} onChange={(e) => patch(a.id, { kind: e.target.value })} className={field}>
                        {KIND_OPTIONS.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
                      </select>
                      <div className="ml-auto flex items-center gap-1">
                        <button onClick={() => move(a.id, -1)} disabled={idx === 0} className="text-gray-400 hover:text-gray-700 disabled:opacity-30 px-1" aria-label="上移">↑</button>
                        <button onClick={() => move(a.id, 1)} disabled={idx === dayActs.length - 1} className="text-gray-400 hover:text-gray-700 disabled:opacity-30 px-1" aria-label="下移">↓</button>
                        <button onClick={() => remove(a.id)} className="text-red-400 hover:text-red-600 px-1" aria-label="删除">✕</button>
                      </div>
                    </div>
                    <input value={a.title} onChange={(e) => patch(a.id, { title: e.target.value })} placeholder="活动标题" className={`${field} w-full mt-2`} />
                    <input value={a.reason} onChange={(e) => patch(a.id, { reason: e.target.value })} placeholder="备注 / 理由（可选）" className={`${field} w-full mt-2`} />
                    {a.kind === 'sightseeing' && (
                      <div className="mt-2 flex items-center gap-2 text-sm">
                        <span className="text-gray-500">地点：</span>
                        {a.place_id
                          ? <span className="text-gray-800">{a.place_name ?? a.place_id}</span>
                          : <span className="text-amber-600">未选择</span>}
                        <button onClick={() => setPickerFor(a.id)} className="text-brand-600 hover:text-brand-700">{a.place_id ? '更换' : '选择地点'}</button>
                      </div>
                    )}
                    <div className="mt-1 text-xs text-gray-400">{KIND_LABEL[a.kind]}</div>
                  </li>
                ))}
                {dayActs.length === 0 && <li className="text-sm text-gray-400">这一天暂无活动</li>}
              </ul>
            </section>
          )
        })}

        <button onClick={onSubmit} disabled={edit.isPending} className="w-full bg-brand-500 hover:bg-brand-600 text-white rounded-lg py-2.5 font-medium disabled:opacity-60">
          {edit.isPending ? '正在保存…' : '保存并生成新版本'}
        </button>
      </div>

      {pickerFor && (
        <PlacePicker
          defaultCity={city}
          onClose={() => setPickerFor(null)}
          onPick={(p: PlaceFull) => { patch(pickerFor, { place_id: p.id, place_name: p.name }); setPickerFor(null) }}
        />
      )}
    </div>
  )
}
