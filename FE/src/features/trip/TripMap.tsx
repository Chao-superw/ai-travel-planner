import { useEffect, useRef, useState } from 'react'
import type { Activity, Route } from '../../types/api'
import { loadAmap, parseGcj02, parsePolyline } from '../../lib/amap'
import { minuteToClock } from '../../lib/time'
import { centsToYuan } from '../../lib/money'

// 点位元信息：由 TripPage 统一计算，保证地图 marker 与时间线徽标编号/配色一致
export interface PointMeta { num: number; color: string; dayIndex: number }

interface Props {
  activities: Activity[]
  routes: Route[]
  pointMeta: Map<string, PointMeta>
  selectedId?: string | null
  onSelect?: (id: string | null) => void
}

// 数字圆点 marker（选中时放大、其余保持常态）
function dotHtml(num: number, color: string, selected: boolean) {
  const s = selected ? 30 : 22
  return `<div style="width:${s}px;height:${s}px;border-radius:9999px;background:${color};color:#fff;border:2px solid #fff;box-shadow:0 1px 4px rgba(0,0,0,.3);display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:600;font-family:system-ui">${num}</div>`
}

// 点击某个点后展示的详情卡片
function infoHtml(a: Activity) {
  const time = `${minuteToClock(a.start_minute)}–${minuteToClock(a.end_minute)}`
  const place = a.place?.name ? `<div style="color:#6b7280;font-size:12px;margin-top:2px">📍 ${a.place.name}</div>` : ''
  const reason = a.reason ? `<div style="color:#9ca3af;font-size:12px;margin-top:4px;line-height:1.4">${a.reason}</div>` : ''
  const cost = (a.costs ?? [])
    .map((c) => `${c.category} ${c.amount_cents == null ? '待确认' : '¥' + centsToYuan(c.amount_cents)}`)
    .join(' · ')
  const costLine = cost ? `<div style="color:#9ca3af;font-size:12px;margin-top:4px">${cost}</div>` : ''
  return `<div style="padding:8px 10px;max-width:220px;font-family:system-ui">
    <div style="font-size:11px;color:#9ca3af">${time}</div>
    <div style="font-weight:600;color:#111827;font-size:13px;margin-top:2px">${a.title}</div>
    ${place}${reason}${costLine}
  </div>`
}

