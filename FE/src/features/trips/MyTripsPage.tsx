import { Link } from 'react-router-dom'
import Header from '../../components/Header'
import { useMyTrips } from '../../api/trips'
import { useState } from 'react'

function dayCount(start: string, end: string): number {
  const d = (new Date(end).getTime() - new Date(start).getTime()) / 86400000 + 1
  return Number.isFinite(d) && d >= 1 ? Math.round(d) : 1
}

function fmtDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export default function MyTripsPage() {
  const [page, setPage] = useState(1)
  const { data, isLoading, error } = useMyTrips(page)

  return (
    <div className="min-h-screen bg-white font-sans">
      <Header />
      <main className="pt-16 mx-auto max-w-container px-4 py-8">
        <h1 className="text-2xl font-serif text-gray-900 mb-6">我的行程</h1>

        {isLoading && <p className="text-gray-500">正在加载…</p>}

        {error && (
          <p className="text-red-600">加载失败：{error instanceof Error ? error.message : '未知错误'}</p>
        )}

        {data && data.items.length === 0 && (
          <div className="py-16 text-center">
            <p className="text-gray-600">还没有行程，去首页生成一条吧。</p>
            <Link to="/" className="inline-block mt-4 rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-5 py-2.5">去首页规划</Link>
          </div>
        )}

        {data && data.items.length > 0 && (
          <>
            <ul className="grid grid-cols-1 sm:grid-cols-2 gap-5">
              {data.items.map((t) => (
                <li key={t.id}>
                  <Link
                    to={`/trips/${t.id}`}
                    className="block rounded-2xl border border-gray-100 p-5 hover:shadow-lg transition"
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-medium text-gray-900">{t.title}</span>
                      <span className="text-xs text-gray-400">{fmtDate(t.created_at)}</span>
                    </div>
                    <p className="mt-2 text-sm text-gray-500 line-clamp-2">{t.summary}</p>
                    <div className="mt-3 text-sm text-brand-600">
                      {t.constraints.city} · {dayCount(t.constraints.start_date, t.constraints.end_date)} 天
                    </div>
                  </Link>
                </li>
              ))}
            </ul>

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
        )}
      </main>
    </div>
  )
}
