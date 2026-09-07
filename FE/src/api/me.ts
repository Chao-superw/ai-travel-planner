import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { ApiError } from '../lib/errors'
import { useAuth, loadToken, saveUser, clearSession } from '../store/auth'
import type { User } from '../types/api'

export function useMe(enabled = true) {
  const { token } = useAuth()
  const q = useQuery({
    queryKey: ['me', token],
    enabled: enabled && !!token,
    retry: false,
    staleTime: 5 * 60 * 1000,
    queryFn: async () => (await apiRequest<User>('/api/v1/me', { token })).data,
  })
  useEffect(() => {
    if (q.data && token && token === loadToken()) saveUser(q.data)
  }, [q.data, token])
  useEffect(() => {
    if (q.error instanceof ApiError && q.error.status === 401 && token === loadToken()) clearSession()
  }, [q.error, token])
  return q
}
