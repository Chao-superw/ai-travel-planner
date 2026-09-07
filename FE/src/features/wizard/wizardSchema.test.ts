import { describe, it, expect } from 'vitest'
import { wizardSchema, toConstraintsInput } from './wizardSchema'

const valid = {
  city: '杭州市', start_date: '2026-10-01', end_date: '2026-10-03',
  party_size: 2, budget_yuan: 2000, budget_scope: 'per_person',
  interests: ['文化', '美食'], pace: 'balanced', transport: 'transit',
}

describe('wizardSchema', () => {
  it('accepts valid input', () => {
    expect(wizardSchema.safeParse(valid).success).toBe(true)
  })
  it('rejects end_date before start_date', () => {
    const r = wizardSchema.safeParse({ ...valid, end_date: '2026-09-30' })
    expect(r.success).toBe(false)
  })
  it('rejects party_size out of 1..8', () => {
    expect(wizardSchema.safeParse({ ...valid, party_size: 9 }).success).toBe(false)
  })
  it('converts yuan to cents', () => {
    expect(toConstraintsInput(valid as any).budget_cents).toBe(200000)
  })
})
