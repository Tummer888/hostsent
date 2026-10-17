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
  // restarting 由 dcimcloud 的 wait_reboot/task 映射而来：机器仍在通电重启。
  restarting: '重启中',
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
 * 过渡态（迁移 112）：平台正在执行某个动作，结果尚未确认。
 *
 * 客户侧同样需要看到"我点的那下正在跑" —— 否则会以为没生效而反复点重启，
 * 同一台机器堆多次指令。
 */
export interface InstanceTransitionView {
  state: string
  label: string
  verb: string
  started_at?: string
  expires_at?: string
  overdue?: boolean
}

/**
 * powerStateView 机器电源维度（与账期无关）—— 客户读的版本。
 *
 * 【本会话修复的根因】此前把电源状态与账期状态压成一句话，判据是
 * `stage === 'suspended' || status === 'stopped'`，而上游把 suspended 也映射成
 * stopped，客户手动关机同样是 stopped —— 于是"我关了机"被显示成「已暂停」。
 * 现在两个维度分开：这里只答"机器通电了吗"。
 */
export function powerStateView(input: {
  status: string
  power_state?: string
  pending?: boolean
  lifecycle_stage?: string
  /** 服务维度（同一次响应下发）：违规暂停要把机器文案换成"服务被停"。 */
  service_state?: string
  transition?: InstanceTransitionView | null
}): { label: string; theme: string; detail: string; spinning: boolean } {
  const stage = input.lifecycle_stage || ''
  const t = input.transition
  if (t && t.state) {
    if (t.overdue) {
      return {
        label: `${t.label}（结果未确认）`,
        theme: 'warning',
        detail: '已超过预计耗时仍未确认，可刷新页面或联系客服核对',
        spinning: false,
      }
    }
    return { label: t.label, theme: 'primary', detail: `正在${t.verb}，请稍候`, spinning: true }
  }
  if (stage === 'destroyed' || input.status === 'deleted') {
    return { label: '已销毁', theme: 'default', detail: '实例已回收，不可恢复', spinning: false }
  }
  // 已取消：订单未付款/作废，机器从未创建 —— 谈不上"通电状态"。
  if (stage === 'cancelled' || input.status === 'cancelled') {
    return { label: '未开通', theme: 'default', detail: '订单未支付或已作废，实例未创建', spinning: false }
  }
  if (input.pending) {
    return { label: '开通中', theme: 'warning', detail: '正在为你创建实例', spinning: false }
  }
  // 违规暂停：机器侧可能仍通电，但对客户是"服务被停"且需联系客服。
  if (input.service_state === 'suspended_violation') {
    return { label: '已停止服务', theme: 'danger', detail: '因违规被暂停，请联系客服处理', spinning: false }
  }
  if (stage === 'suspended') {
    // 到期暂停是平台动作：机器侧通常已被关机，但对客户而言语义是"服务被停"。
    return { label: '已停止服务', theme: 'danger', detail: '实例已到期暂停，续费后自动恢复运行', spinning: false }
  }
  switch (input.power_state || '') {
    case 'on':
      return { label: '运行中', theme: 'success', detail: '实例运行正常', spinning: false }
    case 'off':
      return { label: '已关机', theme: 'default', detail: '实例已关机，可点击「开机」启动', spinning: false }
    case 'transitioning':
      return { label: '状态同步中', theme: 'primary', detail: '正在获取最新状态', spinning: true }
  }
  const label = instanceStatusLabels[input.status] || input.status || '未知'
  let theme = 'default'
  if (input.status === 'running') theme = 'success'
  else if (input.status === 'creating' || input.status === 'restarting') theme = 'warning'
  else if (input.status === 'error') theme = 'danger'
  return { label, theme, detail: '', spinning: input.status === 'creating' || input.status === 'restarting' }
}

/**
 * serviceStateView 账期/服务维度（与机器是否开机无关）—— 客户读的版本。
 *
 * 关着机但没到期，这里仍是「服务中」：服务在有效期内，只是机器没开。
 */
