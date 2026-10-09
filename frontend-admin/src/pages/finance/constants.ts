// 财务管理 · 常量与格式化工具

// 流水类型。
// 必须与后端 transmodel 的 TxType* 全集一致（历史遗漏 referral_transfer/freeze/unfreeze，
// 导致列表把这三个类型渲染成英文原文、筛选也选不到）。
export const txTypeOptions = [
  { label: '充值', value: 'recharge' },
  { label: '消费', value: 'consume' },
  { label: '退款', value: 'refund' },
  { label: '佣金', value: 'commission' },
  { label: '结算', value: 'settlement' },
  { label: '调账', value: 'adjust' },
  // 后台代客下单扣款（server 装配层的代下单路径直接写 type=order，历史无标签）。
  { label: '后台下单', value: 'order' },
  { label: '返现转入', value: 'referral_transfer' },
  { label: '冻结', value: 'freeze' },
  { label: '解冻', value: 'unfreeze' },
]

/** 内部划转类型：可用余额 ↔ 冻结余额，不是真实收付，不计入收支口径（与后端一致）。 */
export const internalTxTypes = ['freeze', 'unfreeze']

export function isInternalTxType(type: string): boolean {
  return internalTxTypes.includes(type)
}

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

// 账单分类（doc36 §3.4）：按期账单按消费构成区分购买/续费；充值账单为逐笔凭证。
export const billTypeOptions = [
  { label: '产品购买', value: 'consumption' },
  { label: '产品续费', value: 'renewal' },
  { label: '购买+续费', value: 'mixed' },
  { label: '充值账单', value: 'recharge' },
]

// 账单来源：按期归集的消费账单 vs 逐笔开的充值账单。
export const billSourceOptions = [
  { label: '消费账单（按期）', value: 'period' },
  { label: '充值账单（逐笔）', value: 'recharge' },
]

// 发票状态（doc36 §3.3）。
export const invoiceStatusOptions = [
  { label: '未开票', value: 'none' },
  { label: '已申请', value: 'applied' },
  { label: '已开票', value: 'issued' },
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
    case 'referral_transfer':
      return 'success'
    case 'order':
      return 'danger'
    // 冻结/解冻为内部划转，用中性色提示「不是真实收付」。
    case 'freeze':
    case 'unfreeze':
      return defaultTheme
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

// 账单分类标签（doc36 §3.4）。
export function billTypeLabel(type: string): string {
  const found = billTypeOptions.find((item) => item.value === type)
  return found ? found.label : type || '—'
}

export function billTypeTheme(type: string): string {
  switch (type) {
    case 'consumption':
      return 'primary'
    case 'renewal':
      return 'success'
    case 'mixed':
      return 'warning'
    case 'recharge':
      return 'default'
    default:
      return defaultTheme
  }
}

// 发票状态标签（账单行内）。
export function invoiceStatusLabel(status: string): string {
  const found = invoiceStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '未开票'
}

export function invoiceStatusTheme(status: string): string {
  switch (status) {
    case 'issued':
      return 'success'
    case 'applied':
      return 'warning'
    default:
      return defaultTheme
  }
}

// 发票申请状态标签。
export function invoiceRequestStatusLabel(status: string): string {
  const found = invoiceRequestStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function invoiceRequestStatusTheme(status: string): string {
  switch (status) {
    case 'issued':
      return 'success'
    case 'pending':
      return 'warning'
    case 'rejected':
      return 'danger'
    default:
      return defaultTheme
  }
}

export function invoiceTypeLabel(type: string): string {
  const found = invoiceTypeOptions.find((item) => item.value === type)
  return found ? found.label : type || '普票'
}

// 支付方式标签（账单结清口径，doc34 F-11）。
export function payMethodLabel(method: string): string {
  const map: Record<string, string> = {
    balance: '余额支付',
    alipay: '支付宝',
    wechat: '微信支付',
    manual: '线下/人工',
  }
  return map[method] || method || '—'
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

// ===== 统计期间（总览 / 报表共用） =====

/** 期间快捷项。值统一由 resolvePeriodRange 解析为 YYYY-MM-DD 区间。 */
export const periodPresetOptions = [
  { label: '今日', value: 'today' },
  { label: '近 7 天', value: 'last7' },
  { label: '近 30 天', value: 'last30' },
  { label: '本月', value: 'this-month' },
  { label: '上月', value: 'last-month' },
  { label: '近 90 天', value: 'last90' },
]

export type PeriodPreset = 'today' | 'last7' | 'last30' | 'this-month' | 'last-month' | 'last90' | 'custom'

export interface PeriodRange {
  start: string
  end: string
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

/** 本地日期格式化（避免 toISOString 的时区偏移导致日期差一天）。 */
export function formatDate(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function daysAgo(days: number): Date {
  const date = new Date()
  date.setDate(date.getDate() - days)
  return date
}

/** 解析期间快捷项为闭区间 [start, end]（end 含当天）。 */
export function resolvePeriodRange(preset: PeriodPreset): PeriodRange {
  const today = new Date()
  switch (preset) {
    case 'today':
      return { start: formatDate(today), end: formatDate(today) }
    case 'last7':
      return { start: formatDate(daysAgo(6)), end: formatDate(today) }
    case 'last90':
      return { start: formatDate(daysAgo(89)), end: formatDate(today) }
    case 'this-month':
      return { start: formatDate(new Date(today.getFullYear(), today.getMonth(), 1)), end: formatDate(today) }
    case 'last-month': {
      const first = new Date(today.getFullYear(), today.getMonth() - 1, 1)
      const last = new Date(today.getFullYear(), today.getMonth(), 0)
      return { start: formatDate(first), end: formatDate(last) }
    }
    case 'last30':
    default:
      return { start: formatDate(daysAgo(29)), end: formatDate(today) }
  }
}

/** 把日期区间格式化为接口所需的闭区间时间串（end 取当天 23:59:59）。 */
export function toQueryTimeRange(range: PeriodRange): { start_time?: string; end_time?: string } {
  return {
    start_time: range.start ? `${range.start} 00:00:00` : undefined,
    end_time: range.end ? `${range.end} 23:59:59` : undefined,
  }
}

// ===== CSV 导出（客户端，报表用） =====

/** 下载 CSV：UTF-8 BOM + 逗号分隔，Excel 直接打开中文不乱码。 */
export function downloadCsv(filename: string, header: string[], rows: (string | number)[][]): void {
  const escape = (value: string | number): string => {
    const text = String(value ?? '')
    return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
  }
  const lines = [header, ...rows].map((row) => row.map(escape).join(','))
  const blob = new Blob([`\uFEFF${lines.join('\n')}`], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
