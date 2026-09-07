import { useState } from 'react'
import Header from '../../components/Header'
import { useSearchPlaces } from '../../api/places'
import { minuteToClock } from '../../lib/time'
import { centsToYuan } from '../../lib/money'
import type { PlaceFull } from '../../types/api'

function feeText(p: PlaceFull): string {
  if (p.fee_cents == null) return '门票待确认'
  if (p.fee_cents === 0) return '免费'
  return `¥${centsToYuan(p.fee_cents)}`
}
function hoursText(p: PlaceFull): string | null {
  if (p.open_minute == null || p.close_minute == null) return null
  return `${minuteToClock(p.open_minute)}–${minuteToClock(p.close_minute)}`
}

export function PlaceCard({ p }: { p: PlaceFull }) {
  const hours = hoursText(p)
  return (
    <div className="rounded-2xl border border-gray-100 p-4 hover:shadow-md transition">
      <div className="flex items-start justify-between gap-2">
        <span className="font-medium text-gray-900">{p.name}</span>
        {!p.active && <span className="shrink-0 rounded-full bg-gray-100 text-gray-500 text-xs px-2 py-0.5">已下架</span>}
      </div>
      {p.address && <p className="mt-1 text-sm text-gray-500 line-clamp-2">📍 {p.address}</p>}
      <div className="mt-2 flex flex-wrap gap-2 text-xs">
        <span className="rounded bg-brand-50 text-brand-600 px-2 py-0.5">{feeText(p)}</span>
        <span className="rounded bg-gray-50 text-gray-500 px-2 py-0.5">游玩约 {p.duration_minutes} 分钟</span>
        {hours && <span className="rounded bg-gray-50 text-gray-500 px-2 py-0.5">{hours}</span>}
      </div>
      {p.tags.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {p.tags.map((t) => <span key={t} className="rounded-full border border-gray-200 text-gray-500 text-xs px-2 py-0.5">{t}</span>)}
        </div>
      )}
    </div>
  )
}

export default function PlacesPage() {
  const [city, setCity] = useState('')
  const [keyword, setKeyword] = useState('')
  const [query, setQuery] = useState<{ city: string; keyword: string }>({ city: '', keyword: '' })
  const [page, setPage] = useState(1)

  const { data, isFetching, error } = useSearchPlaces({ city: query.city, keyword: query.keyword, page }, query.city !== '')

  const onSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setPage(1)
    setQuery({ city: city.trim(), keyword: keyword.trim() })
  }

  return (
    <div className="min-h-screen bg-white font-sans">
      <Header />
      <main className="pt-16 mx-auto max-w-container px-4 py-8">
        <h1 className="text-2xl font-serif text-gray-900 mb-2">发现地点</h1>
        <p className="text-gray-500 mb-6">按城市搜索景点、餐饮与游玩地，作为你规划行程的灵感来源。</p>

        <form onSubmit={onSearch} className="flex flex-col sm:flex-row gap-3 mb-8">
          <input
            value={city}
            onChange={(e) => setCity(e.target.value)}
            placeholder="城市（必填，如 杭州市）"
            className="flex-1 border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="关键词（可选，如 西湖 / 博物馆）"
            className="flex-1 border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <button
            type="submit"
            disabled={!city.trim()}
            className="rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-6 py-2 text-sm font-medium disabled:opacity-50"
          >
            搜索
          </button>
        </form>

        {error && <p className="text-red-600">搜索失败：{error instanceof Error ? error.message : '未知错误'}</p>}
        {isFetching && <p className="text-gray-500">正在搜索…</p>}

        {data && !isFetching && (
          data.items.length === 0 ? (
            <p className="text-gray-500 py-10 text-center">没有找到相关地点，换个关键词试试。</p>
          ) : (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {data.items.map((p) => <PlaceCard key={p.id} p={p} />)}
              </div>
              <div className="mt-8 flex items-center justify-center gap-3">
                <button
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page <= 1}
                  className="rounded-lg border border-gray-200 text-gray-600 px-4 py-2 text-sm disabled:opacity-50 hover:border-gray-300"
                >
                  上一页
                </button>
                <span className="text-sm text-gray-500">第 {page} 页</span>
                <button
                  onClick={() => setPage((p) => p + 1)}
                  disabled={!data.has_more}
                  className="rounded-lg border border-gray-200 text-gray-600 px-4 py-2 text-sm disabled:opacity-50 hover:border-gray-300"
                >
                  下一页
                </button>
              </div>
            </>
          )
        )}
      </main>
    </div>
  )
}
