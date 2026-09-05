// 工单支持 · 常量与格式化工具（doc50）

// 工单状态选项
export const ticketStatusOptions = [
  { label: '待响应', value: 'open' },
  { label: '处理中', value: 'in_progress' },
  { label: '等待用户', value: 'waiting_user' },
  { label: '已解决', value: 'resolved' },
  { label: '已关闭', value: 'closed' },
  { label: '已取消', value: 'cancelled' },
]

// 优先级选项
export const ticketPriorityOptions = [
  { label: '低', value: 'low' },
  { label: '中', value: 'medium' },
  { label: '高', value: 'high' },
  { label: '紧急', value: 'urgent' },
]

// 分类状态下拉选项
export const categoryStatusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const defaultTheme = 'default'

/** 工单状态 → 展示文案 */
export function ticketStatusLabel(status: string): string {
  const found = ticketStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

/** 工单状态 → 标签主题色 */
export function ticketStatusTheme(status: string): string {
  switch (status) {
    case 'open':
    case 'waiting_user':
      return 'warning'
    case 'in_progress':
      return 'primary'
    case 'resolved':
      return 'success'
    case 'closed':
    case 'cancelled':
      return 'default'
    default:
      return defaultTheme
  }
}

/** 优先级 → 展示文案 */
export function ticketPriorityLabel(priority: string): string {
  const found = ticketPriorityOptions.find((item) => item.value === priority)
  return found ? found.label : priority || '—'
}

/** 优先级 → 标签主题色 */
export function ticketPriorityTheme(priority: string): string {
  switch (priority) {
    case 'urgent':
      return 'danger'
    case 'high':
      return 'warning'
    case 'medium':
      return 'primary'
    case 'low':
      return 'default'
    default:
      return defaultTheme
  }
}

/** 分类状态 → 展示文案 */
export function categoryStatusLabel(status: string): string {
  return status === 'active' ? '启用' : status === 'disabled' ? '禁用' : status || '—'
}

/** 分类状态 → 标签主题色 */
export function categoryStatusTheme(status: string): string {
  return status === 'active' ? 'success' : status === 'disabled' ? 'default' : defaultTheme
}

/** 发送人类型 → 展示文案 */
export function senderTypeLabel(senderType: string): string {
  return senderType === 'admin' ? '客服' : senderType === 'user' ? '用户' : senderType || '—'
}

/** 格式化金额（备用） */
export function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

/** 格式化时间为 YYYY-MM-DD HH:mm */
export function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** 平均首次响应秒数 → 人类可读文案 */
export function formatDuration(seconds: number): string {
  const total = Math.max(0, Math.floor(Number(seconds || 0)))
  if (total === 0) return '—'
  if (total < 60) return `${total} 秒`
  const minutes = Math.floor(total / 60)
  if (minutes < 60) return `${minutes} 分钟`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时 ${minutes % 60} 分`
  const days = Math.floor(hours / 24)
  return `${days} 天 ${hours % 24} 小时`
}

// 将日期选择器输入统一规范为 YYYY-MM-DD 字符串
export function toDateString(value: unknown): string {
  if (!value) return ''
  if (value instanceof Date && !Number.isNaN(value.getTime())) {
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}`
  }
  const s = String(value)
  const matched = s.match(/^\d{4}-\d{2}-\d{2}/)
  return matched ? matched[0] : s
}
