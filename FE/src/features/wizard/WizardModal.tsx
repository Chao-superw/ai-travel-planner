import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from 'react-router-dom'
import { wizardSchema, toConstraintsInput, type WizardValues } from './wizardSchema'
import { useCreatePlanningJob } from '../../api/jobs'
import { newIdempotencyKey } from '../../lib/idempotency'
import { ApiError } from '../../lib/errors'
import type { TripTemplate } from '../home/templates'

const INTEREST_OPTIONS = ['文化', '美食', '自然', '亲子', '摄影', '购物', '夜生活'] as const
const PACE_OPTIONS: { value: string; label: string }[] = [
  { value: 'relaxed', label: '轻松' }, { value: 'balanced', label: '适中' }, { value: 'packed', label: '紧凑' },
]

function prefillDefaults(prefill?: Partial<TripTemplate>): Partial<WizardValues> {
  if (!prefill) return { party_size: 2, budget_scope: 'per_person', pace: 'balanced', transport: 'transit', interests: [] }
  return {
    city: prefill.city,
    party_size: 2,
    budget_yuan: prefill.budget_cents != null ? prefill.budget_cents / 100 : undefined,
    budget_scope: 'per_person',
    interests: prefill.interests ?? [],
    pace: 'balanced',
    transport: 'transit',
  }
}

export default function WizardModal({ open, onClose, prefill }: { open: boolean; onClose: () => void; prefill?: Partial<TripTemplate> }) {
  const nav = useNavigate()
  const create = useCreatePlanningJob()
  const keyRef = useRef<string>('')
  const [topError, setTopError] = useState<string | null>(null)

  const { register, handleSubmit, reset, setError, watch, setValue, formState: { errors } } = useForm<WizardValues>({
    resolver: zodResolver(wizardSchema),
    defaultValues: prefillDefaults(prefill) as WizardValues,
  })

  // 每次打开：重置表单为最新 prefill，并生成新的幂等键
  useEffect(() => {
    if (open) {
      reset(prefillDefaults(prefill) as WizardValues)
      keyRef.current = newIdempotencyKey()
      setTopError(null)
    }
  }, [open, prefill, reset])

  const interests = watch('interests') ?? []
  const toggleInterest = (i: string) => {
    const next = interests.includes(i) ? interests.filter((x) => x !== i) : [...interests, i]
    setValue('interests', next, { shouldValidate: true })
  }

  if (!open) return null

  const onSubmit = (v: WizardValues) => {
    setTopError(null)
    create.mutate(
      { constraints: toConstraintsInput(v), idempotencyKey: keyRef.current },
      {
        onSuccess: (job) => { onClose(); nav('/planning/' + job.id) },
        onError: (e) => {
          if (e instanceof ApiError) {
            if (e.status === 400 && e.fieldErrors) {
              for (const [k, m] of Object.entries(e.fieldErrors)) setError(k as keyof WizardValues, { message: m })
              return
            }
            if (e.status === 401) { onClose(); nav('/login'); return }
            if (e.status === 429) { setTopError('今日生成次数已达上限，请稍后再试'); return }
            setTopError(e.message)
            return
          }
          setTopError('网络异常，请重试')
        },
      },
    )
  }

  const field = 'w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500'

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 px-4" onClick={onClose}>
      <div className="w-full max-w-lg bg-white rounded-2xl shadow-xl p-6 font-sans max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-medium text-gray-900">告诉「行迹 Wandr」你的偏好</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600" aria-label="关闭">✕</button>
        </div>

        {topError && <div className="mb-3 rounded-lg bg-red-50 text-red-600 text-sm px-3 py-2">{topError}</div>}

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div>
            <label className="text-sm text-gray-700">目的地城市</label>
            <input {...register('city')} placeholder="如：杭州市" className={field} />
            {errors.city && <p className="text-sm text-red-500 mt-1">{errors.city.message}</p>}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-sm text-gray-700">开始日期</label>
              <input type="date" {...register('start_date')} className={field} />
              {errors.start_date && <p className="text-sm text-red-500 mt-1">{errors.start_date.message}</p>}
            </div>
            <div>
              <label className="text-sm text-gray-700">结束日期</label>
              <input type="date" {...register('end_date')} className={field} />
              {errors.end_date && <p className="text-sm text-red-500 mt-1">{errors.end_date.message}</p>}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-sm text-gray-700">出行人数</label>
              <input type="number" min={1} max={8} {...register('party_size', { valueAsNumber: true })} className={field} />
              {errors.party_size && <p className="text-sm text-red-500 mt-1">{errors.party_size.message}</p>}
            </div>
            <div>
              <label className="text-sm text-gray-700">预算（元）</label>
              <input type="number" min={1} {...register('budget_yuan', { valueAsNumber: true })} className={field} />
              {errors.budget_yuan && <p className="text-sm text-red-500 mt-1">{errors.budget_yuan.message}</p>}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-sm text-gray-700">预算口径</label>
              <select {...register('budget_scope')} className={field}>
                <option value="per_person">每人</option>
                <option value="total">总预算</option>
              </select>
            </div>
            <div>
              <label className="text-sm text-gray-700">出行节奏</label>
              <select {...register('pace')} className={field}>
                {PACE_OPTIONS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
              </select>
            </div>
          </div>

          <div>
            <label className="text-sm text-gray-700">交通方式</label>
            <select {...register('transport')} className={field}>
              <option value="transit">公共交通</option>
              <option value="walking">步行为主</option>
            </select>
          </div>

          <div>
            <label className="text-sm text-gray-700">兴趣（至少选 1 项）</label>
            <div className="mt-2 flex flex-wrap gap-2">
              {INTEREST_OPTIONS.map((i) => (
                <button
                  type="button"
                  key={i}
                  onClick={() => toggleInterest(i)}
                  className={`rounded-full border px-3 py-1 text-sm transition ${
                    interests.includes(i) ? 'border-brand-500 text-brand-600 bg-brand-50' : 'border-gray-200 text-gray-600 hover:border-gray-300'
                  }`}
                >
                  {i}
                </button>
              ))}
            </div>
            {errors.interests && <p className="text-sm text-red-500 mt-1">{errors.interests.message as string}</p>}
          </div>

          <button type="submit" disabled={create.isPending} className="w-full bg-brand-500 hover:bg-brand-600 text-white rounded-lg py-2.5 font-medium disabled:opacity-60">
            {create.isPending ? '正在创建生成任务…' : '开始生成行程'}
          </button>
        </form>
      </div>
    </div>
  )
}
