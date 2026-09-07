import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useReplan } from '../../api/trips'
import { newIdempotencyKey } from '../../lib/idempotency'
import { minuteToClock } from '../../lib/time'
import { ApiError } from '../../lib/errors'
import type { Activity, ReplanInput } from '../../types/api'

// 自然语言重规划：选一天 + 勾选允许 AI 改动的活动 + 填写指令。scope 窗口取整天(0–1440)。
export default function ReplanModal({
  tripId,
  version,
  activities,
  onClose,
}: {
  tripId: string
  version: number
  activities: Activity[]
  onClose: () => void
}) {
  const nav = useNavigate()
  const replan = useReplan(tripId)

  const dates = useMemo(() => [...new Set(activities.map((a) => a.date))].sort(), [activities])
  const firstDate = dates[0] ?? ''
  const [date, setDate] = useState(firstDate)
  const dayActs = useMemo(
    () => activities.filter((a) => a.date === date).sort((a, b) => a.start_minute - b.start_minute),
    [activities, date],
  )
  // 默认全选首日活动为可修改
  const [editable, setEditable] = useState<Set<string>>(
    () => new Set(activities.filter((a) => a.date === firstDate).map((a) => a.id)),
  )
  const [instruction, setInstruction] = useState('')
  const [topError, setTopError] = useState<string | null>(null)

  // 切换日期时，默认全选当天活动为可修改
  const pickDate = (d: string) => {
    setDate(d)
    setEditable(new Set(activities.filter((a) => a.date === d).map((a) => a.id)))
  }

  const toggle = (id: string) => {
    setEditable((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const onSubmit = () => {
    setTopError(null)
    const ids = [...editable]
    if (!date) { setTopError('请选择要调整的日期'); return }
    if (ids.length === 0) { setTopError('请至少勾选一个可调整的活动'); return }
    const text = instruction.trim()
    if (text.length < 1 || text.length > 2000) { setTopError('修改要求需为 1–2000 字'); return }

    const input: ReplanInput = {
      expected_version: version,
      scope: { date, start_minute: 0, end_minute: 1440, editable_item_ids: ids, locked_item_ids: [] },
      instruction: text,
    }
    replan.mutate(
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

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 px-4" onClick={onClose}>
      <div className="w-full max-w-lg bg-white rounded-2xl shadow-xl p-6 font-sans max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-medium text-gray-900">让 AI 帮你调整行程</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600" aria-label="关闭">✕</button>
        </div>

        {topError && <div className="mb-3 rounded-lg bg-red-50 text-red-600 text-sm px-3 py-2">{topError}</div>}

        <label className="text-sm text-gray-700">选择要调整的日期</label>
        <div className="mt-2 flex flex-wrap gap-2">
          {dates.map((d) => (
            <button
              key={d}
              type="button"
              onClick={() => pickDate(d)}
              className={`rounded-full border px-3 py-1 text-sm transition ${date === d ? 'border-brand-500 text-brand-600 bg-brand-50' : 'border-gray-200 text-gray-600 hover:border-gray-300'}`}
            >
              {d}
            </button>
          ))}
        </div>

        <div className="mt-4">
          <label className="text-sm text-gray-700">允许 AI 改动的活动（不勾选的将尽量保留）</label>
          <ul className="mt-2 space-y-1.5">
            {dayActs.map((a) => (
              <li key={a.id}>
                <label className="flex items-center gap-2 text-sm text-gray-700 rounded-lg px-2 py-1.5 hover:bg-gray-50">
                  <input type="checkbox" checked={editable.has(a.id)} onChange={() => toggle(a.id)} />
                  <span className="text-gray-400 text-xs w-24 shrink-0">{minuteToClock(a.start_minute)}–{minuteToClock(a.end_minute)}</span>
                  <span className="truncate">{a.title}</span>
                </label>
              </li>
            ))}
            {dayActs.length === 0 && <li className="text-sm text-gray-400">这一天没有活动</li>}
          </ul>
        </div>

        <div className="mt-4">
          <label className="text-sm text-gray-700">修改要求（1–2000 字）</label>
          <textarea
            value={instruction}
            onChange={(e) => setInstruction(e.target.value)}
            rows={4}
            placeholder="例如：下午想去博物馆，午餐换成本地小吃，整体节奏放慢一点。"
            className="mt-1 w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <div className="mt-1 text-right text-xs text-gray-400">{instruction.trim().length}/2000</div>
        </div>

        <button
          onClick={onSubmit}
          disabled={replan.isPending}
          className="mt-2 w-full bg-brand-500 hover:bg-brand-600 text-white rounded-lg py-2.5 font-medium disabled:opacity-60"
        >
          {replan.isPending ? '正在提交…' : '提交并重新规划'}
        </button>
      </div>
    </div>
  )
}
