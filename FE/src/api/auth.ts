import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { saveSession, saveUser, clearSession, loadToken } from '../store/auth'
import type { EmailChallenge, Session, User } from '../types/api'

interface Credentials { email: string; password: string }
interface EmailProof { email: string; code: string; challenge_id: string }

export function useLogin() {
  const client = useQueryClient()
  return useMutation({
    retry: false,
    mutationFn: async (c: Credentials) => (await apiRequest<Session>('/api/v1/auth/login', {
      method: 'POST', body: c,
    })).data,
    onSuccess: (s) => {
      // Account switching must not reuse the previous user's private query cache.
      client.removeQueries()
      client.setQueryData(['me', s.token], s.user)
      saveSession(s)
    },
  })
}
export function requestEmailCode(email: string, purpose: 'register' | 'bind') {
  return apiRequest<EmailChallenge>(purpose === 'register' ? '/api/v1/auth/register/code' : '/api/v1/me/email/code', {
    method: 'POST', body: { email }, token: purpose === 'bind' ? loadToken() : undefined,
  }).then(r => r.data)
}
export function useRegister() {
  return useMutation({
    retry: false,
    mutationFn: async (c: EmailProof & { password: string }) => (await apiRequest<User>('/api/v1/auth/register', { method: 'POST', body: c })).data,
  })
}
export function useBindEmail() {
  const client = useQueryClient()
  return useMutation({
    retry: false,
    mutationFn: async (proof: EmailProof) => {
      const token = loadToken()
      const user = (await apiRequest<User>('/api/v1/me/email', { method: 'POST', body: proof, token })).data
      return { user, token }
    },
    onSuccess: async ({ user, token }) => {
      await client.cancelQueries({ queryKey: ['me', token], exact: true })
      if (token && loadToken() === token) {
        client.setQueryData(['me', token], user)
        saveUser(user)
      }
    },
  })
}
export function useLogout() {
  const client = useQueryClient()
  return useMutation({
    retry: false,
    mutationFn: async () => {
      const token = loadToken()
      try {
        await apiRequest<{ logged_out: boolean }>('/api/v1/auth/logout', { method: 'POST', token })
      } finally {
        const current = loadToken()
        if (!current || current === token) { clearSession(); client.removeQueries() }
      }
    },
  })
}
