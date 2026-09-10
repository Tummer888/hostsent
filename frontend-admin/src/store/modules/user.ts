import { defineStore } from 'pinia'

import { changePassword as changePasswordApi, getCurrentUser, login as loginApi } from '@/api/auth'
import { useMenuStore } from '@/store/modules/menu'
import type { UserInfo } from '@/types/interface'

const initUserInfo: UserInfo = {
  id: 0,
  name: '',
  username: '',
  role: '',
  roles: [],
  email: '',
  phone: '',
  status: '',
}

const REMEMBER_KEY = 'hostsent_admin_remember'
const CREDENTIAL_KEY = 'hostsent_admin_credentials'

function safeSet(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    /* ignore */
  }
}

function safeGet(key: string): string {
  try {
    return localStorage.getItem(key) || ''
  } catch {
    return ''
  }
}

function safeRemove(key: string) {
  try {
    localStorage.removeItem(key)
  } catch {
    /* ignore */
  }
}

// 读取记住的凭证字段，解析失败返回空串，避免 localStorage 损坏时初始化崩溃。
function readSavedCredential(field: 'username' | 'password'): string {
  try {
    const raw = safeGet(CREDENTIAL_KEY)
    if (!raw) return ''
    const parsed = JSON.parse(raw)
    return (parsed?.[field] as string) || ''
  } catch {
    return ''
  }
}

export const useUserStore = defineStore('user', {
  state: () => ({
    token: '',
    userInfo: { ...initUserInfo },
    // 权限码集合：超管为 ['*']，后端 /auth/me 会刷新（不长期信任本地缓存）
    permissions: [] as string[],
    // 首次登录/重置密码后需强制改密
    mustChangePassword: false,
    remember: safeGet(REMEMBER_KEY) === '1',
    savedUsername: readSavedCredential('username'),
    savedPassword: readSavedCredential('password'),
  }),
  getters: {
    isAdmin: (state) => {
      const roleSet = new Set([state.userInfo.role, ...(state.userInfo.roles || [])].filter(Boolean))
      return roleSet.has('admin') || roleSet.has('super_admin')
    },
    roles: (state) => state.userInfo.roles,
    isSuperAdmin: (state) => {
      const roleSet = new Set([state.userInfo.role, ...(state.userInfo.roles || [])].filter(Boolean))
      return state.permissions.includes('*') || roleSet.has('super_admin')
    },
  },
  actions: {
    // 是否持有任一权限码；超管通配 "*" 恒通过。
    hasPermission(...codes: string[]): boolean {
      if (!codes.length) return true
      if (this.permissions.includes('*')) return true
      return codes.some((code) => this.permissions.includes(code))
    },
    persistCredentials(username: string, password: string, remember: boolean) {
      this.remember = remember
      if (remember) {
        safeSet(REMEMBER_KEY, '1')
        safeSet(CREDENTIAL_KEY, JSON.stringify({ username, password }))
      } else {
        safeRemove(REMEMBER_KEY)
        safeRemove(CREDENTIAL_KEY)
      }
    },
    async login(payload: Record<string, unknown>) {
      const { username, password, captchaKey, captchaCode, remember } = payload as {
        username: string
        password: string
        captchaKey?: string
        captchaCode?: string
        remember?: boolean
      }
      const res = await loginApi({
        username,
        password,
        captcha_key: captchaKey,
        captcha_code: captchaCode,
      })
      this.token = res.token
      this.permissions = res.permissions || []
      this.mustChangePassword = res.must_change_password === true
      this.userInfo = {
        id: res.user_info.id,
        name: res.user_info.username,
        username: res.user_info.username,
        role: res.user_info.role,
        roles: res.user_info.roles?.length ? res.user_info.roles : [res.user_info.role],
        email: res.user_info.email,
        phone: res.user_info.phone || '',
        status: res.user_info.status,
        avatar: res.user_info.avatar,
        department: res.user_info.department,
        position: res.user_info.position,
      }
      if (!this.isAdmin) {
        await this.logout()
        throw new Error('仅管理员账号可登录管理后台')
      }
      this.persistCredentials(username, password, remember === true)
    },
    async getUserInfo() {
      const res = await getCurrentUser()
      this.userInfo = {
        id: res.id,
        name: res.username,
        username: res.username,
        role: res.role,
        roles: res.roles?.length ? res.roles : [res.role],
        email: res.email,
        phone: res.phone || '',
        status: res.status,
        avatar: res.avatar,
        department: res.department,
        position: res.position,
      }
      // 刷新时从 /auth/me 恢复权限，避免长期信任 localStorage
      if (res.permissions) {
        this.permissions = res.permissions
      }
      if (typeof res.must_change_password === 'boolean') {
        this.mustChangePassword = res.must_change_password
      }
      if (!this.isAdmin) {
        await this.logout()
        throw new Error('当前账号不是管理员')
      }
      return this.userInfo
    },
    async changePassword(oldPassword: string, newPassword: string) {
      await changePasswordApi({ old_password: oldPassword, new_password: newPassword })
      this.mustChangePassword = false
    },
    async logout() {
      this.token = ''
      this.userInfo = { ...initUserInfo }
      this.permissions = []
      this.mustChangePassword = false
      // 登出时清空动态菜单，避免下次登录残留旧菜单
      useMenuStore().reset()
    },
  },
  persist: {
    key: 'hostsent_admin_user',
    pick: ['token'],
  },
})
