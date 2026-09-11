// 实例运维台 · 常量与格式化工具

export const instanceStatusOptions = [
  { label: '运行中', value: 'running' },
  { label: '已关机', value: 'stopped' },
  { label: '创建中', value: 'creating' },
  { label: '异常', value: 'error' },
  { label: '删除中', value: 'deleting' },
]

export const expireStateOptions = [
  { label: '全部', value: 'all' },
  { label: '临期', value: 'expiring' },
  { label: '已过期', value: 'expired' },
  { label: '未设到期', value: 'none' },
]

export const powerActions = [
  { label: '开机', value: 'on', theme: 'success' },
  { label: '关机', value: 'off', theme: 'warning' },
  { label: '强制关机', value: 'hard_off', theme: 'danger' },
  { label: '重启', value: 'reboot', theme: 'primary' },
  { label: '强制重启', value: 'hard_reboot', theme: 'danger' },
]

// 电源动作影响提示（用于二次确认弹窗）
export const powerActionTips: Record<string, string> = {
  off: '关机将停止实例内所有服务，未保存的数据可能丢失。',
  hard_off: '强制关机等同于断电，可能造成数据损坏，请谨慎操作。',
  reboot: '重启会中断实例内服务，请确认业务已做好切换准备。',
  hard_reboot: '强制重启等同于断电重启，可能造成数据损坏，请谨慎操作。',
}

export const operationActionLabels: Record<string, string> = {
  power_on: '开机',
  power_off: '关机',
  hard_off: '强制关机',
  reboot: '重启',
  hard_reboot: '强制重启',
  vnc: '控制台',
  sync: '同步刷新',
  resize: '变更配置',
  destroy: '销毁',
  remark: '备注',
  suspend: '暂停',
}

export const operatorTypeLabels: Record<string, string> = {
  admin: '管理员',
  user: '用户',
  system: '系统',
}

const defaultTheme = 'default'

export function instanceStatusLabel(status: string): string {
  const found = instanceStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function instanceStatusTheme(status: string): string {
  switch (status) {
    case 'running':
      return 'success'
    case 'creating':
    case 'deleting':
      return 'warning'
    case 'error':
      return 'danger'
    case 'stopped':
    default:
      return defaultTheme
  }
}

export function expireStateLabel(state: string): string {
  const found = expireStateOptions.find((item) => item.value === state)
  return found ? found.label : state || '—'
}

export function expireStateTheme(state: string): string {
  switch (state) {
    case 'expiring':
      return 'warning'
    case 'expired':
      return 'danger'
    case 'normal':
      return 'success'
    case 'none':
    default:
      return defaultTheme
  }
}

export function powerActionLabel(action: string): string {
  const found = powerActions.find((item) => item.value === action)
  return found ? found.label : action || '—'
}

export function powerActionTheme(action: string): string {
  const found = powerActions.find((item) => item.value === action)
  return found ? found.theme : defaultTheme
}

export function operationActionLabel(action: string): string {
  return operationActionLabels[action] || action || '—'
}

export function operatorTypeLabel(type: string): string {
  return operatorTypeLabels[type] || type || '—'
}

export function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

export function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
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
