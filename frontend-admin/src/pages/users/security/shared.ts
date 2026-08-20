export const securityRiskTagTheme: Record<string, string> = {
  low: 'default',
  medium: 'warning',
  high: 'danger',
  critical: 'danger',
}

export const securityStatusTagTheme: Record<string, string> = {
  success: 'success',
  failed: 'danger',
  pending: 'warning',
  ignored: 'default',
  handled: 'success',
  active: 'success',
  inactive: 'default',
  online: 'success',
  revoked: 'danger',
  expired: 'warning',
}

export function formatSecurityTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function formatSecurityCount(value?: number) {
  return Number(value || 0).toLocaleString('zh-CN')
}

export function formatSecurityBool(value?: boolean) {
  return value ? '是' : '否'
}
