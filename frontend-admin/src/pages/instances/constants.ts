// 实例运维台 · 常量与格式化工具

export const instanceStatusOptions = [
  { label: '运行中', value: 'running' },
  { label: '已关机', value: 'stopped' },
  { label: '创建中', value: 'creating' },
  { label: '异常', value: 'error' },
  { label: '删除中', value: 'deleting' },
  // deleted 由本系统销毁动作写入（上游适配器不产出该值），此前下拉与标签都没有它。
  { label: '已销毁', value: 'deleted' },
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
  unsuspend: '恢复',
  stage: '阶段推进',
  reinstall: '重装系统',
  reset_password: '重置密码',
  rescue: '进入救援',
  exit_rescue: '退出救援',
  snapshot_create: '创建快照',
  snapshot_delete: '删除快照',
  snapshot_restore: '快照恢复',
  bandwidth: '改带宽',
  add_ip: '增加 IP',
  attach_disk: '挂载数据盘',
}

// 操作结果筛选项（全局操作流水页）
export const operationResultOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '未执行', value: 'skipped' },
]

export const operationResultLabels: Record<string, string> = {
  success: '成功',
  failed: '失败',
  skipped: '未执行',
}

export const operationActionOptions = Object.entries(operationActionLabels).map(([value, label]) => ({ value, label }))

export const operatorTypeOptions = [
  { label: '管理员', value: 'admin' },
  { label: '用户', value: 'user' },
  { label: '系统', value: 'system' },
]

// 生命周期阶段（与后端 lifecyclemodel.Stage* 一致）
export const lifecycleStageLabels: Record<string, string> = {
  active: '服务中',
  grace: '宽限期',
  suspended: '已暂停',
  destroyed: '已销毁',
}

export const lifecycleStageThemes: Record<string, string> = {
  active: 'success',
  grace: 'warning',
  suspended: 'danger',
  destroyed: 'default',
}

