import { useSyncExternalStore } from 'react'
import type { Session, User } from '../types/api'

const TOKEN_KEY = 'wandr.token'
const USER_KEY = 'wandr.user'
let _cachedUserRaw: string | null | undefined = undefined
let _cachedUser: User | null = null
const listeners = new Set<() => void>()
function emit() { listeners.forEach((l) => l()) }

export function loadToken(): string | null { return localStorage.getItem(TOKEN_KEY) }
export function loadUser(): User | null {
  const raw = localStorage.getItem(USER_KEY)
  if (raw !== _cachedUserRaw) {
    _cachedUserRaw = raw
    try {
      _cachedUser = raw ? (JSON.parse(raw) as User) : null
    } catch {
      _cachedUser = null
    }
  }
  return _cachedUser
}
export function saveSession(s: Session) {
  localStorage.setItem(TOKEN_KEY, s.token)
  localStorage.setItem(USER_KEY, JSON.stringify(s.user))
  emit()
}
export function saveUser(u: User) {
  localStorage.setItem(USER_KEY, JSON.stringify(u))
  emit()
}
export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  emit()
}

function subscribe(cb: () => void) { listeners.add(cb); return () => listeners.delete(cb) }

export function useAuth() {
  const token = useSyncExternalStore(subscribe, loadToken)
  const user = useSyncExternalStore(subscribe, loadUser)
  return { token, user, isAuthed: !!token, login: saveSession, logout: clearSession }
}
