// 用户中心 · 积分常量与格式化
//
// 边界提示：积分独立于余额账本，不能抵扣订单/账单，也不能提现或转入余额，
// 仅用于活动参与与权益兑换。

/** 积分流水类型。 */
export const pointTxTypeOptions = [
  { label: '订单发放', value: 'earn_payment' },
  { label: '续费发放', value: 'earn_renewal' },
  { label: '活动发放', value: 'earn_activity' },
  { label: '兑换消耗', value: 'spend_exchange' },
  { label: '人工调整', value: 'adjust' },
  { label: '到期回收', value: 'expire' },
]

export const pointDirectionOptions = [
  { label: '获得', value: 1 },
  { label: '消耗', value: -1 },
]

export const pointSceneLabels: Record<string, string> = {
  order_purchase: '订单购买',
  order_renewal: '订单续费',
  bill_payment: '账单结清',
  activity: '运营活动',
}

const txTypeLabelMap: Record<string, string> = pointTxTypeOptions.reduce(
  (acc, item) => ({ ...acc, [item.value]: item.label }),
  {},
)

export function pointTxTypeLabel(type: string): string {
  return txTypeLabelMap[type] || type || '—'
}

export function pointTxTypeTheme(type: string): string {
  switch (type) {
    case 'earn_payment':
    case 'earn_renewal':
    case 'earn_activity':
      return 'success'
    case 'spend_exchange':
    case 'expire':
      return 'warning'
    case 'adjust':
      return 'primary'
    default:
      return 'default'
  }
}

export function pointSceneLabel(scene: string): string {
  return pointSceneLabels[scene] || scene || '—'
}

/** 积分数量展示：千分位，正数带 + 号。 */
export function formatPoints(value?: number, signed = false): string {
  const n = Number(value || 0)
  const text = n.toLocaleString('zh-CN')
  return signed && n > 0 ? `+${text}` : text
}

/** 规则计发描述（只读展示）：说明积分怎么来。 */
export function pointRuleSummary(rule: {
  earn_mode: string
  points_per_yuan: number
  fixed_points: number
  min_amount: number
  max_points_per_order: number
  valid_days: number
}): string {
  const parts: string[] = []
  if (rule.earn_mode === 'rate') {
    parts.push(`每消费 1 元得 ${rule.points_per_yuan} 分`)
  } else {
    parts.push(`每笔固定 ${rule.fixed_points} 分`)
  }
  if (rule.min_amount > 0) parts.push(`满 ¥${rule.min_amount} 起算`)
  if (rule.max_points_per_order > 0) parts.push(`单笔上限 ${rule.max_points_per_order} 分`)
  parts.push(rule.valid_days > 0 ? `有效期 ${rule.valid_days} 天` : '永久有效')
  return parts.join(' · ')
}

export function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}
