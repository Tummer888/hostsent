import { defineStore } from 'pinia'

import { getCurrentUser, login as loginApi } from '@/api/auth'
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

export const useUserStore = defineStore('user', {
  state: () => ({
    token: '',
    userInfo: { ...initUserInfo },
    remember: safeGet(REMEMBER_KEY) === '1',
    savedUsername: safeGet(CREDENTIAL_KEY) ? JSON.parse(safeGet(CREDENTIAL_KEY)).username || '' : '',
    savedPassword: safeGet(CREDENTIAL_KEY) ? JSON.parse(safeGet(CREDENTIAL_KEY)).password || '' : '',
  }),
  getters: {
    isAdmin: (state) => {
      const roleSet = new Set([state.userInfo.role, ...(state.userInfo.roles || [])].filter(Boolean))
      return roleSet.has('admin') || roleSet.has('super_admin')
    },
    roles: (state) => state.userInfo.roles,
  },
  actions: {
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
      this.userInfo = {
        id: res.user_info.id,
        name: res.user_info.username,
        username: res.user_info.username,
        role: res.user_info.role,
        roles: res.user_info.roles?.length ? res.user_info.roles : [res.user_info.role],
        email: res.user_info.email,
        phone: res.user_info.phone,
        status: res.user_info.status,
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
        phone: res.phone,
        status: res.status,
      }
      if (!this.isAdmin) {
        await this.logout()
        throw new Error('当前账号不是管理员')
      }
      return this.userInfo
    },
    async logout() {
      this.token = ''
      this.userInfo = { ...initUserInfo }
      // 登出时清空动态菜单，避免下次登录残留旧菜单
      useMenuStore().reset()
    },
  },
  persist: {
    key: 'hostsent_admin_user',
    pick: ['token'],
  },
})
