import { defineStore } from 'pinia'

import { applyRadius, applyThemeColor, DEFAULT_THEME_COLOR } from '@/utils/theme'

// 色彩方案：浅色 / 深色 / 跟随系统
export type ColorScheme = 'light' | 'dark' | 'auto'

interface SettingsState {
  themeColor: string
  colorScheme: ColorScheme
  radius: number // 圆角档位百分比，100 = 默认
  colorWeak: boolean
  systemPrefersDark: boolean // 运行时状态，不持久化
}

const STORAGE_KEY = 'hostsent_user_settings'

function readSavedSettings(): Partial<SettingsState> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Partial<SettingsState>
    if (!parsed || typeof parsed !== 'object') return {}
    return parsed
  } catch {
    return {}
  }
}

// 系统深浅色变化监听（模块级，避免重复注册）
function onSystemSchemeChange(e: MediaQueryListEvent) {
  const s = useSettingsStore()
  s.systemPrefersDark = e.matches
  s.applyColorScheme()
}

export const useSettingsStore = defineStore('settings', {
  state: (): SettingsState => ({
    themeColor: DEFAULT_THEME_COLOR,
    colorScheme: 'light',
    radius: 100,
    colorWeak: false,
    systemPrefersDark: false,
    ...readSavedSettings(),
  }),
  getters: {
    isDark: (state) =>
      state.colorScheme === 'dark' || (state.colorScheme === 'auto' && state.systemPrefersDark),
  },
  actions: {
    // 应用启动时调用：注册系统主题监听并应用全部设置
    init() {
      if (typeof window !== 'undefined' && window.matchMedia) {
        const mq = window.matchMedia('(prefers-color-scheme: dark)')
        this.systemPrefersDark = mq.matches
        mq.removeEventListener('change', onSystemSchemeChange)
        mq.addEventListener('change', onSystemSchemeChange)
      }
      this.applyAll()
    },
    update(partial: Partial<SettingsState>) {
      Object.assign(this, partial)
      this.applyAll()
      try {
        localStorage.setItem(
          STORAGE_KEY,
          JSON.stringify({
            themeColor: this.themeColor,
            colorScheme: this.colorScheme,
            radius: this.radius,
            colorWeak: this.colorWeak,
          }),
        )
      } catch {
        /* ignore */
      }
    },
    toggleDark() {
      this.colorScheme = this.isDark ? 'light' : 'dark'
      this.applyColorScheme()
      try {
        localStorage.setItem(
          STORAGE_KEY,
          JSON.stringify({
            themeColor: this.themeColor,
            colorScheme: this.colorScheme,
            radius: this.radius,
            colorWeak: this.colorWeak,
          }),
        )
      } catch {
        /* ignore */
      }
    },
    applyColorScheme() {
      const root = document.documentElement
      root.classList.toggle('dark', this.isDark)
      // TDesign 组件按官方约定读取 theme-mode 属性切换深色 token
      if (this.isDark) root.setAttribute('theme-mode', 'dark')
      else root.removeAttribute('theme-mode')
    },
    applyAll() {
      applyThemeColor(this.themeColor)
      applyRadius(this.radius)
      this.applyColorScheme()
      document.documentElement.classList.toggle('color-weak', this.colorWeak)
    },
    reset() {
      this.update({
        themeColor: DEFAULT_THEME_COLOR,
        colorScheme: 'light',
        radius: 100,
        colorWeak: false,
      })
    },
  },
})