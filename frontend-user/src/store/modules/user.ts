import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  login as loginApi,
  register as registerApi,
  getUserInfo,
  verifyLoginOTP,
  type LoginParams,
  type RegisterParams,
} from '@/api/auth'

interface UserInfo {
  id?: number
  username?: string
  name?: string
  email?: string
  phone?: string
  avatar?: string
  role?: string
  tier?: string
  /** 子账号信息（P4-09）：是否子账号、归属主账号 ID/用户名、备注 */
  is_sub_account?: boolean
  owner_user_id?: number
  owner_name?: string
  remark?: string
  /** 子账号已授予的客户侧权限码；主账号为空数组 */
  permissions?: string[]
}

export const useUserStore = defineStore('user', () => {
  const token = ref<string>(localStorage.getItem('user_token') || '')
  const userInfo = ref<UserInfo>({})
  const loaded = ref(false)

  const isLoggedIn = computed(() => !!token.value)
  const displayName = computed(() => userInfo.value.name || userInfo.value.username || '用户')
  const isSubAccount = computed(() => Boolean(userInfo.value.is_sub_account))
  const permissions = computed<string[]>(() => userInfo.value.permissions || [])

  /**
   * 密码登录（doc91 §5.3）。
   *
   * 命中二次验证时不写令牌，返回值里带 need_otp/otp_token，由调用方弹 OTP 框。
   * 旧调用方忽略返回值即可（不传账号密码以外字段时后端行为与升级前一致）。
   */
  async function login(credentials: LoginParams) {
    const { data } = await loginApi(credentials)
    if (data.need_otp === true && data.otp_token) {
      return {
        needOTP: true,
        otpToken: data.otp_token,
        otpChannel: data.otp_channel || '',
        otpTargetMasked: data.otp_target_masked || '',
        otpExpireIn: data.otp_expire_in || 0,
      }
    }
    applySession(data)
    return { needOTP: false }
  }

  /** 完成登录二次验证：凭 otp_token + 验证码换正式令牌（doc91 §4.6）。 */
  async function loginVerifyOTP(otpToken: string, code: string) {
    const { data } = await verifyLoginOTP({ otp_token: otpToken, code })
    applySession(data)
  }

  /** 写入登录会话（令牌 + 用户信息）。二次验证链路复用同一套，避免漏写。 */
  function applySession(data: { token: string; user?: UserInfo }) {
    token.value = data.token
    localStorage.setItem('user_token', data.token)
    if (data.user) {
      userInfo.value = data.user
    }
    loaded.value = true
  }

  async function register(data: RegisterParams) {
    const { data: result } = await registerApi(data)
    return result
  }

  async function fetchUserInfo() {
    if (!token.value) return
    try {
      const { data } = await getUserInfo()
      userInfo.value = data
      loaded.value = true
    } catch (e) {
      console.error('Failed to fetch user info:', e)
    }
  }

  function logout() {
    token.value = ''
    userInfo.value = {}
    loaded.value = false
    localStorage.removeItem('user_token')
  }

  return {
    token,
    userInfo,
    loaded,
    isLoggedIn,
    displayName,
    isSubAccount,
    permissions,
    login,
    loginVerifyOTP,
    register,
    fetchUserInfo,
    logout,
  }
})
