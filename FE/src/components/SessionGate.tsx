import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useMe } from '../api/me'
import { useLogout } from '../api/auth'
import { useAuth } from '../store/auth'
import { returnPath } from '../features/auth/authForm'

export default function SessionGate({ children }: { children: ReactNode }) {
  const { token } = useAuth()
  const me = useMe()
  const logout = useLogout()
  const loc = useLocation()
  if (token) {
    if (me.isPending) return <div role="status" className="min-h-screen grid place-items-center text-gray-500">正在确认登录状态…</div>
    if (me.isError) return <main className="min-h-screen flex flex-col gap-4 items-center justify-center px-5 text-center">
      <p role="alert" className="text-gray-600">暂时无法确认登录状态，请稍后重试。</p>
      <button onClick={() => me.refetch()} disabled={me.isFetching} className="text-brand-600">{me.isFetching ? '重试中…' : '重新连接'}</button>
      <button onClick={() => logout.mutate()} disabled={logout.isPending} className="text-sm text-gray-500">退出并重新登录</button>
    </main>
    if (!me.data?.email_verified && loc.pathname !== '/bind-email') return <Navigate to="/bind-email" replace state={{ from: returnPath(loc.state?.from ?? loc.pathname + loc.search + loc.hash) }} />
  }
  return <>{children}</>
}
