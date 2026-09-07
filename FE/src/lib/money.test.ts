import { describe, it, expect } from 'vitest'
import { centsToYuan, budgetStatusText } from './money'

describe('money', () => {
  it('converts cents to yuan with 2 decimals', () => {
    expect(centsToYuan(2000)).toBe('20.00')
    expect(centsToYuan(199)).toBe('1.99')
  })
  it('never shows within-budget when incomplete', () => {
    expect(budgetStatusText(false, undefined, 3)).toBe('含 3 项待确认费用')
  })
  it('shows within/over budget only when complete', () => {
    expect(budgetStatusText(true, true, 0)).toBe('未超预算')
    expect(budgetStatusText(true, false, 0)).toBe('已超预算')
  })
})
