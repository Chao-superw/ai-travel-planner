import { describe, it, expect } from 'vitest'
import { parseGcj02, parsePolyline } from './amap'

describe('amap parsing', () => {
  it('parses "lng,lat" into [lng, lat] numbers', () => {
    expect(parseGcj02('120.155,30.274')).toEqual([120.155, 30.274])
  })
  it('parses polyline into coordinate pairs', () => {
    expect(parsePolyline('120.1,30.2;120.3,30.4')).toEqual([[120.1, 30.2], [120.3, 30.4]])
  })
})
