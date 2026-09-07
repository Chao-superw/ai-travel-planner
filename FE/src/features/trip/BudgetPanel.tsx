import type { BudgetSummary, AmountGroup } from '../../types/api'
import { centsToYuan, budgetStatusText } from '../../lib/money'

function GroupList({ title, groups }: { title: string; groups: AmountGroup[] }) {
  if (!groups.length) return null
  return (
    <div className="mt-4">
      <h4 className="text-sm font-medium text-gray-700 mb-2">{title}</h4>
      <ul className="space-y-1">
        {groups.map((g) => (
          <li key={g.key} className="flex items-center justify-between text-sm text-gray-600">
            <span>{g.key}</span>
            <span>
              ¥{centsToYuan(g.known_cents)}
              {g.unknown_count > 0 && <span className="text-gray-400"> · {g.unknown_count} 项待确认</span>}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

export default function BudgetPanel({ budget, warnings }: { budget: BudgetSummary; warnings: string[] }) {
  const status = budgetStatusText(budget.complete, budget.within_budget, budget.unknown_count)
  return (
    <aside className="rounded-2xl border border-gray-100 p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-medium text-gray-900">预算概览</h3>
        <span className={`text-sm ${budget.complete && budget.within_budget === false ? 'text-red-500' : 'text-brand-600'}`}>{status}</span>
      </div>

      <dl className="mt-4 space-y-2 text-sm">
        <div className="flex justify-between"><dt className="text-gray-500">总预算</dt><dd className="text-gray-900">¥{centsToYuan(budget.budget_total_cents)}</dd></div>
        <div className="flex justify-between"><dt className="text-gray-500">已知合计</dt><dd className="text-gray-900">¥{centsToYuan(budget.known_total_cents)}</dd></div>
        <div className="flex justify-between"><dt className="text-gray-500">待确认项</dt><dd className="text-gray-900">{budget.unknown_count} 项</dd></div>
      </dl>

      <GroupList title="按天" groups={budget.by_day} />
      <GroupList title="按类别" groups={budget.by_category} />

      {warnings.length > 0 && (
        <div className="mt-4 rounded-lg bg-amber-50 border border-amber-100 p-3">
          <h4 className="text-sm font-medium text-amber-700 mb-1">提示</h4>
          <ul className="list-disc list-inside space-y-1 text-sm text-amber-700">
            {warnings.map((w, i) => <li key={i}>{w}</li>)}
          </ul>
        </div>
      )}
    </aside>
  )
}
