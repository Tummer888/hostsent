export const securityRiskTagTheme: Record<string, string> = {
  low: 'default',
  medium: 'warning',
  high: 'danger',
  critical: 'danger',
}

export const securityStatusTagTheme: Record<string, string> = {
  success: 'success',
  failed: 'danger',
  pending: 'warning',
  ignored: 'default',
  handled: 'success',
  active: 'success',
  inactive: 'default',
  revoked: 'danger',
  expired: 'warning',
}

// —— 枚举取值对齐（doc104 F7）——
//
// 下面这些常量是**后端真实落库的值**，不是界面文案。此前各页面的筛选下拉用的是
// 自己编的中文别名（例如登录类型写 admin/user、会话状态写 online），与库里对不上，
// 选中后一律返回空列表。展示文案统一走 *_LABEL 映射，筛选与渲染不会再次分叉。

/**
 * 登录日志 login_type。
 *
 * 除 uc/auth 写入的 password/sms/email 外，还有两条真实来源：
 *   - oauth：uc/oauth 登录时写的是 provider 名（wechat/qq/alipay/…）；
 *   - impersonate：管理端代登录（见 server/assembly_impersonation.go）。
 * 少列这两类会让安全页显示原始英文值，且按类型筛选时选不到它们。
 */
export const LOGIN_TYPE_OPTIONS = [
  { label: '账号密码', value: 'password' },
  { label: '手机验证码', value: 'sms' },
  { label: '邮箱验证码', value: 'email' },
  { label: '代登录', value: 'impersonate' },
  { label: '微信', value: 'wechat' },
  { label: 'QQ', value: 'qq' },
  { label: '支付宝', value: 'alipay' },
  { label: '企业微信', value: 'wecom' },
  { label: 'GitHub', value: 'github' },
]

export const LOGIN_TYPE_LABEL: Record<string, string> = {
  password: '账号密码',
  sms: '手机验证码',
  email: '邮箱验证码',
  impersonate: '代登录',
  wechat: '微信',
  qq: 'QQ',
  alipay: '支付宝',
  wecom: '企业微信',
  github: 'GitHub',
}

/** 登录日志 / 会话的 risk_flag。 */
export const RISK_FLAG_OPTIONS = [
  { label: '正常', value: 'normal' },
  { label: '暴力破解', value: 'brute_force' },
  { label: '可疑 IP', value: 'suspicious_ip' },
  { label: '设备变更', value: 'device_change' },
]

export const RISK_FLAG_LABEL: Record<string, string> = {
  normal: '正常',
  brute_force: '暴力破解',
  suspicious_ip: '可疑 IP',
  device_change: '设备变更',
}

/** 风险事件 risk_type。 */
export const RISK_TYPE_OPTIONS = [
  { label: '暴力破解', value: 'brute_force' },
  { label: '可疑 IP', value: 'suspicious_ip' },
  { label: '设备变更', value: 'device_change' },
]

export const RISK_TYPE_LABEL: Record<string, string> = {
  brute_force: '暴力破解',
  suspicious_ip: '可疑 IP',
  device_change: '设备变更',
}

/** 风险等级（critical 保留给未来的严重级）。 */
export const RISK_LEVEL_OPTIONS = [
  { label: '低', value: 'low' },
  { label: '中', value: 'medium' },
  { label: '高', value: 'high' },
  { label: '严重', value: 'critical' },
]

export const RISK_LEVEL_LABEL: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '严重',
}

/** 风险事件处置状态。 */
export const RISK_STATUS_OPTIONS = [
  { label: '待处理', value: 'pending' },
  { label: '已忽略', value: 'ignored' },
  { label: '已处置', value: 'handled' },
]

export const RISK_STATUS_LABEL: Record<string, string> = {
  pending: '待处理',
  ignored: '已忽略',
  handled: '已处置',
}

/** 会话状态：库里只有 active / revoked / expired 三种（没有 online）。 */
export const SESSION_STATUS_OPTIONS = [
  { label: '在线', value: 'active' },
  { label: '已失效', value: 'revoked' },
  { label: '已过期', value: 'expired' },
]

export const SESSION_STATUS_LABEL: Record<string, string> = {
  active: '在线',
  revoked: '已失效',
  expired: '已过期',
}

/**
 * 会话平台：真实值是终端类型（web/mobile/desktop），不是「前台/后台」。
 *
 * admin 是管理端代登录写下的会话（见 server/assembly_impersonation.go）——
 * 代登录会真的开一条用户会话，运营必须能在列表里把它与用户自己的登录区分开，
 * 否则「这个用户当前有几个登录态」会把管理员的排查行为算进去。
 */
export const SESSION_PLATFORM_OPTIONS = [
  { label: 'Web', value: 'web' },
  { label: '移动端', value: 'mobile' },
  { label: '桌面端', value: 'desktop' },
  { label: '管理端代登录', value: 'admin' },
]

export const SESSION_PLATFORM_LABEL: Record<string, string> = {
  web: 'Web',
  mobile: '移动端',
  desktop: '桌面端',
  admin: '管理端代登录',
}

/** 登录日志的 platform 与会话同源（含 admin），这里保持一份独立常量便于日后分叉。 */
export const LOGIN_PLATFORM_LABEL: Record<string, string> = {
  ...SESSION_PLATFORM_LABEL,
}

/** 黑名单类型 / 来源 / 状态。 */
export const BLACKLIST_TYPE_OPTIONS = [
  { label: 'IP', value: 'ip' },
  { label: '账号', value: 'user' },
  { label: '设备', value: 'device' },
]

export const BLACKLIST_TYPE_LABEL: Record<string, string> = {
  ip: 'IP',
  user: '账号',
  device: '设备',
}

export const BLACKLIST_SOURCE_OPTIONS = [
  { label: '人工', value: 'manual' },
  { label: '系统', value: 'system' },
  { label: '风险事件', value: 'risk_event' },
]

export const BLACKLIST_SOURCE_LABEL: Record<string, string> = {
  manual: '人工',
  system: '系统',
  risk_event: '风险事件',
}

export const BLACKLIST_STATUS_OPTIONS = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'inactive' },
]

export const BLACKLIST_STATUS_LABEL: Record<string, string> = {
  active: '启用',
  inactive: '停用',
}

/**
 * 日期区间 → start_time / end_time。
 *
 * t-date-range-picker 给的是 ['YYYY-MM-DD','YYYY-MM-DD']。后端按 time.Local 解析，
 * 且纯日期作为上界会自动补到当天 23:59:59（见 security_repo.parseFilterTime），
 * 因此这里原样透传即可；清空时同步清掉两个边界。
 */
export function applyDateRange(
  target: { start_time?: string; end_time?: string },
  value: unknown,
): void {
  const range = Array.isArray(value) ? (value as string[]) : undefined
  if (!range || range.length !== 2 || !range[0] || !range[1]) {
    target.start_time = undefined
    target.end_time = undefined
    return
  }
  target.start_time = range[0]
  target.end_time = range[1]
}

// 零值时间（Go time.Time 零值序列化成 "0001-01-01T00:00:00Z"）在 JS 里是合法日期，
// 直接 toLocaleString 会渲染成 "1/1/1 00:00:00"。可空的时间列（如会话 expired_at）
// 空值就是这么过来的，必须在这里折成「—」，否则页面上会出现公元 1 年的过期时间。
export function formatSecurityTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) return '—'
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function formatSecurityCount(value?: number) {
  return Number(value || 0).toLocaleString('zh-CN')
}

export function formatSecurityBool(value?: boolean) {
  return value ? '是' : '否'
}
