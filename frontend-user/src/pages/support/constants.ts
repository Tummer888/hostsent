// 用户中心 · 工单支持常量与格式化

// 工单状态选项
export const ticketStatusOptions = [
  { label: '待处理', value: 'open' },
  { label: '处理中', value: 'in_progress' },
  { label: '等待用户', value: 'waiting_user' },
  { label: '已解决', value: 'resolved' },
  { label: '已关闭', value: 'closed' },
  { label: '已取消', value: 'cancelled' },
]

// 工单优先级选项
export const ticketPriorityOptions = [
  { label: '低', value: 'low' },
  { label: '中', value: 'medium' },
  { label: '高', value: 'high' },
  { label: '紧急', value: 'urgent' },
]

// 工单状态 → 文案
export function ticketStatusLabel(status: string): string {
  const map: Record<string, string> = {
    open: '待处理',
    in_progress: '处理中',
    waiting_user: '等待用户',
    resolved: '已解决',
    closed: '已关闭',
    cancelled: '已取消',
  }
  return map[status] || status || '—'
}

// 工单状态 → TDesign 标签主题色
export function ticketStatusTheme(status: string): string {
  switch (status) {
    case 'open':
      return 'warning'
    case 'in_progress':
      return 'primary'
    case 'waiting_user':
      return 'warning'
    case 'resolved':
      return 'success'
    case 'closed':
    case 'cancelled':
      return 'default'
    default:
      return 'default'
  }
}

// 工单优先级 → 文案
export function ticketPriorityLabel(priority: string): string {
  const map: Record<string, string> = { low: '低', medium: '中', high: '高', urgent: '紧急' }
  return map[priority] || priority || '—'
}

// 工单优先级 → TDesign 标签主题色
export function ticketPriorityTheme(priority: string): string {
  switch (priority) {
    case 'low':
      return 'default'
    case 'medium':
      return 'primary'
    case 'high':
      return 'warning'
    case 'urgent':
      return 'danger'
    default:
      return 'default'
  }
}

// 时间格式化：YYYY-MM-DD HH:mm（value 允许为空，返回占位符）
export function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}
