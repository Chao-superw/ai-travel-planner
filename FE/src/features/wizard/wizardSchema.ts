import { z } from 'zod'
import type { ConstraintsInput } from '../../types/api'

export const wizardSchema = z.object({
  city: z.string().min(1, '请填写目的地城市'),
  start_date: z.string().min(1),
  end_date: z.string().min(1),
  party_size: z.number().int().min(1).max(8),
  budget_yuan: z.number().positive('预算需大于 0'),
  budget_scope: z.enum(['per_person', 'total']),
  interests: z.array(z.string()).min(1, '至少选择一个兴趣'),
  pace: z.string().min(1),
  transport: z.enum(['walking', 'transit']),
}).refine((v) => v.end_date >= v.start_date, { path: ['end_date'], message: '结束日期不能早于开始日期' })
  .refine((v) => {
    const days = (new Date(v.end_date).getTime() - new Date(v.start_date).getTime()) / 86400000 + 1
    return days >= 1 && days <= 7
  }, { path: ['end_date'], message: '行程天数需在 1–7 天' })

export type WizardValues = z.infer<typeof wizardSchema>

export function toConstraintsInput(v: WizardValues): ConstraintsInput {
  return {
    city: v.city, start_date: v.start_date, end_date: v.end_date,
    party_size: v.party_size, budget_cents: Math.round(v.budget_yuan * 100),
    budget_scope: v.budget_scope, interests: v.interests, pace: v.pace, transport: v.transport,
  }
}
