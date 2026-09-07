import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useLogin } from '../../api/auth'
import { ApiError } from '../../lib/errors'
import AuthLayout, { buttonClass, inputClass } from './AuthLayout'
import { authError, emailSchema, passwordSchema, returnPath } from './authForm'
import { useCountdown } from './useCountdown'

export default function LoginPage() {
  const nav = useNavigate()
  const loc = useLocation()
  const login = useLogin()
  const [email, setEmail] = useState((loc.state?.email as string) ?? '')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [retryAt, setRetryAt] = useState(0)
  const retryIn = useCountdown(retryAt)
  const from = returnPath(loc.state?.from)

  async function submit(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (login.isPending || retryIn) return
    setError('')
    const address = emailSchema.safeParse(email)
    if (!address.success) { setError(address.error.issues[0].message); return }
    const checked = passwordSchema.safeParse(password)
    if (!checked.success) { setError(checked.error.issues[0].message); return }
    try {
      const session = await login.mutateAsync({ email: address.data, password })
      nav(session.user.email_verified ? from : '/bind-email', { replace: true, state: { from } })
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) setError('邮箱或密码错误')
      else setError(authError(e))
      if (e instanceof ApiError && e.retryAfter) setRetryAt(Date.now() + e.retryAfter * 1000)
    }
  }
  return <AuthLayout title="邮箱登录" subtitle="登录后，继续规划和管理你的旅行。">
    {loc.state?.registered && <p role="status" className="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-700">注册成功，请使用邮箱和密码登录。</p>}
    <form noValidate onSubmit={submit} className="space-y-4">
      <div><label htmlFor="login-email" className="block text-sm text-gray-700 mb-1.5">邮箱</label>
        <input id="login-email" type="email" autoComplete="username" autoCapitalize="none" spellCheck={false} value={email} onChange={e => setEmail(e.target.value)} placeholder="你的邮箱地址" className={inputClass} disabled={login.isPending} /></div>
      <div><label htmlFor="login-password" className="block text-sm text-gray-700 mb-1.5">密码</label>
        <input id="login-password" type="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} placeholder="输入登录密码" className={inputClass} disabled={login.isPending} /></div>
      {error && <p role="alert" className="text-sm leading-6 text-red-600">{error}</p>}
      <button type="submit" disabled={login.isPending || retryIn > 0} className={buttonClass}>{login.isPending ? '登录中…' : retryIn ? `${retryIn} 秒后重试` : '登录'}</button>
    </form>
    <div className="space-y-3 text-sm text-gray-500">
      <p>还没有账号？<Link to="/register" state={{ from }} className="text-brand-600">去注册</Link></p>
    </div>
  </AuthLayout>
}
