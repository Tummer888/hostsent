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
}

export const useUserStore = defineStore('user', () => {
  const token = ref<string>(localStorage.getItem('user_token') || '')
  const userInfo = ref<UserInfo>({})
  const loaded = ref(false)

  const isLoggedIn = computed(() => !!token.value)
  const displayName = computed(() => userInfo.value.name || userInfo.value.username || '用户')

  async function login(credentials: { username: string; password: string }) {
    const { data } = await loginApi(credentials)
    token.value = data.token
    localStorage.setItem('user_token', data.token)
    if (data.user) {
      userInfo.value = data.user
    }
    loaded.value = true
  }

  async function register(data: { username: string; password: string; email: string }) {
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
    login,
    register,
    fetchUserInfo,
    logout,
  }
})
