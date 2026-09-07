import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../store/auth'

export default function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthed } = useAuth()
  const loc = useLocation()
  if (!isAuthed) return <Navigate to="/login" state={{ from: loc.pathname + loc.search + loc.hash }} replace />
  return <>{children}</>
}
