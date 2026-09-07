import { useMemo, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import Header from '../../components/Header'
import Timeline from './Timeline'
import BudgetPanel from './BudgetPanel'
import TripMap, { type PointMeta } from './TripMap'
import ReplanModal from './ReplanModal'
import ManualEditor from './ManualEditor'
import VersionsPanel from './VersionsPanel'
import { useTrip, useTripBudget } from '../../api/trips'
import { ApiError } from '../../lib/errors'

// 每天一种配色，点在地图/时间线上按“天内先后顺序”编号，保证两处徽标一致
const DAY_COLORS = ['#F43F5E', '#3B82F6', '#10B981', '#F59E0B', '#8B5CF6', '#0EA5E9', '#EC4899']

type Panel = 'replan' | 'edit' | 'versions' | null

export default function TripPage() {
  const { id = '' } = useParams()
  const nav = useNavigate()
  const trip = useTrip(id)
  const budget = useTripBudget(id)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [panel, setPanel] = useState<Panel>(null)

  const notFound = trip.error instanceof ApiError && trip.error.status === 404

  const activities = trip.data?.plan.activities

  // 计算点位元信息：按天分组、天内按开始时间排序，仅给有坐标的活动编号（与地图 marker 对应）
  const pointMeta = useMemo(() => {
    const meta = new Map<string, PointMeta>()
    if (!activities) return meta
    const dates = [...new Set(activities.map((a) => a.date))].sort()
    dates.forEach((date, dayIndex) => {
      const color = DAY_COLORS[dayIndex % DAY_COLORS.length]
      const dayActs = activities
        .filter((a) => a.date === date)
        .slice()
        .sort((a, b) => a.start_minute - b.start_minute)
      let num = 0
      for (const a of dayActs) {
        if (!a.place?.location) continue
        num += 1
        meta.set(a.id, { num, color, dayIndex })
      }
    })
    return meta
  }, [activities])

  return (
    <div className="min-h-screen bg-white font-sans">
      <Header />
      <main className="pt-16 mx-auto max-w-container px-4 py-8">
        {trip.isLoading && <p className="text-gray-500">正在加载行程…</p>}

        {notFound && (
          <div className="py-16 text-center">
            <p className="text-gray-600">行程不存在或已被删除。</p>
            <button onClick={() => nav('/trips')} className="mt-4 rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-5 py-2.5">返回我的行程</button>
          </div>
        )}

        {trip.error && !notFound && (
          <p className="text-red-600">加载失败：{trip.error instanceof Error ? trip.error.message : '未知错误'}</p>
        )}

        {trip.data && (
          <>
            <header className="mb-6 flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
              <div>
                <h1 className="text-2xl font-serif text-gray-900">{trip.data.plan.title}</h1>
                <p className="mt-2 text-gray-500">{trip.data.plan.summary}</p>
                <p className="mt-1 text-xs text-gray-400">当前版本 v{trip.data.version}</p>
              </div>
              <div className="flex flex-wrap gap-2 shrink-0">
                <button onClick={() => setPanel('replan')} className="rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-4 py-2 text-sm">调整行程</button>
                <button onClick={() => setPanel('edit')} className="rounded-lg border border-gray-200 text-gray-600 px-4 py-2 text-sm hover:border-gray-300">手动编辑</button>
                <button onClick={() => setPanel('versions')} className="rounded-lg border border-gray-200 text-gray-600 px-4 py-2 text-sm hover:border-gray-300">版本历史</button>
              </div>
            </header>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
              <div className="lg:col-span-2 space-y-6">
                <div className="lg:sticky lg:top-20 z-10 bg-white pb-2">
                  <TripMap
                    activities={trip.data.plan.activities}
                    routes={trip.data.plan.routes}
                    pointMeta={pointMeta}
                    selectedId={selectedId}
                    onSelect={setSelectedId}
                  />
                </div>
                <Timeline
                  activities={trip.data.plan.activities}
                  routes={trip.data.plan.routes}
                  pointMeta={pointMeta}
                  selectedId={selectedId}
                  onSelect={setSelectedId}
                />
              </div>
              <div className="lg:sticky lg:top-20 self-start">
                {budget.data
                  ? <BudgetPanel budget={budget.data} warnings={trip.data.plan.warnings} />
                  : <div className="rounded-2xl border border-gray-100 p-5 text-gray-400 text-sm">预算加载中…</div>}
              </div>
            </div>

            {panel === 'replan' && (
              <ReplanModal
                tripId={id}
                version={trip.data.version}
                activities={trip.data.plan.activities}
                onClose={() => setPanel(null)}
              />
            )}
            {panel === 'edit' && (
              <ManualEditor
                tripId={id}
                version={trip.data.version}
                plan={trip.data.plan}
                city={trip.data.constraints.city}
                onClose={() => setPanel(null)}
              />
            )}
            {panel === 'versions' && (
              <VersionsPanel
                tripId={id}
                currentVersion={trip.data.version}
                onClose={() => setPanel(null)}
              />
            )}
          </>
        )}
      </main>
    </div>
  )
}
