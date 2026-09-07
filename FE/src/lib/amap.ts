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
