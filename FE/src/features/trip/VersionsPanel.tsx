import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTripVersions, useTripVersion, useManualEdit } from '../../api/trips'
import { newIdempotencyKey } from '../../lib/idempotency'
import { ApiError } from '../../lib/errors'
import type { ActivityEditInput, ManualEditInput, Plan } from '../../types/api'

function planToEditInput(plan: Plan): ManualEditInput['plan'] {
  return {
    title: plan.title,
    summary: plan.summary,
    activities: plan.activities.map<ActivityEditInput>((a) => ({
      id: a.id, date: a.date, start_minute: a.start_minute, end_minute: a.end_minute,
      kind: a.kind, place_id: a.kind === 'sightseeing' ? a.place_id : '', title: a.title, reason: a.reason,
    })),
  }
}

// 版本历史：列出各版本，预览历史版本内容，并支持回滚（以历史 plan 走一次 manual_edit）。
export default function VersionsPanel({
  tripId,
  currentVersion,
  onClose,
}: {
  tripId: string
  currentVersion: number
  onClose: () => void
}) {
  const nav = useNavigate()
  const list = useTripVersions(tripId)
  const [preview, setPreview] = useState<number | null>(null)
  const detail = useTripVersion(tripId, preview)
  const rollback = useManualEdit(tripId)
  const [topError, setTopError] = useState<string | null>(null)

  const onRollback = (plan: Plan) => {
    setTopError(null)
    const input: ManualEditInput = { expected_version: currentVersion, plan: planToEditInput(plan) }
    rollback.mutate(
      { input, idempotencyKey: newIdempotencyKey() },
      {
        onSuccess: (job) => { onClose(); nav('/planning/' + job.id) },
        onError: (e) => {
          if (e instanceof ApiError) {
            if (e.status === 409) { setTopError('行程已被修改，请关闭后刷新页面再试'); return }
            setTopError(e.message)
            return
          }
          setTopError('网络异常，请重试')
        },
      },
    )
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 px-4" onClick={onClose}>
      <div className="w-full max-w-lg bg-white rounded-2xl shadow-xl p-6 font-sans max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-medium text-gray-900">版本历史</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600" aria-label="关闭">✕</button>
        </div>

        {topError && <div className="mb-3 rounded-lg bg-red-50 text-red-600 text-sm px-3 py-2">{topError}</div>}

        {list.isLoading && <p className="text-gray-500 text-sm">正在加载版本…</p>}
        {list.error && <p className="text-red-600 text-sm">加载失败：{list.error instanceof Error ? list.error.message : '未知错误'}</p>}

        <ul className="space-y-2">
          {list.data?.items.map((v) => (
            <li key={v.version} className="rounded-xl border border-gray-100 p-3">
              <div className="flex items-center justify-between">
                <div>
                  <span className="font-medium text-gray-900">v{v.version}</span>
                  {v.version === currentVersion && <span className="ml-2 text-xs rounded-full bg-brand-50 text-brand-600 px-2 py-0.5">当前</span>}
                  <div className="text-sm text-gray-500 mt-0.5">{v.title}</div>
                </div>
                <button
                  onClick={() => setPreview(preview === v.version ? null : v.version)}
                  className="text-sm text-brand-600 hover:text-brand-700"
                >
                  {preview === v.version ? '收起' : '查看'}
                </button>
              </div>

              {preview === v.version && (
                <div className="mt-3 border-t border-gray-100 pt-3">
                  {detail.isLoading && <p className="text-sm text-gray-500">正在加载该版本…</p>}
                  {detail.error && <p className="text-sm text-red-600">加载失败</p>}
                  {detail.data && (
                    <>
                      <p className="text-sm text-gray-600">{detail.data.plan.summary}</p>
                      <ul className="mt-2 space-y-1 text-sm text-gray-500 max-h-40 overflow-y-auto">
                        {detail.data.plan.activities
                          .slice()
                          .sort((a, b) => (a.date === b.date ? a.start_minute - b.start_minute : a.date < b.date ? -1 : 1))
                          .map((a) => <li key={a.id}>{a.date} · {a.title}</li>)}
                      </ul>
                      {v.version !== currentVersion && (
                        <button
                          onClick={() => onRollback(detail.data!.plan)}
                          disabled={rollback.isPending}
                          className="mt-3 w-full bg-brand-500 hover:bg-brand-600 text-white rounded-lg py-2 text-sm font-medium disabled:opacity-60"
                        >
                          {rollback.isPending ? '正在回滚…' : `回滚到 v${v.version}`}
                        </button>
                      )}
                    </>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
        {list.data && list.data.items.length === 0 && <p className="text-gray-500 text-sm">暂无历史版本。</p>}
      </div>
    </div>
  )
}
