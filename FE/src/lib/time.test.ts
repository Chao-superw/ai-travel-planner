import { describe, it, expect } from 'vitest'
import { minuteToClock } from './time'

describe('minuteToClock', () => {
  it('formats minutes since midnight', () => {
    expect(minuteToClock(540)).toBe('09:00')
    expect(minuteToClock(0)).toBe('00:00')
    expect(minuteToClock(1080)).toBe('18:00')
  })
})
