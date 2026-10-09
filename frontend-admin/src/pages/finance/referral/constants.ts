// 推广返现 · 常量与格式化工具

/** 返现台账类型。 */
export const referralTxTypeOptions = [
  { label: '订单返现', value: 'cashback' },
  { label: '续费返现', value: 'renewal_cashback' },
  { label: '退款冲减', value: 'refund_clawback' },
  { label: '提现冻结', value: 'withdraw_freeze' },
  { label: '提现解冻', value: 'withdraw_return' },
  { label: '转入余额', value: 'transfer_out' },
  { label: '转入回滚', value: 'transfer_return' },
]

/** 返现提现单状态（与现金提现不同：无打款环节）。 */
export const referralWithdrawStatusOptions = [
  { label: '待审核', value: 'pending' },
  { label: '已通过', value: 'approved' },
  { label: '已驳回', value: 'rejected' },
]

const txTypeLabelMap: Record<string, string> = referralTxTypeOptions.reduce(
  (acc, item) => ({ ...acc, [item.value]: item.label }),
  {},
)

export function referralTxTypeLabel(type: string): string {
  return txTypeLabelMap[type] || type || '—'
}

export function referralTxTypeTheme(type: string): string {
  switch (type) {
    case 'cashback':
    case 'renewal_cashback':
      return 'success'
    case 'refund_clawback':
    case 'transfer_return':
      return 'danger'
    case 'withdraw_freeze':
      return 'warning'
    case 'withdraw_return':
    case 'transfer_out':
      return 'primary'
    default:
      return 'default'
  }
}

export function referralWithdrawStatusLabel(status: string): string {
  const map: Record<string, string> = { pending: '待审核', approved: '已通过', rejected: '已驳回' }
  return map[status] || status || '—'
}

export function referralWithdrawStatusTheme(status: string): string {
  switch (status) {
    case 'approved':
      return 'success'
    case 'pending':
      return 'warning'
    case 'rejected':
      return 'danger'
    default:
      return 'default'
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