export default function TripMap({ activities, routes, pointMeta, selectedId, onSelect }: Props) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [failed, setFailed] = useState(false)
  // 地图异步加载完成的标记：用来在 ready 后补跑一次选中联动，避免“初始选中但卡片/聚焦未应用”
  const [ready, setReady] = useState(false)

  // 保存实例与引用，供选中态联动使用
  const mapRef = useRef<any>(null)
  const AMapRef = useRef<any>(null)
  const infoRef = useRef<any>(null)
  const markersRef = useRef<Map<string, { marker: any; activity: Activity; meta: PointMeta }>>(new Map())
  const linesRef = useRef<Array<{ date: string; from: string; to: string; line: any; isTransit: boolean }>>([])
  const onSelectRef = useRef(onSelect)
  onSelectRef.current = onSelect

  // 初始化地图 + 常态画出所有点位/折线（activities/routes/pointMeta 变化时重建）
  useEffect(() => {
    let cancelled = false
    const key = import.meta.env.VITE_AMAP_KEY ?? ''

    loadAmap(key)
      .then((AMap) => {
        if (cancelled || !containerRef.current) return
        AMapRef.current = AMap
        const map = new AMap.Map(containerRef.current, { zoom: 12, resizeEnable: true })
        mapRef.current = map
        infoRef.current = new AMap.InfoWindow({ offset: new AMap.Pixel(0, -16), isCustom: false })

        // 常态：所有有坐标的活动都画数字点
        for (const a of activities) {
          const loc = a.place?.location
          const meta = pointMeta.get(a.id)
          if (!loc || !meta) continue
          const [lng, lat] = parseGcj02(loc)
          if (Number.isNaN(lng) || Number.isNaN(lat)) continue
          const marker = new AMap.Marker({
            position: [lng, lat],
            anchor: 'center',
            content: dotHtml(meta.num, meta.color, false),
            zIndex: 100,
          })
          marker.on('click', () => onSelectRef.current?.(a.id))
          map.add(marker)
          markersRef.current.set(a.id, { marker, activity: a, meta })
        }

        // 路线折线（按 mode 着色）
        for (const r of routes) {
          const path = parsePolyline(r.polyline)
          if (path.length < 2) continue
          const isTransit = r.mode === 'transit'
          const line = new AMap.Polyline({
            path,
            strokeColor: isTransit ? '#F97316' : '#9CA3AF',
            strokeWeight: isTransit ? 5 : 4,
            strokeStyle: isTransit ? 'solid' : 'dashed',
            strokeOpacity: 0.5,
          })
          map.add(line)
          linesRef.current.push({ date: r.date, from: r.from_item_id, to: r.to_item_id, line, isTransit })
        }

        map.setFitView()
        // 点击空白处取消选中，关闭详情卡片
        map.on('click', () => onSelectRef.current?.(null))
        setReady(true)
      })
      .catch(() => {
        if (!cancelled) setFailed(true)
      })

    return () => {
      cancelled = true
      setReady(false)
      markersRef.current.clear()
      linesRef.current = []
      infoRef.current = null
      if (mapRef.current) { mapRef.current.destroy?.(); mapRef.current = null }
    }
  }, [activities, routes, pointMeta])

  // 选中态联动：按天筛选显示、聚焦放大、弹出详情
  useEffect(() => {
    const map = mapRef.current
    const AMap = AMapRef.current
    if (!map || !AMap) return

    // 命中“有坐标的点”才进入单日聚焦；命中空点(餐饮/住宿)或未选中 → 回到全览
    const hit = selectedId != null ? markersRef.current.get(selectedId) : undefined
    const focusDate = hit?.activity.date ?? null

    // 点位：单日聚焦时只显示当天的点，其余隐藏；全览时全部显示
    for (const [id, { marker, activity, meta }] of markersRef.current) {
      if (focusDate == null || activity.date === focusDate) marker.show?.()
      else marker.hide?.()
      const sel = id === selectedId
      marker.setContent?.(dotHtml(meta.num, meta.color, sel))
      marker.setzIndex?.(sel ? 300 : 100)
    }

    // 路线：同样按天显隐，选中点相邻段加粗提亮
    for (const { date, from, to, line, isTransit } of linesRef.current) {
      if (focusDate == null || date === focusDate) line.show?.()
      else line.hide?.()
      const active = selectedId != null && (from === selectedId || to === selectedId)
      line.setOptions?.({
        strokeColor: active ? '#F97316' : isTransit ? '#F97316' : '#9CA3AF',
        strokeWeight: active ? (isTransit ? 7 : 6) : isTransit ? 5 : 4,
        strokeOpacity: active ? 1 : 0.5,
      })
    }

    if (hit) {
      // 聚焦某个点：平移 + 放大到街区级，并弹出详情
      const pos = hit.marker.getPosition?.()
      if (pos) map.setZoomAndCenter?.(15, pos, false, 300)
      infoRef.current?.setContent?.(infoHtml(hit.activity))
      if (pos) infoRef.current?.open?.(map, pos)
    } else {
      // 未选中 or 选中空点：关闭详情、回到全览
      infoRef.current?.close?.()
      map.setFitView?.()
    }
  }, [selectedId, ready])

  if (failed) {
    return (
      <div className="h-80 rounded-2xl bg-gray-50 border border-gray-100 flex items-center justify-center text-center text-gray-400 text-sm px-6">
        地图不可用，可参考左侧文字路线
      </div>
    )
  }

  return <div ref={containerRef} className="h-80 rounded-2xl overflow-hidden border border-gray-100 bg-gray-50" />
}
