/**
 * 设备指纹：登录时随 X-Device-Fingerprint 上报，服务端用它判定「登录设备是否变更」、
 * 以及支持按设备拉黑。没有它，风控的设备变更规则与设备维度黑名单在真实环境里
 * 永远不会触发（后端只是因为请求头缺失而静默跳过）。
 *
 * 口径：**本机随机 ID 才是主体，环境特征只是辅助**。
 * 不追求「同一台机器的不同浏览器算出同一个值」——那种跨浏览器指纹既不稳定
 * （浏览器升级、插件变化都会改变结果）又涉及更强的用户追踪，对一个登录风控来说
 * 是负收益。这里要回答的是「还是这台浏览器的这个登录环境吗」：
 *   - 换浏览器 → 新指纹（合理：对服务端来说确实是新设备）
 *   - 清缓存 → 新指纹（合理：清缓存本身就是要抹掉身份）
 *   - 换网络 / 升级系统小版本 → 指纹不变（这是我们想要的效果，避免误报）
 */

const STORAGE_KEY = 'hostsent_device_id'

/** 环境特征参与哈希的部分：只取变化频率低、且不识别到个人的维度。 */
function environmentSignals(): string {
  const nav = navigator
  const screenInfo = typeof window !== 'undefined' ? window.screen : undefined
  return [
    nav.userAgent ?? '',
    nav.language ?? '',
    String(nav.hardwareConcurrency ?? ''),
    // 时区是「这台设备常在哪用」的弱信号，且跟人走、不跟账号走。
    Intl.DateTimeFormat().resolvedOptions().timeZone ?? '',
    screenInfo ? `${screenInfo.width}x${screenInfo.height}x${screenInfo.colorDepth}` : '',
  ].join('|')
}

/** 生成一个随机 ID（crypto 不可用时退化为时间戳 + 随机数，够用即可）。 */
function randomID(): string {
  const cryptoObj = typeof window !== 'undefined' ? window.crypto : undefined
  if (cryptoObj?.randomUUID) {
    return cryptoObj.randomUUID()
  }
  if (cryptoObj?.getRandomValues) {
    const bytes = new Uint8Array(16)
    cryptoObj.getRandomValues(bytes)
    return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`
}

/** 稳定 ID：首次访问生成后落 localStorage，之后一直复用。 */
function stableID(): string {
  try {
    const existing = window.localStorage.getItem(STORAGE_KEY)
    if (existing) {
      return existing
    }
    const created = randomID()
    window.localStorage.setItem(STORAGE_KEY, created)
    return created
  } catch {
    // 隐私模式 / 禁用存储：退化为会话级 ID（刷新会变，但至少本次报得出设备）。
    return randomID()
  }
}

/** djb2 哈希：把「稳定 ID + 环境特征」压成一个短字符串，避免上报长串。 */
function hash(input: string): string {
  let h = 5381
  for (let i = 0; i < input.length; i += 1) {
    h = ((h << 5) + h + input.charCodeAt(i)) >>> 0
  }
  return h.toString(36)
}

let cached: string | null = null

/**
 * 取当前设备指纹。同一页面生命周期内结果恒定（缓存），避免每次请求重新计算
 * 造成同一会话里指纹漂移——那会让服务端把一次登录判成设备变更。
 */
export function getDeviceFingerprint(): string {
  if (cached) {
    return cached
  }
  if (typeof window === 'undefined') {
    return '' // SSR / 非浏览器环境：不报，服务端按「无指纹」跳过设备规则。
  }
  const id = stableID()
  const env = environmentSignals()
  cached = `web-${hash(`${id}::${env}`)}`
  return cached
}
