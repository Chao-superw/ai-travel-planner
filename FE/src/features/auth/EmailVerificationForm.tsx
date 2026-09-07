import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useBindEmail, useLogout, useRegister } from '../../api/auth'
import { useMe } from '../../api/me'
import { ApiError } from '../../lib/errors'
import { useAuth } from '../../store/auth'
import AuthLayout, { buttonClass, inputClass } from './AuthLayout'
import { authError, emailSchema, passwordSchema, returnPath } from './authForm'
import { useCountdown } from './useCountdown'
import { useEmailChallenge } from './useEmailChallenge'

export default function EmailVerificationForm({ mode }: { mode: 'register' | 'bind' }) {
  const binding = mode === 'bind'
  const nav = useNavigate()
  const loc = useLocation()
  const from = returnPath(loc.state?.from)
  const { user } = useAuth()
  const register = useRegister()
  const bind = useBindEmail()
  const logout = useLogout()
  const me = useMe(binding)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [retryAt, setRetryAt] = useState(0)
  const retryIn = useCountdown(retryAt)
  const challenge = useEmailChallenge(email, mode)
  const pending = register.isPending || bind.isPending
  async function send() {
    setError('')
    const checked = emailSchema.safeParse(email)
    if (!checked.success) { setError(checked.error.issues[0].message); return }
    if (await challenge.send(checked.data)) setCode('')
  }
  async function submit(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (pending || retryIn || !challenge.valid || !challenge.proof) return
    setError('')
    const checked = emailSchema.safeParse(email)
    if (!checked.success) { setError(checked.error.issues[0].message); return }
    if (!binding) {
      const checkedPassword = passwordSchema.safeParse(password)
      if (!checkedPassword.success) { setError(checkedPassword.error.issues[0].message); return }
      if (password !== confirmation) { setError('两次输入的密码不一致'); return }
    }
    if (!/^\d{6}$/.test(code)) { setError('请输入邮件中的 6 位验证码'); return }
    const proof = { email: checked.data, code, challenge_id: challenge.proof.challenge_id }
    try {
      if (binding) {
        await bind.mutateAsync(proof)
        nav(from, { replace: true })
      } else {
        await register.mutateAsync({ ...proof, password })
        nav('/login', { replace: true, state: { email: checked.data, registered: true, from } })
      }
    } catch (e) {
      setError(authError(e))
      if (e instanceof ApiError) {
        if (e.retryAfter) setRetryAt(Date.now() + e.retryAfter * 1000)
        if (e.code === 'EMAIL_ALREADY_REGISTERED') challenge.invalidate()
        if (binding && e.code === 'EMAIL_ALREADY_BOUND') {
          const refreshed = await me.refetch()
          if (refreshed.data?.email_verified) nav(from, { replace: true })
        }
      }
    }
  }
  return <AuthLayout title={binding ? '绑定邮箱' : '创建账号'} subtitle={binding ? '验证你的邮箱，继续使用原来的账号和行程。之后可用邮箱和原密码登录。' : '用邮箱接收验证码，开启你的下一程。'}>
    {binding && <div className="rounded-lg bg-brand-50 px-3 py-2 text-sm text-brand-700">当前账号：{user?.username || '原账号'}</div>}
    <form noValidate onSubmit={submit} className="space-y-4">
      <div><label htmlFor="verify-email" className="block text-sm text-gray-700 mb-1.5">邮箱</label>
        <input id="verify-email" type="email" autoComplete="email" autoCapitalize="none" spellCheck={false} placeholder="你的邮箱地址" className={inputClass} value={email} disabled={pending} onChange={e => { challenge.changeEmail(e.target.value); setEmail(e.target.value); setCode(''); setError('') }} /></div>
      {!binding && <>
        <div><label htmlFor="register-password" className="block text-sm text-gray-700 mb-1.5">密码</label>
          <input id="register-password" type="password" autoComplete="new-password" placeholder="设置登录密码" className={inputClass} value={password} disabled={pending} onChange={e => setPassword(e.target.value)} /></div>
        <div><label htmlFor="register-confirm" className="block text-sm text-gray-700 mb-1.5">确认密码</label>
          <input id="register-confirm" type="password" autoComplete="new-password" placeholder="再次输入密码" className={inputClass} value={confirmation} disabled={pending} onChange={e => setConfirmation(e.target.value)} /></div>
      </>}
      <div>
        <label htmlFor="email-code" className="block text-sm text-gray-700 mb-1.5">邮箱验证码</label>
        <div className="flex gap-2">
          <input id="email-code" type="text" inputMode="numeric" autoComplete="one-time-code" maxLength={6} placeholder="6 位验证码" className={`${inputClass} min-w-0 flex-1 tracking-widest`} value={code} disabled={pending} onChange={e => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))} />
          <button type="button" onClick={send} disabled={pending || challenge.sending || challenge.resendIn > 0 || !email.trim()} className="shrink-0 rounded-lg border border-brand-200 px-3 py-2 text-sm text-brand-600 hover:bg-brand-50 disabled:text-gray-400 disabled:border-gray-200 disabled:cursor-not-allowed">
            {challenge.sending ? '发送中…' : challenge.resendIn ? `${challenge.resendIn} 秒后重发` : challenge.proof ? '重新发送' : '获取验证码'}
          </button>
        </div>
        {challenge.proof && <p role="status" className={`mt-2 text-xs leading-5 ${challenge.valid ? 'text-gray-500' : 'text-amber-700'}`}>
          {challenge.valid ? `验证码已发送，请查看邮箱（含垃圾箱）。${Math.ceil(challenge.expiresIn / 60)} 分钟内有效。` : '验证码已过期，请重新获取。'}
        </p>}
        {challenge.error && <p role="alert" className="mt-2 text-sm leading-6 text-red-600">{challenge.error}</p>}
      </div>
      {error && <p role="alert" className="text-sm leading-6 text-red-600">{error}</p>}
      <button type="submit" disabled={pending || challenge.sending || !challenge.valid || retryIn > 0} className={buttonClass}>{pending ? '验证中…' : retryIn ? `${retryIn} 秒后重试` : binding ? '验证并绑定' : '注册'}</button>
    </form>
    {binding ? <button type="button" onClick={() => logout.mutate(undefined, { onSettled: () => nav('/login', { replace: true }) })} disabled={logout.isPending || pending} className="text-sm text-gray-500 hover:text-brand-600">退出当前账号</button>
      : <p className="text-sm text-gray-500">已有账号？<Link to="/login" state={{ from }} className="text-brand-600">去登录</Link></p>}
  </AuthLayout>
}
