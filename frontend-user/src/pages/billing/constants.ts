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

// 账单分类（doc36 §3.4）。
export const billTypeOptions = [
  { label: '产品购买', value: 'consumption' },
  { label: '产品续费', value: 'renewal' },
  { label: '购买+续费', value: 'mixed' },
  { label: '余额充值', value: 'recharge' },
]

// 账单发票状态（doc36 §3.3）。
export const billInvoiceStatusOptions = [
  { label: '未开票', value: 'none' },
  { label: '已申请', value: 'applied' },
  { label: '已开票', value: 'issued' },
  { label: '已驳回', value: 'rejected' },
]

// 发票申请状态。
export const invoiceRequestStatusOptions = [
  { label: '待开票', value: 'pending' },
  { label: '已开票', value: 'issued' },
  { label: '已驳回', value: 'rejected' },
]

// 发票类型。
export const invoiceTypeOptions = [
  { label: '增值税普通发票', value: 'normal' },
  { label: '增值税专用发票', value: 'special' },
]

// 支付方式（含支付中心新增渠道）。
export const payMethodOptions = [
  { label: '余额支付', value: 'balance' },
  { label: '支付宝', value: 'alipay' },
  { label: '微信支付', value: 'wechat' },
  { label: '云闪付', value: 'unionpay' },
  { label: '翼支付', value: 'bestpay' },
  { label: '线下/人工', value: 'manual' },
]

export function billTypeLabel(type?: string): string {
  const found = billTypeOptions.find((item) => item.value === type)
  return found ? found.label : type || '—'
}

export function billTypeTheme(type?: string): string {
  switch (type) {
    case 'consumption':
      return 'primary'
    case 'renewal':
      return 'success'
    case 'mixed':
      return 'warning'
    default:
      return 'default'
  }
}

export function billInvoiceStatusLabel(status?: string): string {
  const found = billInvoiceStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '未开票'
}

export function billInvoiceStatusTheme(status?: string): string {
  switch (status) {
    case 'issued':
      return 'success'
    case 'applied':
      return 'warning'
    case 'rejected':
      return 'danger'
    default:
      return 'default'
  }
}

export function invoiceRequestStatusLabel(status?: string): string {
  const found = invoiceRequestStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function invoiceRequestStatusTheme(status?: string): string {
  switch (status) {
    case 'issued':
      return 'success'
    case 'pending':
      return 'warning'
    case 'rejected':
      return 'danger'
    default:
      return 'default'
  }
}

export function invoiceTypeLabel(type?: string): string {
  const found = invoiceTypeOptions.find((item) => item.value === type)
  return found ? found.label : type || '普票'
}

export function payMethodLabel(method?: string): string {
  const found = payMethodOptions.find((item) => item.value === method)
  return found ? found.label : method || '—'
}

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
