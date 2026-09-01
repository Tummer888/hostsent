// 财务管理 · 常量与格式化工具

// 流水类型
export const txTypeOptions = [
  { label: '充值', value: 'recharge' },
  { label: '消费', value: 'consume' },
  { label: '退款', value: 'refund' },
  { label: '佣金', value: 'commission' },
  { label: '结算', value: 'settlement' },
  { label: '调账', value: 'adjust' },
]

export const directionOptions = [
  { label: '收入', value: 1 },
  { label: '支出', value: -1 },
]

export const rechargeStatusOptions = [
  { label: '待支付', value: 'pending' },
  { label: '已到账', value: 'success' },
  { label: '失败', value: 'failed' },
]

export const rechargeMethodOptions = [
  { label: '支付宝', value: 'alipay' },
  { label: '微信支付', value: 'wechat' },
  { label: '线下/人工', value: 'manual' },
]

export const withdrawStatusOptions = [
  { label: '待审核', value: 'pending' },
  { label: '已通过', value: 'approved' },
  { label: '已驳回', value: 'rejected' },
  { label: '已打款', value: 'paid' },
  { label: '打款失败', value: 'failed' },
]

export const billStatusOptions = [
  { label: '未结', value: 'unpaid' },
  { label: '已结清', value: 'paid' },
  { label: '已关账', value: 'closed' },
]

const defaultTheme = 'default'

export function txTypeLabel(type: string): string {
  const found = txTypeOptions.find((item) => item.value === type)
  return found ? found.label : type || '—'
}

export function txTypeTheme(type: string): string {
  switch (type) {
    case 'recharge':
      return 'success'
    case 'refund':
      return 'warning'
    case 'commission':
      return 'primary'
    case 'consume':
      return 'danger'
    case 'settlement':
    case 'adjust':
      return 'warning'
    default:
      return defaultTheme
  }
}

export function directionLabel(direction: number): string {
  if (direction > 0) return '收入'
  if (direction < 0) return '支出'
  return '—'
}

export function rechargeStatusLabel(status: string): string {
  const found = rechargeStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function rechargeStatusTheme(status: string): string {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'pending':
      return 'warning'
    default:
      return defaultTheme
  }
}

export function rechargeMethodLabel(method: string): string {
  const found = rechargeMethodOptions.find((item) => item.value === method)
  return found ? found.label : method || '—'
}

export function withdrawStatusLabel(status: string): string {
  const found = withdrawStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function withdrawStatusTheme(status: string): string {
  switch (status) {
    case 'approved':
    case 'paid':
      return 'success'
    case 'rejected':
    case 'failed':
      return 'danger'
    case 'pending':
      return 'warning'
    default:
      return defaultTheme
  }
}

export function billStatusLabel(status: string): string {
  const found = billStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function billStatusTheme(status: string): string {
  switch (status) {
    case 'unpaid':
      return 'warning'
    case 'paid':
      return 'success'
    case 'closed':
      return 'default'
    default:
      return defaultTheme
  }
}

// 金额展示：正数带 + 号，负数带 - 号。direction<0（支出）时取负值显示。
export function formatAmount(value: number, direction?: number): string {
  const n = Number(value || 0)
  const signed = direction != null && direction < 0 ? -n : n
  return signed > 0 ? `+${signed.toFixed(2)}` : signed.toFixed(2)
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
