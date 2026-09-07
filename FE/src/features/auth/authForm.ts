import { z } from 'zod'
import { ApiError } from '../../lib/errors'

export function normalizedEmail(value: string) {
  const trimmed = value.trim()
  const split = trimmed.lastIndexOf('@')
  return split < 0 ? trimmed : trimmed.slice(0, split + 1) + trimmed.slice(split + 1).toLowerCase()
}
export const emailSchema = z.string().refine(v => ![...v].some(c => c.charCodeAt(0) < 32 || c.charCodeAt(0) === 127), '请输入有效的邮箱地址')
  .transform(normalizedEmail).pipe(z.email('请输入有效的邮箱地址').max(254, '邮箱地址过长'))
export const passwordSchema = z.string().refine(v => {
  const length = new TextEncoder().encode(v).length
  return length >= 8 && length <= 72
}, '密码长度须为 8–72 字节，中文通常占 3 字节')
export function returnPath(from: unknown) {
  if (typeof from !== 'string' || !from.startsWith('/') || from.startsWith('//') || from.includes('\\')) return '/'
  return ['/login', '/register', '/bind-email'].includes(from.split('?')[0]) ? '/' : from
}
export function authError(error: unknown) {
  if (error instanceof ApiError) {
    if (error.code === 'INVALID_VERIFICATION_CODE') return '验证码错误、已过期或已使用，请检查或重新获取。'
    if (error.code === 'EMAIL_ALREADY_REGISTERED') return '这个邮箱已有账号，请直接登录，或使用其他邮箱。'
    if (error.code === 'MAIL_UNAVAILABLE') return '邮件暂时无法发送，请稍后重试。先前仍有效的验证码可以继续使用。'
    if (error.code === 'AUTH_RATE_LIMITED') return '操作过于频繁，请等待倒计时结束后重试。'
    if (error.status === 503 || error.status === 504) return '服务暂时不可用或请求超时，请稍后重试。'
    return error.message
  }
  return '网络连接失败，请检查网络后重试。'
}
