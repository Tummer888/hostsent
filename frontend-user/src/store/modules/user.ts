import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as loginApi, register as registerApi, getUserInfo } from '@/api/auth'

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

  async function login(credentials: { username: string; password: string }) {
    const { data } = await loginApi(credentials)
    token.value = data.token
    localStorage.setItem('user_token', data.token)
    if (data.user) {
      userInfo.value = data.user
    }
    loaded.value = true
  }

  async function register(data: { username: string; password: string; email: string; phone?: string; invite_code?: string }) {
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
    register,
    fetchUserInfo,
    logout,
  }
})
