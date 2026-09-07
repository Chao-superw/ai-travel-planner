import { useEffect, useState } from 'react'

export function useCountdown(until: number) {
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    if (!until) return
    const update = () => setNow(Date.now())
    const firstTick = window.setTimeout(update, 0)
    const timer = window.setInterval(update, 1000)
    document.addEventListener('visibilitychange', update)
    return () => {
      window.clearTimeout(firstTick)
      window.clearInterval(timer)
      document.removeEventListener('visibilitychange', update)
    }
  }, [until])
  return Math.max(0, Math.ceil((until - now) / 1000))
}
