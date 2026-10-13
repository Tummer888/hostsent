// 用户端 · 云主机常量与格式化工具。
//
// 与前端管理端 src/pages/instances/constants.ts 同源（combineInstanceState 是同一套
// 结论规则），差异只有文案口吻：管理端写给运营，这里写给客户。
//
// 为什么两端不共用一份：管理端有筛选下拉、批量动作、处置阶段等运营概念，
// 用户端只读展示；共用会把这些运营词汇泄漏到客户界面。

export const instanceStatusLabels: Record<string, string> = {
  running: '运行中',
  stopped: '已关机',
  creating: '创建中',
  error: '异常',
  deleting: '删除中',
  deleted: '已销毁',
}

export const billingModeLabels: Record<string, string> = {
  hourly: '按小时',
  daily: '按天',
  monthly: '按月',
  quarterly: '按季',
  semiannually: '按半年',
  annually: '按年',
  biennially: '按两年',
  triennially: '按三年',
  onetime: '一次性',
}

export const operationActionLabels: Record<string, string> = {
  power_on: '开机',
  power_off: '关机',
  hard_off: '强制关机',
  reboot: '重启',
  hard_reboot: '强制重启',
  vnc: '远程控制台',
  sync: '同步刷新',
  resize: '变更配置',
  destroy: '销毁',
  remark: '备注',
  suspend: '暂停',
  unsuspend: '恢复',
  stage: '到期处置',
  reinstall: '重装系统',
  reset_password: '重置密码',
  rescue: '进入救援',
  exit_rescue: '退出救援',
  snapshot_create: '创建快照',
  snapshot_delete: '删除快照',
  snapshot_restore: '快照恢复',
  bandwidth: '修改带宽',
  add_ip: '增加 IP',
  attach_disk: '挂载数据盘',
  provision_retry: '重新开通',
}

export const operationResultLabels: Record<string, string> = {
  success: '成功',
  failed: '失败',
  skipped: '未执行',
}

export function billingModeLabel(mode: string): string {
  return billingModeLabels[mode] || mode || '—'
}

export function operationActionLabel(action: string): string {
  return operationActionLabels[action] || action || '—'
}

export function operationResultLabel(result: string): string {
  return operationResultLabels[result] || result || '—'
}

export function operationResultTheme(result: string): string {
  switch (result) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'skipped':
      return 'warning'
    default:
      return 'default'
  }
}

export function operatorTypeLabel(type: string): string {
  switch (type) {
    case 'admin':
      return '平台'
    case 'user':
      return '我'
    case 'system':
      return '系统'
    default:
      return type || '—'
  }
}

/**
 * combineInstanceState 把「服务状态 / 生命周期阶段 / 到期状态」合成一句客户读得懂的结论。
 *
 * 三个字段语义不同：status 来自上游（关机就是 stopped）、lifecycle_stage 是本系统的
 * 到期处置阶段、expire_state 是时间派生。单独看会自相矛盾——宽限期内实例
 * status=running 而 expire_state=expired，只显示 status 会把已过期的机器写成「运行中」。
 * 按处置优先级给唯一结论：已销毁 > 待开通 > 已暂停 > 已过期(宽限) > 正常服务状态。
 */
export function combineInstanceState(input: {
  status: string
  lifecycle_stage?: string
  expire_state?: string
  pending?: boolean
}): { label: string; theme: string; detail: string } {
  const stage = input.lifecycle_stage || ''
  if (stage === 'destroyed' || input.status === 'deleted') {
    return { label: '已销毁', theme: 'default', detail: '实例已终止，不可恢复' }
  }
  if (input.pending) {
    return { label: '待开通', theme: 'warning', detail: '尚未开通完成，可在详情页查看原因并重新开通' }
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
  const label = instanceStatusLabels[input.status] || input.status || '未知'
  let theme = 'default'
  if (input.status === 'running') theme = 'success'
  else if (input.status === 'creating') theme = 'warning'
  else if (input.status === 'error') theme = 'danger'
  return { label, theme, detail: input.expire_state === 'expiring' ? '即将到期，请及时续费' : '' }
}

/** 内存展示：MB → 人类可读（>=1024 转 GB，保留必要小数）。 */
export function formatMemory(mb: number): string {
  const value = Number(mb || 0)
  if (value <= 0) return '—'
  if (value >= 1024) {
    const gb = value / 1024
    return `${Number.isInteger(gb) ? gb : gb.toFixed(1)} GB`
  }
  return `${value} MB`
}

/** 时间格式化（ISO → "YYYY-MM-DD HH:mm"）。 */
export function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** 剩余天数文案（与到期状态配合展示）。 */
export function daysLeftText(days: number, expireState?: string): string {
  if (expireState === 'none') return '未设置'
  if (days < 0) return `已过期 ${Math.abs(days)} 天`
  if (days === 0) return '今天到期'
  return `剩余 ${days} 天`
}

/** 剩余天数配色（负值红、临期橙、其余常规）。 */
export function daysLeftTheme(days: number, expireState?: string): string {
  if (expireState === 'none') return 'default'
  if (days < 0) return 'danger'
  if (days <= 7) return 'warning'
  return 'default'
}
