/**
 * 后端统一响应信封的工具函数。
 *
 * Go 后端所有接口都返回 `{ code, message, data, timestamp }`：
 * 业务失败时 HTTP 仍为 200，但 `code !== 0` 且 `data` 缺省。
 * 因此 BFF 层必须先拆信封再交给 schema 校验，否则会把错误响应当成正常数据。
 */

/** 拆信封：取 `code === 0` 时的 data，异常/业务失败一律返回 null。 */
export function unwrapEnvelope<T>(raw: unknown): T | null {
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) return null
  const body = raw as { code?: unknown; data?: unknown }
  if (typeof body.code !== 'number' || body.code !== 0) return null
  return (body.data ?? null) as T | null
}

/** 把未知值收敛为字符串（用于 query 透传）。 */
export function asQueryString(value: unknown): string | undefined {
  if (typeof value === 'string' && value.trim() !== '') return value.trim()
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return undefined
}

/** 把未知值收敛为有限整数（用于分页参数），非法返回 undefined。 */
export function asQueryInt(value: unknown): number | undefined {
  const raw = asQueryString(value)
  if (raw === undefined) return undefined
  const parsed = Number(raw)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined
}

/** 把未知值收敛为布尔（用于 `featured=true` 这类开关），非法返回 undefined。 */
export function asQueryBool(value: unknown): boolean | undefined {
  const raw = asQueryString(value)
  if (raw === undefined) return undefined
  const normalized = raw.toLowerCase()
  if (normalized === 'true' || normalized === '1') return true
  if (normalized === 'false' || normalized === '0') return false
  return undefined
}

/** 从 ofetch / Nuxt 错误对象中提取 HTTP 状态码（形态随调用方不同，逐个兜底）。 */
export function extractStatusCode(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined
  const candidate = error as {
    statusCode?: unknown
    status?: unknown
    response?: { status?: unknown }
  }
  for (const value of [candidate.statusCode, candidate.status, candidate.response?.status]) {
    if (typeof value === 'number' && Number.isFinite(value)) return value
  }
  return undefined
}

