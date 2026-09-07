import { useEffect, useRef } from 'react'
import type { Activity, Route, Cost } from '../../types/api'
import type { PointMeta } from './TripMap'
import { minuteToClock } from '../../lib/time'
import { centsToYuan } from '../../lib/money'

function fmtDistance(m: number): string {
  return m >= 1000 ? `${(m / 1000).toFixed(1)} km` : `${m} m`
}
function fmtDuration(s: number): string {
  const min = Math.round(s / 60)
  return min >= 1 ? `${min} 分钟` : `${s} 秒`
}
function costText(c: Cost): string {
  const amount = c.amount_cents == null ? '待确认' : `¥${centsToYuan(c.amount_cents)}`
  return `${c.category} ${amount}`
}

function ModeBadge({ mode }: { mode: Route['mode'] }) {
  const isTransit = mode === 'transit'
  return (
    <span className={`inline-block rounded-full px-2 py-0.5 text-xs ${isTransit ? 'bg-brand-50 text-brand-600' : 'border border-dashed border-gray-300 text-gray-500'}`}>
      {isTransit ? '公共交通' : '步行'}
    </span>
  )
}

interface TimelineProps {
  activities: Activity[]
  routes: Route[]
  pointMeta: Map<string, PointMeta>
  selectedId?: string | null
  onSelect?: (id: string) => void
}

export default function Timeline({ activities, routes, pointMeta, selectedId, onSelect }: TimelineProps) {
  // 反向联动：地图 marker 被选中时，把时间线上对应项滚动到可视区
  const itemRefs = useRef<Map<string, HTMLLIElement>>(new Map())
  useEffect(() => {
    if (!selectedId) return
    const el = itemRefs.current.get(selectedId)
    el?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [selectedId])

  const byDate = new Map<string, Activity[]>()
  for (const a of activities) {
    const arr = byDate.get(a.date) ?? []
    arr.push(a)
    byDate.set(a.date, arr)
  }
  const dates = [...byDate.keys()].sort()

  const findRoute = (fromId: string, toId: string) =>
    routes.find((r) => r.from_item_id === fromId && r.to_item_id === toId)

  return (
    <div className="space-y-8">
      {dates.map((date) => {
        const dayActs = (byDate.get(date) ?? []).slice().sort((a, b) => a.start_minute - b.start_minute)
        return (
          <section key={date}>
            <h3 className="text-sm font-medium text-gray-900 mb-3">{date}</h3>
            <ol className="relative border-l border-gray-100 pl-4 space-y-4">
              {dayActs.map((a, idx) => {
                const next = dayActs[idx + 1]
                const leg = next ? findRoute(a.id, next.id) : undefined
                const selected = a.id === selectedId
                const meta = pointMeta.get(a.id)
                const locatable = !!a.place?.location
                return (
                  <li
                    key={a.id}
                    ref={(el) => {
                      if (el) itemRefs.current.set(a.id, el)
                      else itemRefs.current.delete(a.id)
                    }}
                  >
                    <button
                      type="button"
                      onClick={() => onSelect?.(a.id)}
                      aria-pressed={selected}
                      className={`w-full text-left rounded-xl -ml-2 px-2 py-2 transition ${
                        selected ? 'bg-brand-50 ring-1 ring-brand-200' : 'hover:bg-gray-50'
                      }`}
                    >
                      <div className="flex items-center gap-2 text-xs text-gray-400">
                        {meta ? (
                          <span
                            className="inline-flex items-center justify-center w-4 h-4 rounded-full text-[10px] font-semibold text-white shrink-0"
                            style={{ backgroundColor: meta.color }}
                          >
                            {meta.num}
                          </span>
                        ) : (
                          <span className="inline-block w-1.5 h-1.5 rounded-full bg-gray-300" />
                        )}
                        {minuteToClock(a.start_minute)}–{minuteToClock(a.end_minute)}
                        {locatable && <span className="ml-auto text-brand-500">在地图上查看 →</span>}
                      </div>
                      <div className={`mt-0.5 font-medium ${selected ? 'text-brand-700' : 'text-gray-900'}`}>{a.title}</div>
                      {a.place?.name && <div className="text-sm text-gray-500">📍 {a.place.name}</div>}
                      {a.reason && <div className="text-sm text-gray-500 mt-1">{a.reason}</div>}
                      {a.costs && a.costs.length > 0 && (
                        <div className="mt-1 flex flex-wrap gap-2 text-xs text-gray-500">
                          {a.costs.map((c, i) => <span key={i} className="rounded bg-gray-50 px-2 py-0.5">{costText(c)}</span>)}
                        </div>
                      )}
                    </button>
                    {leg && (
                      <div className="mt-2 ml-2 flex items-center gap-2 text-xs text-gray-500">
                        <ModeBadge mode={leg.mode} />
                        <span>{fmtDistance(leg.distance_m)} · {fmtDuration(leg.duration_s)}</span>
                        {leg.summary && <span className="text-gray-400">· {leg.summary}</span>}
                      </div>
                    )}
                  </li>
                )
              })}
            </ol>
          </section>
        )
      })}
    </div>
  )
}
