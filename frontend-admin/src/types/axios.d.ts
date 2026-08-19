export interface ApiResult<T = unknown> {
  code: number
  message?: string
  msg?: string
  data?: T
  timestamp?: number
}

export interface Result<T = unknown> extends ApiResult<T> {}
