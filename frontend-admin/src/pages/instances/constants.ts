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

// combineInstanceState 把「服务状态 / 到期阶段 / 到期状态」合成一句人读得懂的结论。
//
// 三个字段语义不同（服务状态来自上游、阶段是本系统的到期处置、到期状态是时间派生），
// 单独看容易互相矛盾——例如宽限期内的实例 status=running 而 expire_state=expired，
// 列表只显示后者就会写成「运行中（已过期）」。这里按处置优先级给唯一结论：
// 已销毁 > 已暂停 > 宽限期(已过期仍服务) > 正常服务状态，并保留阶段作为补充说明。
export function combineInstanceState(input: {
  status: string
  lifecycle_stage?: string
  expire_state?: string
}): { label: string; theme: string; detail: string } {
  const stage = input.lifecycle_stage || ''
  if (stage === 'destroyed' || input.status === 'deleted') {
    return { label: '已销毁', theme: 'default', detail: '实例已终止，不可恢复' }
  }
  if (stage === 'suspended' || input.status === 'stopped') {
    return {
      label: '已暂停',
      theme: 'danger',
      detail: stage === 'suspended' ? '到期暂停中，续费后自动恢复' : '已关机',
    }
  }
  if (input.expire_state === 'expired') {
    return {
      label: stage === 'grace' ? '已过期·宽限期' : '已过期',
      theme: 'warning',
      detail: '已过到期时间，宽限期内仍可服务，请尽快续费',
    }
  }
  return {
    label: instanceStatusLabel(input.status),
    theme: instanceStatusTheme(input.status),
    detail: input.expire_state === 'expiring' ? '即将到期' : '',
  }
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
