// 日志中心（doc92）· 共享常量与展示工具
//
// 26 个源共用一套页面，列定义完全由后端 catalog 下发；这里只放「与源无关」的
// 枚举映射与格式化工具（状态七色、动作、字节/行数的人类可读化）。

/** 清理任务状态（七色，与后端状态机一一对应） */
export const CLEANUP_STATUS_OPTIONS = [
  { value: 'pending', label: '待执行' },
  { value: 'exporting', label: '导出中' },
  { value: 'verifying', label: '校验中' },
  { value: 'deleting', label: '删除中' },
  { value: 'done', label: '已完成' },
  { value: 'failed', label: '失败' },
  { value: 'cancelled', label: '已取消' },
]

export type CleanupStatusTheme = 'default' | 'primary' | 'warning' | 'success' | 'danger'

export function cleanupStatusLabel(status?: string): string {
  return CLEANUP_STATUS_OPTIONS.find((item) => item.value === status)?.label || status || '—'
}

export function cleanupStatusTheme(status?: string): CleanupStatusTheme {
  switch (status) {
    case 'pending':
      return 'default'
    case 'exporting':
    case 'verifying':
      return 'primary'
    case 'deleting':
      return 'warning'
    case 'done':
      return 'success'
    case 'failed':
      return 'danger'
    case 'cancelled':
      return 'default'
    default:
      return 'default'
  }
}

/** 非终态需要按 3 秒轮询刷新进度 */
export function cleanupStatusActive(status?: string): boolean {
  return status === 'pending' || status === 'exporting' || status === 'verifying' || status === 'deleting'
}

/** 触发方式 */
export function triggerLabel(trigger?: string): string {
  if (trigger === 'manual') return '手动'
  if (trigger === 'scheduled') return '调度'
  return trigger || '—'
}

/** 导出文件状态 */
export function exportStatusLabel(status?: string): string {
  switch (status) {
    case 'success':
      return '可用'
    case 'failed':
      return '失败'
    case 'deleted':
      return '已删除'
    default:
      return status || '—'
  }
}

export function exportStatusTheme(status?: string): CleanupStatusTheme {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'deleted':
      return 'default'
    default:
      return 'default'
  }
}

/** 保留策略动作（后端 catalog 亦下发，这里作为兜底与筛选选项） */
export const POLICY_ACTION_OPTIONS = [
  { value: 'clean', label: '清理' },
  { value: 'archive_only', label: '仅备份不清理' },
  { value: 'keep', label: '保留不处理' },
]

export function policyActionLabel(action?: string): string {
  return POLICY_ACTION_OPTIONS.find((item) => item.value === action)?.label || action || '—'
}

export function policyActionTheme(action?: string): CleanupStatusTheme {
  switch (action) {
    case 'clean':
      return 'warning'
    case 'archive_only':
      return 'primary'
    case 'keep':
      return 'default'
    default:
      return 'default'
  }
}

/** 源分级：ops 可清理 / audit 仅备份 */
export function classLabel(cls?: string): string {
  if (cls === 'ops') return '运维流水'
  if (cls === 'audit') return '审计留痕'
  return cls || '—'
}

export function classTheme(cls?: string): CleanupStatusTheme {
  return cls === 'ops' ? 'success' : 'primary'
}

/**
 * 保留期硬地板（与后端 catalog.MinRetentionDays 同口径）。
 * 低于标准 180 天给黄警示、低于 30 天给红警示并要求二次确认，但都不阻止 ——
 * 只有 7 天是硬地板（后端也会拒）。
 */
export const MIN_RETENTION_DAYS = 7
export const STANDARD_RETENTION_DAYS = 180
/** 站内信/已读记录的 seed 默认值：低于它会让用户的「我的消息」历史消失（D12） */
export const INBOX_RETENTION_DAYS = 365

export function retentionWarn(sourceKey: string, days: number): { level: 'none' | 'warn' | 'danger'; text: string } {
  if (!days || days <= 0) return { level: 'none', text: '' }
  if (sourceKey === 'notify_inbox' || sourceKey === 'notify_read') {
    if (days < INBOX_RETENTION_DAYS) {
      return {
        level: 'danger',
        text: `站内信是用户可见内容，低于 ${INBOX_RETENTION_DAYS} 天会使用户历史消息消失`,
      }
    }
    return { level: 'none', text: '' }
  }
  if (days < 30) {
    return { level: 'danger', text: `低于 30 天，删除后不可恢复（硬地板 ${MIN_RETENTION_DAYS} 天）` }
  }
  if (days < STANDARD_RETENTION_DAYS) {
    return { level: 'warn', text: `低于 ${STANDARD_RETENTION_DAYS} 天标准保留期` }
  }
  return { level: 'none', text: '' }
}

/** 统一时间格式（与后端 normalizeValue 的 "2006-01-02 15:04:05" 一致） */
export function formatTime(value?: string): string {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

/** 千分位 */
export function formatNumber(value?: number | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  return value.toLocaleString('zh-CN')
}

/** 人类可读字节 */
export function formatBytes(value?: number | null): string {
  if (!value || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let idx = 0
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024
    idx++
  }
  return `${size.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`
}

/** 人类可读行数（导出文件大小、候选行数等） */
export function formatRows(value?: number | null): string {
  if (value === undefined || value === null) return '—'
  return `${formatNumber(value)} 行`
}

/** JSON 美化（详情抽屉里展示 jsonb / 嵌套 body） */
export function prettyJSON(value: unknown): string {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}
