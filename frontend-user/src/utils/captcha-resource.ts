// 验证码与二次验证前端共享仓库（doc91 §3.5/§8.2）。
//
// auth-config 做「单飞 + 短期缓存」：登录页、注册页、忘记密码页都会挂
// CaptchaImage，各自挂载各拉一次会在登录高峰放大成几十次无谓请求。
//
// 拿不到配置时按「不渲染图形码」处理：后端默认 captcha_enabled=false，
// 与「默认不显示 → 升级后登录页视觉不变」的目标一致，也保证配置接口故障
// 时不会把所有人挡在登录页外面。
import { reactive, readonly } from 'vue'

import { getAuthConfig, type PublicAuthConfig, type PublicScenePolicy } from '@/api/public'

/** auth-config 缓存 TTL（毫秒）。策略改动不频繁，30s 足够且能让运营改动及时生效。 */
const CONFIG_TTL_MS = 30_000

interface CaptchaConfigState {
  loaded: boolean
  loading: boolean
  /** 总闸。false 时任何场景都不渲染图形码（策略处于「预配置」状态）。 */
  enabled: boolean
  scenes: Record<string, PublicScenePolicy>
  imageProvider: string
}

const state = reactive<CaptchaConfigState>({
  loaded: false,
  loading: false,
  enabled: false,
  scenes: {},
  imageProvider: '',
})

let inflight: Promise<CaptchaConfigState> | null = null
let loadedAt = 0

/** 拉取（或复用）auth-config。并发调用共享同一个 Promise。 */
export async function loadAuthConfig(force = false): Promise<CaptchaConfigState> {
  if (!force && state.loaded && Date.now() - loadedAt < CONFIG_TTL_MS) {
    return state
  }
  if (inflight) return inflight
  state.loading = true
  inflight = getAuthConfig()
    .then((cfg: PublicAuthConfig) => {
      state.loaded = true
      state.enabled = cfg?.captcha_enabled === true
      state.scenes = cfg?.scenes || {}
      state.imageProvider = cfg?.image_provider || ''
      loadedAt = Date.now()
      return state
    })
    .catch(() => {
      // 接口失败：不缓存失败结果，按不渲染处理。
      state.loaded = false
      state.enabled = false
      state.scenes = {}
      return state
    })
    .finally(() => {
      state.loading = false
      inflight = null
    })
  return inflight
}

/** 某场景是否要求图形码（总闸关闭时恒 false）。 */
export function imageRequired(scene: string): boolean {
  if (!state.enabled) return false
  return state.scenes[scene]?.image_required === true
}

/** 某场景是否要求 OTP（注册/找回密码的验证码行显隐）。 */
export function otpRequired(scene: string): boolean {
  if (!state.enabled) return false
  return state.scenes[scene]?.otp_required === true
}

/** 某场景 OTP 默认通道。 */
export function otpChannel(scene: string): 'email' | 'sms' {
  return state.scenes[scene]?.otp_channel === 'sms' ? 'sms' : 'email'
}

/** 只读状态（组件里 watch 用；写入仅通过 loadAuthConfig）。 */
export const captchaConfig = readonly(state)

/** 测试/登出后强制失效。 */
export function resetAuthConfig() {
  state.loaded = false
  state.enabled = false
  state.scenes = {}
  loadedAt = 0
}
