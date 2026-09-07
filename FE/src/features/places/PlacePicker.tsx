import { useState } from 'react'
import { useSearchPlaces } from '../../api/places'
import type { PlaceFull } from '../../types/api'

// 地点选择器：按城市搜索并回选一个地点（返回其 id 与展示名）。用于手动编辑挂载 place。
export default function PlacePicker({
  defaultCity,
  onPick,
  onClose,
}: {
  defaultCity?: string
  onPick: (place: PlaceFull) => void
  onClose: () => void
}) {
  const [city, setCity] = useState(defaultCity ?? '')
  const [keyword, setKeyword] = useState('')
  const [query, setQuery] = useState<{ city: string; keyword: string }>({ city: defaultCity ?? '', keyword: '' })

  const { data, isFetching, error } = useSearchPlaces({ city: query.city, keyword: query.keyword, pageSize: 25 }, query.city !== '')

  const onSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setQuery({ city: city.trim(), keyword: keyword.trim() })
  }

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/40 px-4" onClick={onClose}>
      <div className="w-full max-w-md bg-white rounded-2xl shadow-xl p-5 font-sans max-h-[85vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-base font-medium text-gray-900">选择地点</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600" aria-label="关闭">✕</button>
        </div>

        <form onSubmit={onSearch} className="flex gap-2 mb-3">
          <input
            value={city}
            onChange={(e) => setCity(e.target.value)}
            placeholder="城市"
            className="w-28 border border-gray-200 rounded-lg px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="关键词"
            className="flex-1 border border-gray-200 rounded-lg px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <button type="submit" disabled={!city.trim()} className="rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-3 py-1.5 text-sm disabled:opacity-50">搜</button>
        </form>

        {error && <p className="text-sm text-red-600">搜索失败：{error instanceof Error ? error.message : '未知错误'}</p>}
        {isFetching && <p className="text-sm text-gray-500">正在搜索…</p>}

        {data && !isFetching && (
          data.items.length === 0
            ? <p className="text-sm text-gray-500 py-6 text-center">没有结果</p>
            : (
              <ul className="space-y-2">
                {data.items.map((p) => (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => onPick(p)}
                      className="w-full text-left rounded-xl border border-gray-100 px-3 py-2 hover:bg-gray-50 transition"
                    >
                      <div className="font-medium text-gray-900 text-sm">{p.name}</div>
                      {p.address && <div className="text-xs text-gray-500 line-clamp-1">{p.address}</div>}
                    </button>
                  </li>
                ))}
              </ul>
            )
        )}
      </div>
    </div>
  )
}
