export function newIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `key-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
