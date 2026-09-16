// 用户管理 · 常量与格式化工具
//
// 本模块只定义「用户域自己」的枚举（账号状态、分层、认证状态等）。
// 订单 / 工单 / 实例 / 财务的状态标签直接复用对应模块的既有实现，
// 不再各自维护一份 —— 历史上详情页有三套写歪的映射表，把 active 显示成
// 「已支付」、把 running 显示成「运行中」以外的错误值。

export { formatPrice, formatTime, orderStatusLabel, orderStatusTheme, payMethodLabel } from '@/pages/order/constants'
export { ticketStatusLabel, ticketStatusTheme, ticketPriorityLabel, ticketPriorityTheme } from '@/pages/ticket/constants'
export { instanceStatusLabel, instanceStatusTheme, expireStateLabel, expireStateTheme } from '@/pages/instances/constants'
export { txTypeLabel, txTypeTheme, directionLabel, billStatusLabel, billStatusTheme } from '@/pages/finance/constants'

const defaultTheme = 'default'

// ---------- 账号状态 ----------

// 与 users.status 的实际取值对齐（实测库中只有 active / disabled，
// 但状态机允许 pending / cancelled，一并给出文案以免渲染出原始英文）。
export const userStatusOptions = [
  { label: '正常', value: 'active' },
  { label: '待审核', value: 'pending' },
  { label: '已禁用', value: 'disabled' },
  { label: '已注销', value: 'cancelled' },
]

export function userStatusLabel(status: string): string {
  const found = userStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function userStatusTheme(status: string): string {
  switch (status) {
    case 'active':
      return 'success'
    case 'pending':
      return 'warning'
    case 'disabled':
      return 'danger'
    case 'cancelled':
    default:
      return defaultTheme
  }
}

// ---------- 用户分层 ----------

export const userTierOptions = [
  { label: '免费', value: 'free' },
  { label: '专业', value: 'pro' },
  { label: '企业', value: 'enterprise' },
]

export function userTierLabel(tier: string): string {
  const found = userTierOptions.find((item) => item.value === tier)
  return found ? found.label : tier || '—'
}

export function userTierTheme(tier: string): string {
  switch (tier) {
    case 'pro':
      return 'primary'
    case 'enterprise':
      return 'warning'
    case 'free':
    default:
      return defaultTheme
  }
}

// ---------- OAuth 渠道 ----------

export const oauthProviderLabels: Record<string, string> = {
  wechat: '微信',
  alipay: '支付宝',
  qq: 'QQ',
  github: 'GitHub',
  wecom: '企业微信',
}

export function oauthProviderLabel(provider: string): string {
  if (!provider) return '—'
  return oauthProviderLabels[provider] || provider
}

// ---------- 客户侧权限码（对齐后端 pkg/auth/user_permission.go） ----------

export const customerPermissionLabels: Record<string, string> = {
  'instance:view': '查看实例',
  'instance:operate': '实例操作',
  'order:view': '查看订单',
  'order:create': '下单/续费',
  'ticket:view': '查看工单',
  'ticket:submit': '提交工单',
  'billing:view': '查看账单',
  'subaccount:manage': '成员管理',
}

/** 子账号可勾选的权限码顺序；主账号天然拥有全部客户侧权限。 */
export const customerPermissionOptions = Object.keys(customerPermissionLabels).map((code) => ({
  label: customerPermissionLabels[code],
  value: code,
}))

export function customerPermissionLabel(code: string): string {
  return customerPermissionLabels[code] || code || '—'
}

// ---------- 订单计费周期（后台代下单用） ----------

export const billingCycleOptions = [
  { label: '月付', value: 'monthly' },
  { label: '季付', value: 'quarterly' },
  { label: '半年付', value: 'semiannually' },
  { label: '年付', value: 'annually' },
]

// ---------- 登录 / 安全枚举 ----------

export function loginResultLabel(result: string): string {
  switch (result) {
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    default:
      return result || '—'
  }
}

export function loginResultTheme(result: string): string {
  return result === 'success' ? 'success' : result === 'failed' ? 'danger' : defaultTheme
}

export function sessionStatusLabel(status: string): string {
  switch (status) {
    case 'active':
      return '有效'
    case 'expired':
      return '已过期'
    case 'revoked':
      return '已下线'
    default:
      return status || '—'
  }
}

export function sessionStatusTheme(status: string): string {
  switch (status) {
    case 'active':
      return 'success'
    case 'revoked':
      return 'danger'
    case 'expired':
    default:
      return defaultTheme
  }
}

export function riskLevelLabel(level: string): string {
  switch (level) {
    case 'low':
      return '低'
    case 'medium':
      return '中'
    case 'high':
      return '高'
    case 'critical':
      return '严重'
    default:
      return level || '—'
  }
}

export function riskLevelTheme(level: string): string {
  switch (level) {
    case 'critical':
    case 'high':
      return 'danger'
    case 'medium':
      return 'warning'
    case 'low':
    default:
      return defaultTheme
  }
}

export function riskEventStatusLabel(status: string): string {
  switch (status) {
    case 'pending':
      return '待处理'
    case 'handled':
      return '已处理'
    case 'ignored':
      return '已忽略'
    default:
      return status || '—'
  }
}

export function riskEventStatusTheme(status: string): string {
  switch (status) {
    case 'pending':
      return 'warning'
    case 'handled':
      return 'success'
    case 'ignored':
    default:
      return defaultTheme
  }
}

// ---------- 实名认证 ----------

export function verificationTypeLabel(type: string): string {
  switch (type) {
    case 'personal':
      return '个人认证'
    case 'enterprise':
      return '企业认证'
    default:
      return type || '—'
  }
}

export function verificationStatusLabel(status: string): string {
  switch (status) {
    case 'pending':
      return '待审核'
    case 'approved':
      return '已通过'
    case 'rejected':
      return '已驳回'
    default:
      return status || '—'
  }
}

export function verificationStatusTheme(status: string): string {
  switch (status) {
    case 'approved':
      return 'success'
    case 'pending':
      return 'warning'
    case 'rejected':
      return 'danger'
    default:
      return defaultTheme
  }
}

// ---------- 角色域 ----------

export function roleScopeLabel(scope: string): string {
  switch (scope) {
    case 'admin':
      return '后台角色'
    case 'user':
      return '客户角色'
    default:
      return scope || '—'
  }
}

// ---------- 聚合字段 → 展示文案 ----------

export function instanceSpecsText(cpu: number, memory: number, disk: number): string {
  const parts: string[] = []
  if (cpu > 0) parts.push(`${cpu} 核`)
  if (memory > 0) parts.push(`${memory} MB`)
  if (disk > 0) parts.push(`${disk} GB`)
  return parts.length ? parts.join(' / ') : '规格待同步'
}

export function billingModeLabel(mode: string): string {
  switch (mode) {
    case 'hourly':
      return '按量计费'
    case 'monthly':
      return '包月'
    case 'yearly':
      return '包年'
    default:
      return mode || '—'
  }
}

/** 到期时间：null 表示未设置，必须原样显示「未设置」，不能伪造日期。 */
export function expireText(expireAt: string | null): string {
  if (!expireAt) return '未设置'
  const date = new Date(expireAt)
  if (Number.isNaN(date.getTime())) return '未设置'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** 金额（元）→ 千分位两位小数。 */
export function formatAmount(value: number | null | undefined): string {
  return Number(value || 0).toFixed(2)
}

/** ISO 时间 → YYYY-MM-DD HH:mm。 */
export function formatDateTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}