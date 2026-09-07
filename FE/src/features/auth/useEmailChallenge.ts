import { useEffect, useRef, useState } from 'react'
import { requestEmailCode } from '../../api/auth'
import { ApiError } from '../../lib/errors'
import type { EmailChallenge } from '../../types/api'
import { authError, normalizedEmail } from './authForm'
import { useCountdown } from './useCountdown'

type Proof = EmailChallenge & { email: string; expiresAt: number }
export function useEmailChallenge(email: string, purpose: 'register' | 'bind') {
  const [proof, setProof] = useState<Proof | null>(null)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [resendAt, setResendAt] = useState(0)
  const currentEmail = normalizedEmail(email)
  const latestEmail = useRef(currentEmail)
  const sendingRef = useRef(false)
  const mounted = useRef(true)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  useEffect(() => { latestEmail.current = currentEmail }, [currentEmail])
  const expiresIn = useCountdown(proof?.expiresAt ?? 0)
  const resendIn = useCountdown(resendAt)

  async function send(address: string) {
    if (sendingRef.current || resendIn > 0) return
    sendingRef.current = true; setSending(true); setError('')
    try {
      const challenge = await requestEmailCode(address, purpose)
      if (!mounted.current) return
      setResendAt(Date.now() + challenge.retry_after * 1000)
      if (address === latestEmail.current) {
        setProof({ ...challenge, email: address, expiresAt: Date.now() + challenge.expires_in * 1000 })
        return true
      }
    } catch (e) {
      if (!mounted.current) return
      const seconds = e instanceof ApiError ? e.retryAfter : undefined
      if (seconds || (e instanceof ApiError && e.code === 'MAIL_UNAVAILABLE')) setResendAt(Date.now() + (seconds ?? proof?.retry_after ?? 60) * 1000)
      if (address === latestEmail.current) setError(authError(e))
    } finally {
      sendingRef.current = false
      if (mounted.current) setSending(false)
    }
  }
  return { proof, sending, error, expiresIn, resendIn, send,
    valid: !!proof && proof.email === currentEmail && expiresIn > 0,
    changeEmail: (value: string) => {
      latestEmail.current = normalizedEmail(value)
      setProof(null); setError('')
    },
    invalidate: () => setProof(null) }
}
