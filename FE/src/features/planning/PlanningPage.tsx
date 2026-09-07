import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import Header from '../../components/Header'
import { useJobPolling, useRetryJob } from '../../api/jobPolling'
import { newIdempotencyKey } from '../../lib/idempotency'

const SOFT_LIMIT_S = 600

const STAGE_LABELS: Record<string, string> = {
  queued: '任务排队中…',
  running: 'AI 正在为你规划行程…',
  generating: '正在生成行程草稿…',
  querying_routes: '正在查询路线与交通…',
  repairing: '正在校对与修复细节…',
  scoring: '正在评估行程质量…',
  finalizing: '即将完成，正在收尾…',
}

function stageLabel(stage?: string, status?: string): string {
  if (status === 'queued') return STAGE_LABELS.queued
  if (stage && STAGE_LABELS[stage]) return STAGE_LABELS[stage]
  return 'AI 正在为你规划行程…'
}

export default function PlanningPage() {
  const { jobId = '' } = useParams()
  const nav = useNavigate()
  const { data: job, error, isLoading } = useJobPolling(jobId)
  const retry = useRetryJob()

  const [elapsed, setElapsed] = useState(0)
  const startRef = useRef<number>(Date.now())
  useEffect(() => {
    const t = setInterval(() => setElapsed(Math.floor((Date.now() - startRef.current) / 1000)), 1000)
    return () => clearInterval(t)
  }, [])

  // 成功即跳详情
  useEffect(() => {
    if (job?.status === 'succeeded' && job.trip_id) {
      nav('/trips/' + job.trip_id, { replace: true })
    }
  }, [job?.status, job?.trip_id, nav])

  const onRetry = () => {
    retry.mutate(
      { jobId, idempotencyKey: newIdempotencyKey() },
      { onSuccess: (newJob) => nav('/planning/' + newJob.id, { replace: true }) },
    )
  }

  const overSoftLimit = elapsed >= SOFT_LIMIT_S

  const Wrap = ({ children }: { children: React.ReactNode }) => (
    <div className="min-h-screen bg-white font-sans">
      <Header />
      <main className="pt-16 mx-auto max-w-container px-4 py-16 flex flex-col items-center text-center">
        {children}
      </main>
    </div>
  )

  if (error) {
    return (
      <Wrap>
        <p className="text-red-600">加载任务失败：{error instanceof Error ? error.message : '未知错误'}</p>
        <button onClick={() => nav('/')} className="mt-6 rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-5 py-2.5">返回首页</button>
      </Wrap>
    )
  }

  if (isLoading || !job) {
    return <Wrap><p className="text-gray-500">正在获取任务状态…</p></Wrap>
  }

  // 失败 / 中断
  if (job.status === 'failed' || job.status === 'interrupted') {
    return (
      <Wrap>
        <h1 className="text-xl font-medium text-gray-900">行程生成未成功</h1>
        <p className="mt-3 text-gray-500">{job.error?.message ?? '生成过程中出现问题，请重试。'}</p>
        <div className="mt-6 flex gap-3">
          <button onClick={onRetry} disabled={retry.isPending} className="rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-5 py-2.5 disabled:opacity-60">
            {retry.isPending ? '正在重试…' : '重试'}
          </button>
          <button onClick={() => nav('/')} className="rounded-lg border border-gray-200 text-gray-600 px-5 py-2.5 hover:border-gray-300">返回首页</button>
        </div>
      </Wrap>
    )
  }

  // 冲突
  if (job.status === 'conflicted') {
    return (
      <Wrap>
        <h1 className="text-xl font-medium text-gray-900">行程已被修改</h1>
        <p className="mt-3 text-gray-500">请返回首页重新发起规划。</p>
        <button onClick={() => nav('/')} className="mt-6 rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-5 py-2.5">返回首页重填</button>
      </Wrap>
    )
  }

  // succeeded 状态由上面的 useEffect 跳转，这里兜底渲染中间态 UI（queued/running/succeeded 瞬间）
  return (
    <Wrap>
      <div className="w-14 h-14 rounded-full border-4 border-brand-100 border-t-brand-500 animate-spin" />
      <h1 className="mt-6 text-xl font-medium text-gray-900">{stageLabel(job.stage, job.status)}</h1>
      <p className="mt-2 text-gray-400 text-sm">已等待 {elapsed}s · 生成可能需要几分钟，请保持页面打开</p>
      {overSoftLimit && (
        <div className="mt-6">
          <p className="text-gray-500">仍在处理，你可以稍后在「我的行程」查看结果。</p>
          <button onClick={() => nav('/trips')} className="mt-3 rounded-lg border border-gray-200 text-gray-600 px-5 py-2.5 hover:border-gray-300">去我的行程</button>
        </div>
      )}
    </Wrap>
  )
}
