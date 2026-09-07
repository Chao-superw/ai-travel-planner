export class ApiError extends Error {
  code: string
  status: number
  requestId?: string
  fieldErrors?: Record<string, string>
  retryAfter?: number
  constructor(status: number, code: string, message: string, requestId?: string, fieldErrors?: Record<string, string>, retryAfter?: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.requestId = requestId
    this.fieldErrors = fieldErrors
    this.retryAfter = retryAfter
  }
}
