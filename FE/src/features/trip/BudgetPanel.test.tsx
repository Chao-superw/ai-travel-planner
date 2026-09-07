import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import BudgetPanel from './BudgetPanel'
import type { BudgetSummary } from '../../types/api'

const incomplete: BudgetSummary = {
  trip_id: 't', version: 1, budget_total_cents: 400000, known_total_cents: 250000,
  unknown_count: 3, complete: false, by_day: [], by_category: [],
}

describe('BudgetPanel', () => {
  it('shows pending-cost text and never "未超预算" when incomplete', () => {
    render(<BudgetPanel budget={incomplete} warnings={[]} />)
    expect(screen.getByText('含 3 项待确认费用')).toBeInTheDocument()
    expect(screen.queryByText('未超预算')).toBeNull()
  })
})
