// 消息中心 · 多渠道（doc90）前端常量与展示口径
// 枚举与后端 model/channel.go、model/delivery.go、pkg/notifier 保持一致。

// ===== 通道类别 =====
export const categoryOptions = [
  { label: '邮件渠道', value: 'mail' },
  { label: '短信渠道', value: 'sms' },
]

export function categoryLabel(category: string): string {
  const found = categoryOptions.find((item) => item.value === category)
  return found ? found.label : category || '—'
}

export function categoryTheme(category: string): string {
  switch (category) {
    case 'mail':
      return 'primary'
    case 'sms':
      return 'success'
    default:
      return 'default'
  }
}

// ===== 渠道场景（channel.scenes 数组取值）=====
export const channelSceneOptions = [
  { label: '验证码（强制送达）', value: 'otp' },
  { label: '业务通知', value: 'notify' },
  { label: '告警', value: 'alert' },
  { label: '营销', value: 'marketing' },
  { label: '测试发送', value: 'test' },
]

export function channelSceneLabel(scene: string): string {
  const found = channelSceneOptions.find((item) => item.value === scene)
  return found ? found.label : scene
}

// ===== 渠道健康状态 =====
export function channelHealthLabel(status: string): string {
  switch (status) {
    case 'healthy':
      return '正常'
    case 'down':
      return '异常'
    case 'pending':
      return '待接入'
    default:
      return '未检测'
  }
}

export function channelHealthTheme(status: string): string {
  switch (status) {
    case 'healthy':
      return 'success'
    case 'down':
      return 'danger'
    case 'pending':
      return 'default'
    default:
      return 'warning'
  }
}

// ===== 快速测试渠道类型 =====
export function channelModeLabel(mode: string): string {
  switch (mode) {
    case 'api':
      return '接口自动'
    case 'manual':
      return '线下人工'
    default:
      return mode || '—'
  }
}

// ===== 投递状态（六态，doc90 §5.2）=====
export const deliveryStatusOptions = [
  { label: '待发送', value: 'pending' },
  { label: '发送中', value: 'sending' },
  { label: '已发送', value: 'sent' },
  { label: '失败（可重投）', value: 'failed' },
  { label: '已跳过（配置问题）', value: 'skipped' },
  { label: '死信', value: 'dead' },
]

const deliveryStatusMap: Record<string, { label: string; theme: string }> = {
  pending: { label: '待发送', theme: 'warning' },
  sending: { label: '发送中', theme: 'primary' },
  sent: { label: '已发送', theme: 'success' },
  failed: { label: '失败', theme: 'danger' },
  skipped: { label: '已跳过', theme: 'default' },
  dead: { label: '死信', theme: 'danger' },
}

export function deliveryStatusLabel(status: string): string {
  return deliveryStatusMap[status]?.label || status || '—'
}

export function deliveryStatusTheme(status: string): string {
  return deliveryStatusMap[status]?.theme || 'default'
}

/** 仅 failed / dead / skipped 允许重投（与后端 retryable 一致）。 */
export function deliveryRetryable(status: string): boolean {
  return status === 'failed' || status === 'dead' || status === 'skipped'
}

// ===== 投递通道 =====
const deliveryChannelMap: Record<string, { label: string; theme: string }> = {
  inbox: { label: '站内信', theme: 'primary' },
  mail: { label: '邮件', theme: 'success' },
  sms: { label: '短信', theme: 'warning' },
}

export function deliveryChannelLabel(channel: string): string {
  return deliveryChannelMap[channel]?.label || channel || '—'
}

export function deliveryChannelTheme(channel: string): string {
  return deliveryChannelMap[channel]?.theme || 'default'
}

export const deliveryChannelOptions = [
  { label: '站内信', value: 'inbox' },
  { label: '邮件', value: 'mail' },
  { label: '短信', value: 'sms' },
]

// ===== 短信模板场景 =====
export const smsSceneOptions = [
  { label: '验证码', value: 'otp' },
  { label: '业务通知', value: 'notify' },
  { label: '告警', value: 'alert' },
]

export function smsSceneLabel(scene: string): string {
  const found = smsSceneOptions.find((item) => item.value === scene)
  return found ? found.label : scene || '—'
}

// ===== 通用 =====
export function formatTime(value?: string): string {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

/** 金额分 → 元显示。 */
export function fenToYuan(fen: number): string {
  return (Number(fen || 0) / 100).toFixed(2)
}

/** 模板变量占位符文本：{var_key}。
 *
 * 之所以用函数而不是在模板里写反引号插值：`{{ `{${k}}` }}` 里的 `}}` 会被
 * Vue 模板解析器当成插值结束符，直接编译报错。
 */
export function varToken(key: string): string {
  return '{' + key + '}'
}

/** 正文格式（text/html）。 */
export const formatOptions = [
  { label: '纯文本', value: 'text' },
  { label: 'HTML 富文本', value: 'html' },
]

export function formatLabel(format: string): string {
  return format === 'html' ? 'HTML' : '纯文本'
}