export function serviceStateView(input: {
  status?: string
  lifecycle_stage?: string
  expire_state?: string
  service_state?: string
  days_left?: number
}): { label: string; theme: string; detail: string } {
  const stage = input.lifecycle_stage || ''
  if (stage === 'destroyed') {
    return { label: '服务已终止', theme: 'default', detail: '服务已结束' }
  }
  // 已取消：订单未付款超时/作废 —— 这台机器从未开通。
  if (stage === 'cancelled') {
    return { label: '已取消', theme: 'default', detail: '订单未支付或已作废，实例未开通' }
  }
  // 平台侧已删除（同步只落 status、不改阶段）：机器已不存在，服务随之终结。
  // 只认 deleted —— 关机（stopped）是电源维度，不能算成服务终止。
  if (input.status === 'deleted') {
    return { label: '服务已终止', theme: 'default', detail: '实例已由平台回收，如有疑问请联系客服' }
  }
  // 违规暂停必须先于"到期暂停"判断：前者续费解决不了，要走客服处理。
  if (input.service_state === 'suspended_violation') {
    return { label: '违规暂停', theme: 'danger', detail: '因违规被暂停，请联系客服处理' }
  }
  if (stage === 'suspended') {
    return { label: '已到期暂停', theme: 'danger', detail: '续费后可恢复服务' }
  }
  switch (input.service_state || '') {
    // 尚未开通：没有交付物，不能说成"服务中"。
    case 'pending':
      return { label: '待开通', theme: 'warning', detail: '订单已提交，实例正在创建' }
    case 'cancelled':
      return { label: '已取消', theme: 'default', detail: '订单未支付或已作废，实例未开通' }
    case 'expiring': {
      const d = typeof input.days_left === 'number' ? input.days_left : undefined
      return {
        label: '即将到期',
        theme: 'warning',
        detail: d !== undefined ? `还剩 ${d} 天，建议提前续费` : '临近到期，建议提前续费',
      }
    }
    case 'expired':
      return { label: '已过期', theme: 'danger', detail: '已过到期时间，请尽快续费' }
    case 'grace':
      return { label: '已过期·宽限期', theme: 'warning', detail: '宽限期内仍可使用，请尽快续费' }
    case 'active':
      return { label: '服务中', theme: 'success', detail: '服务在有效期内' }
  }
  if (input.expire_state === 'expired') {
    return { label: stage === 'grace' ? '已过期·宽限期' : '已过期', theme: 'warning', detail: '已过到期时间，请尽快续费' }
  }
  if (input.expire_state === 'expiring') {
    return { label: '即将到期', theme: 'warning', detail: '临近到期，建议提前续费' }
  }
  return { label: '服务中', theme: 'success', detail: '服务在有效期内' }
}

/**
 * combineInstanceState 一句话客户结论（列表徽标用；详情页用两个维度分开显示）。
 *
 * 结论分别取自两个维度，不再出现"关机 → 已暂停"的错误归因：
 * 终态/开通中 > 过渡中 > 服务异常（到期暂停/过期）> 电源态。
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
    return { label: '已销毁', theme: 'default', detail: '实例已回收，不可恢复', spinning: false }
  }
  // 已取消排在"开通中"之前：取消后的行仍没有上游实例号，
  // 先判 pending 会把一张已作废的订单显示成"正在为你创建实例"。
  if (stage === 'cancelled' || input.status === 'cancelled') {
    return { label: '已取消', theme: 'default', detail: '订单未支付或已作废，实例未开通', spinning: false }
  }
  if (input.pending) {
    return { label: '开通中', theme: 'warning', detail: '正在为你创建实例，可在详情页查看进度', spinning: false }
  }
  const power = powerStateView(input)
  if (power.spinning) {
    return { label: power.label, theme: power.theme, detail: power.detail, spinning: true }
  }
  const service = serviceStateView(input)
  if (service.theme === 'danger' || service.theme === 'warning') {
    return {
      label: service.label,
      theme: service.theme,
      detail: `${power.label} · ${service.detail}`,
      spinning: false,
    }
  }
  return { label: power.label, theme: power.theme, detail: power.detail, spinning: false }
}

/**
 * 状态图标名：与 combineInstanceState 的结论一一对应。
 *
 * 返回图标名（而不是组件）：constants 是纯 TS 工具，引组件会把视图依赖带进
 * 非视图模块；页面拿到名字后自行映射为图标组件。
 * 图标的作用是让状态可"扫视"——纯文字 tag 一连串排下来，找"哪台关机了"要逐行读。
 */
export function stateIcon(label: string): string {
  switch (label) {
    case '运行中':
      return 'play-circle'
    // 「已关机」用暂停图标：对客户而言"关机"就是服务暂时停着，与「已停止服务」
    // 区分在于后者是平台因到期停的（文案已说明）。
    case '已关机':
    case '已停止服务':
      return 'pause-circle'
    case '已到期暂停':
      return 'pause-circle'
    case '开机中':
    case '关机中':
    case '重启中':
    case '重装中':
    case '重置密码中（结果未确认）':
    case '状态同步中':
    case '开通中':
      return 'time'
    case '已过期':
    case '已过期·宽限期':
      return 'error-circle'
    case '已销毁':
      return 'delete'
    case '异常':
      return 'close-circle'
    default:
      // 带"（结果未确认）"后缀的过渡态统一归到时间图标。
      if (label.startsWith('开机中') || label.startsWith('关机中') || label.startsWith('重启中') || label.startsWith('重装中')) {
        return 'time'
      }
      return 'info-circle'
  }
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
