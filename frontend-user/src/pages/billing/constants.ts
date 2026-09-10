// 用户中心 · 费用中心常量与格式化

export const txTypeOptions = [
  { label: '充值', value: 'recharge' },
  { label: '消费', value: 'consume' },
  { label: '退款', value: 'refund' },
  { label: '佣金', value: 'commission' },
  { label: '结算', value: 'settlement' },
  { label: '调账', value: 'adjust' },
  { label: '返现转入', value: 'referral_transfer' },
]

export const directionOptions = [
  { label: '收入', value: 1 },
  { label: '支出', value: -1 },
]

export const rechargeMethodOptions = [
  { label: '支付宝', value: 'alipay' },
  { label: '微信支付', value: 'wechat' },
  { label: '线下/人工', value: 'manual' },
]

export const billStatusOptions = [
  { label: '未结', value: 'unpaid' },
  { label: '已结清', value: 'paid' },
  { label: '已关账', value: 'closed' },
]

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
    case 'referral_transfer':
      return 'success'
    default:
      return 'default'
  }
}

export function directionLabel(direction: number): string {
  if (direction > 0) return '收入'
  if (direction < 0) return '支出'
  return '—'
}

export function rechargeStatusLabel(status: string): string {
  const map: Record<string, string> = { pending: '待支付', success: '已到账', failed: '失败' }
  return map[status] || status || '—'
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
      return 'default'
  }
}

export function billStatusLabel(status: string): string {
  const map: Record<string, string> = { unpaid: '未结', paid: '已结清', closed: '已关账' }
  return map[status] || status || '—'
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
      return 'default'
  }
}

export function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

export function formatAmount(value: number, direction?: number): string {
  const n = Number(value || 0)
  const signed = direction != null && direction < 0 ? -n : n
  return signed > 0 ? `+${signed.toFixed(2)}` : signed.toFixed(2)
}

export function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}
