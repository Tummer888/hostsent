// 积分中心 · 常量与格式化工具（docs/实施计划/36）
//
// 边界提示：积分独立于余额，不能抵扣订单/账单，也不能提现或转入余额。

/** 发放场景（与后端 point_rules.scene 对应）。 */
export const pointSceneOptions = [
  { label: '订单购买', value: 'order_purchase' },
  { label: '订单续费', value: 'order_renewal' },
  { label: '账单结清', value: 'bill_payment' },
  { label: '运营活动', value: 'activity' },
]

/** 发放模式。 */
export const pointEarnModeOptions = [
  { label: '按金额比例', value: 'rate' },
  { label: '固定积分', value: 'fixed' },
]

/** 规则启用状态。 */
export const pointRuleStatusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

/** 积分流水类型。 */
export const pointTxTypeOptions = [
  { label: '订单发放', value: 'earn_payment' },
  { label: '续费发放', value: 'earn_renewal' },
  { label: '活动发放', value: 'earn_activity' },
  { label: '兑换消耗', value: 'spend_exchange' },
  { label: '人工调整', value: 'adjust' },
  { label: '到期回收', value: 'expire' },
]

/** 流水方向。 */
export const pointDirectionOptions = [
  { label: '获得', value: 1 },
  { label: '消耗', value: -1 },
]

const sceneLabelMap: Record<string, string> = pointSceneOptions.reduce(
  (acc, item) => ({ ...acc, [item.value]: item.label }),
  {},
)

const txTypeLabelMap: Record<string, string> = pointTxTypeOptions.reduce(
  (acc, item) => ({ ...acc, [item.value]: item.label }),
  {},
)

export function pointSceneLabel(scene: string): string {
  return sceneLabelMap[scene] || scene || '—'
}

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

export function pointEarnModeLabel(mode: string): string {
  const found = pointEarnModeOptions.find((item) => item.value === mode)
  return found ? found.label : mode || '—'
}

/** 规则计发描述：按比例 / 固定积分 + 门槛 + 上限 + 有效期。 */
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
    parts.push(`每 1 元得 ${rule.points_per_yuan} 分`)
  } else {
    parts.push(`每笔固定 ${rule.fixed_points} 分`)
  }
  if (rule.min_amount > 0) parts.push(`满 ¥${rule.min_amount} 起算`)
  if (rule.max_points_per_order > 0) parts.push(`单笔上限 ${rule.max_points_per_order} 分`)
  parts.push(rule.valid_days > 0 ? `${rule.valid_days} 天有效` : '永久有效')
  return parts.join(' · ')
}

export function pointRuleStatusLabel(status: number): string {
  return status === 1 ? '启用' : '停用'
}

export function pointRuleStatusTheme(status: number): string {
  return status === 1 ? 'success' : 'default'
}

export function formatPoints(value: number): string {
  return Number(value || 0).toLocaleString('zh-CN')
}

export function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}
