// 订单管理 · 常量与格式化工具

export const orderStatusOptions = [
  { label: '待支付', value: 'pending' },
  { label: '已支付', value: 'paid' },
  { label: '开通中', value: 'provisioning' },
  { label: '服务中', value: 'active' },
  { label: '退款中', value: 'refunding' },
  { label: '已退款', value: 'refunded' },
  { label: '已取消', value: 'cancelled' },
  { label: '已关闭', value: 'closed' },
  { label: '已完成', value: 'completed' },
]

export const payMethodOptions = [
  { label: '余额支付', value: 'balance' },
  { label: '支付宝', value: 'alipay' },
  { label: '微信支付', value: 'wechat' },
  { label: '线下/人工', value: 'manual' },
]

export const refundStatusOptions = [
  { label: '待审核', value: 'pending' },
  { label: '已通过', value: 'approved' },
  { label: '已驳回', value: 'rejected' },
  { label: '已退款', value: 'done' },
]

// 订单类型（doc36 §3.4）：续费由 renewal_id>0 判定。
export const orderTypeOptions = [
  { label: '普通购买', value: 'consume' },
  { label: '产品续费', value: 'renewal' },
]

// 退款去向（doc36 §3.2）。
export const refundModeOptions = [
  { label: '退回余额', value: 'balance' },
  { label: '原路退回', value: 'channel' },
]

// 渠道退款状态：原路退回时由支付中心回填。
export const channelRefundStatusOptions = [
  { label: '未发起', value: 'none' },
  { label: '退款中', value: 'pending' },
  { label: '已退款', value: 'success' },
  { label: '退款失败', value: 'failed' },
]

const defaultOrderTheme = 'default'

export function orderTypeLabel(value: string): string {
  const found = orderTypeOptions.find((item) => item.value === value)
  return found ? found.label : value || '—'
}

export function refundModeLabel(mode: string): string {
  const found = refundModeOptions.find((item) => item.value === mode)
  return found ? found.label : mode || '退回余额'
}

export function refundModeTheme(mode: string): string {
  switch (mode) {
    case 'channel':
      return 'warning'
    case 'balance':
      return 'primary'
    default:
      return defaultOrderTheme
  }
}

export function channelRefundStatusLabel(status: string): string {
  const found = channelRefundStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '未发起'
}

export function channelRefundStatusTheme(status: string): string {
  switch (status) {
    case 'success':
      return 'success'
    case 'pending':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return defaultOrderTheme
  }
}

export function orderStatusLabel(status: string): string {
  const found = orderStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function orderStatusTheme(status: string): string {
  switch (status) {
    case 'active':
    case 'completed':
      return 'success'
    case 'paid':
      return 'primary'
    case 'pending':
    case 'provisioning':
    case 'refunding':
      return 'warning'
    case 'cancelled':
      return 'danger'
    case 'refunded':
    case 'closed':
    default:
      return defaultOrderTheme
  }
}

export function payMethodLabel(method: string): string {
  const found = payMethodOptions.find((item) => item.value === method)
  return found ? found.label : method || '—'
}

export function refundStatusLabel(status: string): string {
  const found = refundStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function refundStatusTheme(status: string): string {
  switch (status) {
    case 'approved':
      return 'success'
    case 'done':
      return 'primary'
    case 'rejected':
      return 'danger'
    case 'pending':
      return 'warning'
    default:
      return defaultOrderTheme
  }
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