// 到期处置动作
export const enforcementActionLabels: Record<string, string> = {
  mark: '登记宽限',
  suspend: '暂停实例',
  destroy: '销毁实例',
  unsuspend: '恢复实例',
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
    case 'deleted':
      // 终态用浅色而非默认灰：与 stopped（可恢复的关机）区分开。
      return 'default'
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

export function operationResultLabel(result: string): string {
  return operationResultLabels[result] || result || '—'
}

export function lifecycleStageLabel(stage: string): string {
  return lifecycleStageLabels[stage] || stage || '—'
}

export function lifecycleStageTheme(stage: string): string {
  return lifecycleStageThemes[stage] || defaultTheme
}

// ===== 实例状态：两个正交维度 =====
//
// 【本轮修复的根因】此前用一个 combineInstanceState 把电源状态与账期状态压成一句话，
// 判据是 `stage === 'suspended' || status === 'stopped'` —— 而上游把 suspended（到期暂停）
// 也映射成本地 stopped，手动关机同样是 stopped。两者无法区分，于是「关机」被显示成
// 「已暂停／到期暂停中」（用户实测反馈）。
//
// 现在拆成两个独立维度，各自回答一个问题：
//   ① 电源维度 powerStateView ：这台机器通电了吗？（运行中 / 已关机 / 重启中 …）
//   ② 服务维度 serviceStateView：我们卖的这个服务还在有效期内吗？（服务中 / 已过期 / 到期暂停 …）
// 两者组合出真实处境，例如「已关机 · 服务中」（客户自己关的机，账期正常）、
// 「运行中 · 已过期」（到期了但客户还没续费、机器也还没被停）。

/** 过渡态（迁移 112）：上游正在执行某个动作，结果尚未确认。 */
export interface InstanceTransitionView {
  state: string
  label: string
  verb: string
  started_at?: string
  expires_at?: string
  /** 已超过窗口仍未确认：不是失败，是"结果未知"，可重试但应先同步。 */
  overdue?: boolean
}

/**
 * powerStateView 电源维度结论。
 *
 * 展示优先级：过渡态 > 运行/关机 > 未知。
 * 过渡中优先展示，因为那正是运营最需要看到的信息（"我点的那下正在跑"）。
 */
export function powerStateView(input: {
  status: string
  power_state?: string
  pending?: boolean
  lifecycle_stage?: string
  transition?: InstanceTransitionView | null
}): { label: string; theme: string; detail: string; spinning: boolean } {
  const stage = input.lifecycle_stage || ''
  const t = input.transition
  // 过渡态优先：显示「重启中」并带动画，避免运营以为没生效而重复点击。
  if (t && t.state) {
    if (t.overdue) {
      return {
        label: `${t.label}（结果未确认）`,
        theme: 'warning',
        detail: `已超过预计耗时仍未确认，请点「同步上游」核对后再决定是否重试`,
        spinning: false,
      }
    }
    return { label: t.label, theme: 'primary', detail: `正在执行：${t.verb}`, spinning: true }
  }
  // 终态与未开通：机器还没起来或已不存在，谈"通电"没有意义。
  if (stage === 'destroyed' || input.status === 'deleted') {
    return { label: '已销毁', theme: 'default', detail: '实例已终止，机器不再运行', spinning: false }
  }
  if (input.pending) {
    return { label: '待开通', theme: 'warning', detail: '尚未在上游开通，机器还未创建', spinning: false }
  }
  if (stage === 'suspended') {
    // 到期暂停是我方动作：机器侧通常已被上游关机，但语义是"服务被停"而非"用户关机"。
    return { label: '已停止服务', theme: 'danger', detail: '到期暂停中，续费后自动恢复运行', spinning: false }
  }
  switch (input.power_state || '') {
    case 'on':
      return { label: '运行中', theme: 'success', detail: '机器已通电运行', spinning: false }
    case 'off':
      // 关键修复：手动关机就是"已关机"，不再写成「已暂停」。
      return { label: '已关机', theme: 'default', detail: '机器已断电（可由本页「开机」恢复）', spinning: false }
    case 'transitioning':
      return { label: '状态同步中', theme: 'primary', detail: '正在从上游获取最新状态', spinning: true }
  }
  // 回落到原始 status（创建中/删除中等过渡状态，或后端未下发 power_state 的旧响应）。
  return {
    label: instanceStatusLabel(input.status),
    theme: instanceStatusTheme(input.status),
    detail: '',
    spinning: input.status === 'creating' || input.status === 'deleting',
  }
}

/**
 * serviceStateView 服务/账期维度结论（与机器是否开机无关）。
 *
 * 一台关机但未到期的实例，这里仍是「服务中」—— 服务在有效期内，只是机器没开。
 */
export function serviceStateView(input: {
  lifecycle_stage?: string
  expire_state?: string
  service_state?: string
  days_left?: number
}): { label: string; theme: string; detail: string } {
  const stage = input.lifecycle_stage || ''
  if (stage === 'destroyed') {
    return { label: '服务已终止', theme: 'default', detail: '账期终结，不可恢复' }
  }
  if (stage === 'suspended') {
    return { label: '到期暂停', theme: 'danger', detail: '已过宽限期，续费后自动恢复服务' }
  }
  switch (input.service_state || '') {
    case 'expiring': {
      const d = typeof input.days_left === 'number' ? input.days_left : undefined
      return {
        label: '即将到期',
        theme: 'warning',
        detail: d !== undefined ? `还剩 ${d} 天，建议提前续费` : '临近到期，建议提前续费',
      }
    }
    case 'expired':
      return { label: '已过期', theme: 'danger', detail: '已过到期时间，请尽快续费以免服务被停' }
    case 'grace':
      return { label: '已过期·宽限期', theme: 'warning', detail: '宽限期内仍可服务，逾期将暂停' }
    case 'active':
      return { label: '服务中', theme: 'success', detail: '账期正常' }
  }
  // 回落：后端未下发 service_state 时按旧口径推算（兼容旧响应）。
  if (input.expire_state === 'expired') {
    return { label: stage === 'grace' ? '已过期·宽限期' : '已过期', theme: 'warning', detail: '已过到期时间，请尽快续费' }
  }
  if (input.expire_state === 'expiring') {
    return { label: '即将到期', theme: 'warning', detail: '临近到期，建议提前续费' }
  }
  return { label: '服务中', theme: 'success', detail: '账期正常' }
}

/**
 * combineInstanceState 一句话结论（列表徽标用）。
 *
 * 保留单一结论是因为列表一行放不下两个标签；但结论现在**分别取自两个维度**，
 * 不会再出现"关机 → 已暂停"的错误归因：
 *   终态/待开通 > 过渡中 > 服务异常（到期暂停/过期）> 电源态
 * 其中"服务异常"优先于电源态：到期暂停的机器即使开着也应当先提示欠费。
 *
 * 需要完整画面时请在详情页用 powerStateView + serviceStateView 两个标签。
 */
export function combineInstanceState(input: {
  status: string
  lifecycle_stage?: string
  expire_state?: string
  service_state?: string
  power_state?: string
  days_left?: number
  pending?: boolean
  transition?: InstanceTransitionView | null
}): { label: string; theme: string; detail: string; spinning: boolean } {
  const stage = input.lifecycle_stage || ''
  if (stage === 'destroyed' || input.status === 'deleted') {
    return { label: '已销毁', theme: 'default', detail: '实例已终止，不可恢复', spinning: false }
  }
  if (input.pending) {
    return {
      label: '待开通',
      theme: 'warning',
      detail: '尚未在上游开通：可在详情页查看开通失败原因并「重新开通」',
      spinning: false,
    }
  }
  // 过渡中优先于普通电源态：这是"操作正在跑"的反馈。
  const power = powerStateView(input)
  if (power.spinning) {
    return { label: power.label, theme: power.theme, detail: power.detail, spinning: true }
  }
  // 服务维度异常时以它为准（欠费/过期比"机器开着"更需要运营立刻看到）。
  const service = serviceStateView(input)
  if (stage === 'suspended') {
    return { label: '到期暂停', theme: 'danger', detail: service.detail, spinning: false }
  }
  if (service.theme === 'danger' || service.theme === 'warning') {
    // 机器状态作为补充：让运营知道"虽然过期了，但机器现在还开着/已关"。
    return {
      label: service.label,
      theme: service.theme,
      detail: `${power.label} · ${service.detail}`,
      spinning: false,
    }
  }
  return { label: power.label, theme: power.theme, detail: power.detail, spinning: false }
}

export function enforcementActionLabel(action: string): string {
  return enforcementActionLabels[action] || action || '—'
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

/**
 * 内存展示：数据库以 MB 存储，>=1024 转 GB。
 *
 * 与管理端列表页原有的 `${row.memory} MB` 写法并存：列表里"8192 MB"还能扫读，
 * 详情页主信息区一行要同时放 CPU/内存/磁盘三组数字，"8192 MB" 会把行撑长，
 * 因此详情统一走本函数。阈值取 1024（不是 1000）：机器内存都是 2 的幂。
 */
export function formatMemory(mb: number): string {
  const value = Number(mb || 0)
  if (value <= 0) return '—'
  if (value >= 1024) {
    const gb = value / 1024
    return `${Number.isInteger(gb) ? gb : gb.toFixed(1)} GB`
  }
  return `${value} MB`
}

/**
 * 产品类型展示名（cloud_host → 云主机）。
 *
 * 取值与后端登记表（backend/internal/pkg/model/product_kind.go）一一对应，
 * 由 product_kind_align_test.go 门禁校验。未登记的取值原样返回 ——
 * 不隐藏信息：显示一个陌生值也比显示空白更容易发现后端多了一条产品线。
 */
const productKindLabels: Record<string, string> = {
  cloud_host: '云主机',
  cdn: 'CDN',
  physical: '物理机',
  ip: 'IP 地址',
  storage: '存储',
  bandwidth: '带宽',
  other: '其它',
}

export function productKindLabel(value?: string): string {
  if (!value) return '其它'
  return productKindLabels[value] || value
}
