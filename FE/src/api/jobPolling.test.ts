import { describe, it, expect } from 'vitest'
import { isTerminal, pollInterval } from './jobPolling'

describe('job polling helpers', () => {
  it('marks terminal statuses', () => {
    expect(isTerminal('succeeded')).toBe(true)
    expect(isTerminal('failed')).toBe(true)
    expect(isTerminal('conflicted')).toBe(true)
    expect(isTerminal('interrupted')).toBe(true)
    expect(isTerminal('queued')).toBe(false)
    expect(isTerminal('running')).toBe(false)
  })
  it('polls every 2s until terminal', () => {
    expect(pollInterval({ status: 'running' } as any)).toBe(2000)
    expect(pollInterval({ status: 'succeeded' } as any)).toBe(false)
    expect(pollInterval(undefined)).toBe(2000)
  })
})
