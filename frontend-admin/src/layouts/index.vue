<template>
  <div class="admin-layout">
    <!-- 移动端遮罩 -->
    <transition name="overlay-fade">
      <div
        v-if="mobileSidebarOpen"
        class="sidebar-overlay"
        @click="mobileSidebarOpen = false"
      ></div>
    </transition>

    <!-- 侧边栏区域 -->
    <div
      class="sidebar-wrapper"
      :class="{
        'is-collapsed': sidebarCollapsed,
        'is-mobile-open': mobileSidebarOpen,
      }"
    >
      <!-- 1. 分组/一级菜单栏 (使用动态菜单的顶级节点) -->
      <aside class="group-sidebar">
        <div class="group-logo">H</div>
        <ul class="group-list">
          <li
            v-for="group in sidebarItems"
            :key="group.path || group.name"
            class="group-item"
            :class="{ 'is-active': activeGroup === (group.path || group.name) }"
            @click="onGroupClick(group)"
          >
            <component :is="group.icon" class="group-icon" v-if="group.icon" />
            <span class="group-label" :class="{ 'label-hidden': sidebarCollapsed }">{{ group.name }}</span>
          </li>
        </ul>
      </aside>

      <!-- 2. 主菜单/二三级栏 -->
      <aside class="menu-sidebar" :class="{ 'is-closed': sidebarCollapsed }">
        <div class="menu-header">
          <span class="menu-header__title">{{ currentGroup?.name || '菜单' }}</span>
          <button
            class="menu-collapse-btn"
            :title="sidebarCollapsed ? '展开菜单' : '收缩菜单'"
            @click="toggleSidebar"
          >
            <ChevronRightIcon v-if="sidebarCollapsed" size="16" />
            <ChevronLeftIcon v-else size="16" />
          </button>
          <!-- 移动端专用：关闭抽屉菜单 -->
          <button class="menu-close-btn" title="关闭菜单" aria-label="关闭菜单" @click="mobileSidebarOpen = false">
            <CloseIcon size="16" />
          </button>
        </div>
        <ul class="menu-list">
          <template v-for="menu in currentMenuList" :key="menu.path || menu.name">
            <!-- 有子菜单 -->
            <li
              v-if="menu.children && menu.children.length"
              class="menu-item has-children"
              :class="{ 'is-active': isMenuActive(menu) }"
            >
              <div class="menu-item-inner" @click="toggleExpand(menu)">
                <span class="menu-icon"><component :is="menu.icon" v-if="menu.icon" /></span>
                <span class="menu-title">{{ menu.name }}</span>
                <ChevronRightIcon class="menu-arrow" :class="{ 'is-open': isExpanded(menu) }" />
              </div>
              <!-- 收缩态二级悬浮弹出 -->
              <div v-if="sidebarCollapsed" class="collapsed-popup" @click.stop>
                <div class="collapsed-popup__header">{{ menu.name }}</div>
                <ul class="collapsed-popup__list">
                  <li
                    v-for="sub in (menu.children || [])"
                    :key="sub.path || sub.name"
                    class="collapsed-popup__item"
                    :class="{ 'is-active': route.path === sub.path }"
                  >
                    <div class="collapsed-popup__link" @click="navigateTo(sub.path)">
                      <span class="collapsed-popup__dot"></span>
                      <span>{{ sub.name }}</span>
                    </div>
                  </li>
                </ul>
              </div>
              <!-- 三级菜单（展开态） -->
              <transition name="slide">
                <ul v-if="!sidebarCollapsed && isExpanded(menu)" class="submenu-list">
                  <li
                    v-for="submenu in menu.children"
                    :key="submenu.path || submenu.name"
                    class="submenu-item"
                    :class="{ 'is-active': route.path === submenu.path }"
                    @click="navigateTo(submenu.path)"
                  >
                    <span class="submenu-dot" aria-hidden="true"></span>
                    <span class="submenu-title">{{ submenu.name }}</span>
                  </li>
                </ul>
              </transition>
            </li>
            <!-- 无子菜单 -->
            <li
              v-else
              class="menu-item"
              :class="{ 'is-active': route.path === menu.path }"
              @click="navigateTo(menu.path)"
            >
              <div class="menu-item-inner">
                <span class="menu-icon"><component :is="menu.icon" v-if="menu.icon" /></span>
                <span class="menu-title">{{ menu.name }}</span>
              </div>
            </li>
          </template>
        </ul>
      </aside>
    </div>

    <!-- 3. 主内容区 -->
    <t-layout direction="vertical" class="main-layout">
      <t-header class="top-header">
        <div class="top-header__left">
          <!-- 移动端菜单按钮 -->
          <button class="mobile-menu-btn" aria-label="菜单" @click="mobileSidebarOpen = true">
            <MenuIcon size="20" />
          </button>
        </div>
        <div class="top-header__right">
          <!-- 导航搜索框 -->
          <div class="navbar-search">
            <SearchIcon size="16" class="navbar-search__icon" />
            <input
              v-model="navSearch"
              class="navbar-search__input"
              type="text"
              placeholder="搜索页面..."
              @focus="navSearchOpen = true"
              @blur="navSearchOpen = false"
              @keyup.enter="gotoFirstResult"
            />
            <div v-if="navSearchOpen && navSearch.trim()" class="navbar-search__dropdown">
              <div
                v-for="r in navResults"
                :key="r.path"
                class="navbar-search__item"
                @mousedown.prevent="switchTo(r.path)"
              >
                <span class="navbar-search__item-icon"><component :is="r.icon" v-if="r.icon" /></span>
                <span>{{ r.title }}</span>
              </div>
              <div v-if="!navResults.length" class="navbar-search__empty">无匹配结果</div>
            </div>
          </div>

          <!-- 主题设置 -->
          <t-tooltip content="主题设置" placement="bottom">
            <t-button variant="text" shape="square" aria-label="主题设置" @click="settingsVisible = true">
              <template #icon><SettingIcon /></template>
            </t-button>
          </t-tooltip>

          <!-- 全屏 -->
          <t-tooltip content="全屏" placement="bottom">
            <t-button variant="text" shape="square" aria-label="全屏" @click="toggleFullscreen">
              <template #icon><FullscreenIcon /></template>
            </t-button>
          </t-tooltip>

          <!-- 主题切换 -->
          <t-tooltip :content="isDark ? '切换浅色' : '切换深色'" placement="bottom">
            <t-button variant="text" shape="square" aria-label="主题切换" @click="toggleTheme">
              <template #icon><MoonIcon v-if="!isDark" /><SunnyIcon v-else /></template>
            </t-button>
          </t-tooltip>

          <!-- 语言切换 -->
          <t-tooltip content="语言切换" placement="bottom">
            <t-dropdown trigger="click" @click="onLangChange">
              <t-button variant="text" shape="square" aria-label="语言切换">
                <template #icon><TranslateIcon /></template>
              </t-button>
              <template #dropdown>
                <t-dropdown-menu>
                  <t-dropdown-item value="zh-CN">中文</t-dropdown-item>
                  <t-dropdown-item value="en-US">English</t-dropdown-item>
                </t-dropdown-menu>
              </template>
            </t-dropdown>
          </t-tooltip>

          <!-- 通知 -->
          <t-tooltip content="通知" placement="bottom">
            <t-badge :count="3" size="small" :offset="[-2, 2]">
              <t-button variant="text" shape="square" aria-label="通知">
                <template #icon>
                  <NotificationIcon />
                </template>
              </t-button>
            </t-badge>
          </t-tooltip>

          <!-- 用户信息 -->
          <t-dropdown trigger="click" @click="onUserMenuClick">
            <div class="user-chip" tabindex="0" role="button" aria-label="用户菜单">
              <t-avatar size="30" class="user-chip__avatar">
                {{ userInitial }}
              </t-avatar>
              <span class="user-chip__name">{{ userName }}</span>
              <ChevronDownIcon size="14" class="user-chip__caret" />
            </div>
            <template #dropdown>
              <t-dropdown-menu>
                <t-dropdown-item value="profile">
                  <template #icon><UserIcon /></template>
                  个人资料
                </t-dropdown-item>
                <t-dropdown-item value="settings">
                  <template #icon><SettingIcon /></template>
                  账号设置
                </t-dropdown-item>
                <t-dropdown-item divider />
                <t-dropdown-item value="logout">
                  <template #icon><PoweroffIcon /></template>
                  退出登录
                </t-dropdown-item>
              </t-dropdown-menu>
            </template>
          </t-dropdown>
        </div>
      </t-header>

      <!-- 已打开页面标签栏（可在主题设置中关闭） -->
      <div v-if="settings.showTabsBar" class="tabs-bar">
        <div class="tabs-bar__scroll">
          <div
            v-for="tab in openedTabs"
            :key="tab.path"
            class="tabs-bar__tab"
            :class="{ 'is-active': isTabActive(tab) }"
            @click="switchTab(tab)"
          >
            <span class="tabs-bar__title">{{ tab.title }}</span>
            <button
              class="tabs-bar__pin"
              :class="{ 'is-pinned': tab.pinned }"
              :title="tab.pinned ? '取消固定' : '固定标签'"
              @click.stop="togglePin(tab)"
            >
              <PinIcon size="12" />
            </button>
            <button v-if="!tab.pinned" class="tabs-bar__close" title="关闭" @click.stop="closeTab(tab)">
              <CloseIcon size="12" />
            </button>
          </div>
        </div>
        <t-dropdown trigger="click" @click="onTabsMore">
          <t-button variant="text" shape="square" class="tabs-bar__more" aria-label="标签操作">
            <template #icon><MoreIcon /></template>
          </t-button>
          <template #dropdown>
            <t-dropdown-menu>
              <t-dropdown-item value="pin-current">固定当前页</t-dropdown-item>
              <t-dropdown-item value="close-others">关闭其它</t-dropdown-item>
              <t-dropdown-item value="close-all">关闭全部</t-dropdown-item>
            </t-dropdown-menu>
          </template>
        </t-dropdown>
      </div>
      <t-content class="content-area">
        <div class="content-inner">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </div>
      </t-content>
    </t-layout>

    <!-- 主题设置抽屉 -->
    <SettingsPanel v-model:visible="settingsVisible" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import {
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CloseIcon,
  FullscreenIcon,
  MenuIcon,
  MoonIcon,
  MoreIcon,
  NotificationIcon,
  PinIcon,
  PoweroffIcon,
  SearchIcon,
  SettingIcon,
  SunnyIcon,
  TranslateIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { navMenu } from '@/permission'
import { useMenuStore } from '@/store/modules/menu'
import { useSettingsStore } from '@/store/modules/settings'
import { useUserStore } from '@/store/modules/user'
import type { FlatMenu } from '@/store/modules/menu'
import SettingsPanel from '@/components/settings-panel/index.vue'

defineOptions({ name: 'AdminLayout' })

const router = useRouter()
const route = useRoute()
const menuStore = useMenuStore()
const userStore = useUserStore()
const settings = useSettingsStore()

// 主题设置抽屉
const settingsVisible = ref(false)

const expandedKeys = ref<Set<string>>(new Set())
const menuLoading = ref(false)
const useFallbackMenu = ref(false)

const SIDEBAR_STORAGE_KEY = 'hostsent_admin_sidebar_collapsed'

function getInitialCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_STORAGE_KEY) === 'true'
  } catch {
    return false
  }
}

const sidebarCollapsed = ref(getInitialCollapsed())

const mobileSidebarOpen = ref(false)

// ---- 导航搜索 ----
const navSearch = ref('')
const navSearchOpen = ref(false)

// ---- 主题（浅/深）：由 settings store 驱动 ----
const isDark = computed(() => settings.isDark)

// ---- 已打开页面标签 ----
const TABS_KEY = 'hostsent_admin_tabs'
interface OpenedTab {
  path: string
  name: string
  title: string
  pinned: boolean
}
const openedTabs = ref<OpenedTab[]>([])

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  try {
    localStorage.setItem(SIDEBAR_STORAGE_KEY, String(sidebarCollapsed.value))
  } catch {
    /* ignore storage errors */
  }
}

function handleResize() {
  if (window.innerWidth > 768) {
    mobileSidebarOpen.value = false
  }
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})

const userName = computed(() => {
  const { name, username } = userStore.userInfo
  return name || username || '管理员'
})

const userInitial = computed(() => {
  const base = userStore.userInfo.name || userStore.userInfo.username || 'A'
  return base.slice(0, 1).toUpperCase()
})

function resolveMenuIcon(icon: any): any {
  if (!icon) return undefined

  if (icon && typeof icon === 'object' && '__v_isRef' in icon) {
    const inner = icon.value
    if (inner && inner.render && typeof inner.render === 'function') {
      return { render: inner.render }
    }
    return inner || icon
  }

  if (icon && typeof icon === 'object' && icon.render && typeof icon.render === 'function') {
    return icon
  }

  return icon
}

const fallbackMenus: FlatMenu[] = navMenu.map((item: any) => ({
  id: 0,
  parentId: 0,
  name: item.title,
  path: item.path,
  icon: resolveMenuIcon(item.icon),
  children: (item.children || []).map((child: any) => ({
    id: 0,
    parentId: 0,
    name: child.title,
    path: child.path,
    icon: resolveMenuIcon(child.icon),
    children: (child.children || []).map((gc: any) => ({
      id: 0,
      parentId: 0,
      name: gc.title,
      path: gc.path,
      icon: resolveMenuIcon(gc.icon),
      children: [],
    })),
  })),
}))

const sidebarItems = computed<FlatMenu[]>(() => {
  if (menuStore.hasMenu && !useFallbackMenu.value) {
    return menuStore.sidebarMenus
  }
  return fallbackMenus
})

onMounted(async () => {
  menuLoading.value = true
  try {
    if (!menuStore.loaded) {
      await menuStore.loadMenus('admin')
      useFallbackMenu.value = false
    }
  } catch (e) {
    console.warn('Failed to load menus from backend, using fallback:', e)
    useFallbackMenu.value = true
  } finally {
    menuLoading.value = false
  }
  initByRoute()
  initTabs()
  settings.init()
})

const activeGroup = ref<string>('')

const currentGroup = computed(() => {
  return sidebarItems.value.find((item) => (item.path || item.name) === activeGroup.value)
})

const currentMenuList = computed(() => currentGroup.value?.children || [])

function isMenuActive(menu: FlatMenu) {
  if (menu.path === route.path) return true
  return menu.children?.some((child) => child.path === route.path) || false
}

function isExpanded(menu: FlatMenu) {
  return expandedKeys.value.has(menu.path || menu.name)
}

function toggleExpand(menu: FlatMenu) {
  const key = menu.path || menu.name
  // 手风琴模式：同时只展开一项
  if (settings.menuAccordion) {
    expandedKeys.value = isExpanded(menu) ? new Set() : new Set([key])
    return
  }
  const next = new Set(expandedKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedKeys.value = next
}

function navigateTo(path?: string) {
  if (!path) return
  if (window.innerWidth <= 768) {
    mobileSidebarOpen.value = false
  }
  router.push(path)
}

function onGroupClick(group: FlatMenu) {
  activeGroup.value = group.path || group.name
  const first = group.children?.[0]
  if (!first) return
  if (first.children?.length) {
    expandedKeys.value = new Set([first.path || first.name])
    // 移动端始终仅切换；桌面端按「点击一级菜单」设置决定是否跳转
    if (window.innerWidth <= 768 || settings.firstMenuClick === 'switch') return
    navigateTo(first.children[0].path)
    return
  }
  // 移动端 / 仅切换模式：等待用户点击具体菜单项再跳转
  if (window.innerWidth <= 768 || settings.firstMenuClick === 'switch') return
  navigateTo(first.path)
}

function initByRoute() {
  for (const group of sidebarItems.value) {
    const matched = group.children?.find((menu) => menu.path === route.path || menu.children?.some((child) => child.path === route.path))
    if (!matched) continue
    activeGroup.value = group.path || group.name
    if (matched.children?.some((child) => child.path === route.path)) {
      expandedKeys.value = new Set([matched.path || matched.name])
    }
    return
  }

  if (!activeGroup.value && sidebarItems.value.length) {
    activeGroup.value = sidebarItems.value[0].path || sidebarItems.value[0].name
  }
}

// ---- 导航搜索：在菜单树里按标题匹配，跳转到对应页面 ----
function flattenMenus(menus: FlatMenu[]): Array<{ path: string; title: string; icon?: any }> {
  const out: Array<{ path: string; title: string; icon?: any }> = []
  for (const m of menus) {
    if (m.path) out.push({ path: m.path, title: m.name, icon: m.icon })
    if (m.children?.length) out.push(...flattenMenus(m.children))
  }
  return out
}

const navResults = computed(() => {
  const q = navSearch.value.trim().toLowerCase()
  if (!q) return []
  return flattenMenus(sidebarItems.value)
    .filter((r) => r.title.toLowerCase().includes(q))
    .slice(0, 10)
})

function switchTo(path?: string) {
  if (!path) return
  navSearch.value = ''
  navSearchOpen.value = false
  router.push(path)
}

function gotoFirstResult() {
  switchTo(navResults.value[0]?.path)
}

// ---- 全屏 ----
function toggleFullscreen() {
  const el = document.documentElement
  if (!document.fullscreenElement) {
    el.requestFullscreen?.().catch(() => {})
  } else {
    document.exitFullscreen?.()
  }
}

// ---- 主题快捷切换（浅/深，由 settings store 持久化） ----
function toggleTheme() {
  settings.toggleDark()
}

// ---- 语言切换（暂无 i18n，占位提示） ----
function onLangChange(value: string | number | Record<string, unknown> | undefined) {
  const key = typeof value === 'object' && value !== null ? String(value.value ?? '') : String(value ?? '')
  if (key === 'en-US') {
    MessagePlugin.info('English 界面开发中，当前暂支持中文')
    return
  }
  MessagePlugin.success('已切换为中文')
}

// ---- 已打开页面标签 ----
const homeTab: OpenedTab = { path: '/dashboard/base', name: 'DashboardBase', title: '概览', pinned: true }

function persistTabs() {
  if (!settings.rememberTabs) return
  try {
    localStorage.setItem(TABS_KEY, JSON.stringify(openedTabs.value))
  } catch {
    /* ignore */
  }
}

function initTabs() {
  let loaded: OpenedTab[] = []
  if (settings.rememberTabs) {
    try {
      loaded = JSON.parse(localStorage.getItem(TABS_KEY) || '[]')
    } catch {
      loaded = []
    }
  }
  const seen = new Set<string>()
  const rest = loaded.filter((t) => t.path && t.path !== homeTab.path && !seen.has(t.path))
  for (const t of loaded) seen.add(t.path)
  openedTabs.value = [homeTab, ...rest]
  if (!openedTabs.value.some((t) => t.path === route.path)) ensureTab(route.path)
  persistTabs()
}

function titleForPath(path: string, fallback: string): string {
  for (const item of flattenMenus(sidebarItems.value)) {
    if (item.path === path) return item.title
  }
  return fallback
}

function ensureTab(path?: string) {
  if (!path || path === '/login') return
  if (openedTabs.value.some((t) => t.path === path)) return
  const title = titleForPath(path, (route.meta.title as string) || '页面')
  openedTabs.value.push({ path, name: String(route.name || ''), title, pinned: false })
  persistTabs()
}

function isTabActive(tab: OpenedTab) {
  return route.path === tab.path
}

function switchTab(tab: OpenedTab) {
  if (route.path !== tab.path) router.push(tab.path)
}

function closeTab(tab: OpenedTab) {
  if (tab.pinned) return
  const idx = openedTabs.value.findIndex((t) => t.path === tab.path)
  if (idx === -1) return
  const wasActive = route.path === tab.path
  openedTabs.value.splice(idx, 1)
  if (!openedTabs.value.length) openedTabs.value = [{ ...homeTab }]
  persistTabs()
  if (wasActive) {
    const next = openedTabs.value[Math.min(idx, openedTabs.value.length - 1)]
    router.push(next.path)
  }
}

function togglePin(tab: OpenedTab) {
  tab.pinned = !tab.pinned
  if (tab.pinned) {
    // 固定的标签排到最前（紧跟既有固定标签之后）
    const idx = openedTabs.value.findIndex((t) => t.path === tab.path)
    if (idx > -1) {
      const [moved] = openedTabs.value.splice(idx, 1)
      let insertAt = 0
      while (insertAt < openedTabs.value.length && openedTabs.value[insertAt].pinned) insertAt += 1
      openedTabs.value.splice(insertAt, 0, moved)
    }
  }
  persistTabs()
}

function onTabsMore(value: string | number | Record<string, unknown> | undefined) {
  const key = typeof value === 'object' && value !== null ? String(value.value ?? '') : String(value ?? '')
  if (key === 'pin-current') {
    const cur = openedTabs.value.find((t) => t.path === route.path)
    if (cur) {
      cur.pinned = true
      persistTabs()
    }
    return
  }
  if (key === 'close-others') {
    openedTabs.value = openedTabs.value.filter((t) => t.pinned || t.path === route.path)
    persistTabs()
    return
  }
  if (key === 'close-all') {
    const keep = openedTabs.value.filter((t) => t.path === homeTab.path)
    openedTabs.value = keep.length ? [keep[0]] : [{ ...homeTab }]
    persistTabs()
    router.push(openedTabs.value[0].path)
  }
}

function onUserMenuClick(value: string | number | Record<string, unknown> | undefined) {
  const key = typeof value === 'object' && value !== null ? String(value.value ?? '') : String(value ?? '')
  if (key === 'logout') {
    userStore.logout()
    router.push('/login')
    return
  }
  if (key === 'profile') {
    MessagePlugin.info('个人资料开发中')
    return
  }
  if (key === 'settings') {
    MessagePlugin.info('账号设置开发中')
  }
}

watch(
  () => route.path,
  (path) => {
    initByRoute()
    ensureTab(path)
  },
  { immediate: true },
)
</script>

<style scoped>
.admin-layout {
  display: flex;
  min-height: 100vh;
  background: #f8fafc;
}

.sidebar-wrapper {
  display: flex;
  flex-shrink: 0;
  position: sticky;
  top: 0;
  z-index: 10;
  height: 100vh;
  transition: width 0.28s cubic-bezier(0.4, 0, 0.2, 1);
  width: 320px;
}

.sidebar-wrapper.is-collapsed {
  width: 148px;
}

.group-sidebar {
  width: 88px;
  background: #ffffff;
  border-right: 1px solid #e5e7eb;
  color: #111827;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0 8px;
}

.group-logo {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: var(--td-brand-color-1);
  color: var(--color-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 17px;
  margin-bottom: 10px;
}

.group-list {
  list-style: none;
  margin: 0;
  padding: 0 8px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
}

.group-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  min-height: 54px;
  border-radius: 12px;
  color: #111827;
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.group-item:hover,
.group-item.is-active {
  color: #111827;
  background: var(--td-brand-color-1);
}

.group-item.is-active {
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.1);
}

.group-icon {
  width: 22px;
  height: 22px;
}

.group-label {
  font-size: 12px;
  line-height: 1.4;
  text-align: center;
  transition: opacity 0.2s ease, width 0.2s ease;
  white-space: nowrap;
  overflow: hidden;
}

.sidebar-wrapper.is-collapsed .group-label.label-hidden {
  opacity: 1;
  width: auto;
  margin: initial;
  overflow: visible;
}

.menu-sidebar {
  width: 232px;
  background: #ffffff;
  border-right: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  padding: 0 0 12px;
  overflow: hidden;
  transition:
    width 0.28s cubic-bezier(0.4, 0, 0.2, 1),
    padding 0.28s cubic-bezier(0.4, 0, 0.2, 1),
    opacity 0.2s ease;
}

.menu-sidebar.is-closed {
  width: 60px;
  padding: 0;
  border-right-color: #e5e7eb;
  opacity: 1;
  overflow: visible;
}

.menu-sidebar.is-closed .menu-header {
  display: flex;
  justify-content: center;
  padding: 14px 0 6px;
}

.menu-sidebar.is-closed .menu-header__title {
  display: none;
}

.menu-sidebar.is-closed .menu-list {
  padding: 12px 6px;
  overflow: visible;
}

.menu-sidebar.is-closed .menu-item {
  margin-bottom: 8px;
}

.menu-sidebar.is-closed .menu-item-inner {
  justify-content: center;
  padding: 0;
  min-height: 48px;
  width: 48px;
  margin: 0 auto;
  border-radius: 12px;
}

.menu-sidebar.is-closed .menu-title,
.menu-sidebar.is-closed .menu-arrow {
  display: none;
}

.menu-sidebar.is-closed .menu-icon {
  width: 24px;
  height: 24px;
}

/* 收缩态二级菜单悬浮弹出 */
.menu-sidebar.is-closed .menu-item {
  position: relative;
}

/* hover 时抬升层级，避免弹窗被后续兄弟菜单项盖住导致“滑向弹窗即消失” */
.menu-sidebar.is-closed .menu-item.has-children:hover,
.menu-sidebar.is-closed .menu-item.has-children:focus-within {
  z-index: 120;
}

.menu-sidebar.is-closed .collapsed-popup {
  position: absolute;
  left: calc(100% + 4px);
  top: -12px;
  z-index: 100;
  width: 200px;
  background: #ffffff;
  border-radius: 10px;
  border: 1px solid #e5e7eb;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.10);
  overflow: hidden;
  display: none;
  padding-top: 12px;
}

/* 悬浮桥接 — 使鼠标从图标滑入弹窗不中断（覆盖 4px 间隙并向图标侧延伸 12px） */
.menu-sidebar.is-closed .collapsed-popup::before {
  content: '';
  position: absolute;
  right: 100%;
  top: 0;
  bottom: 0;
  width: 16px;
}

.menu-sidebar.is-closed .menu-item.has-children:hover .collapsed-popup,
.menu-sidebar.is-closed .collapsed-popup:hover {
  display: block;
}

.collapsed-popup__header {
  padding: 10px 14px 8px;
  font-size: 13px;
  font-weight: 700;
  color: #111827;
  border-bottom: 1px solid #f3f4f6;
  white-space: nowrap;
}

/* 移动端隐藏悬浮弹出 */
@media (max-width: 768px) {
  .collapsed-popup {
    display: none !important;
  }
}

.collapsed-popup__list {
  list-style: none;
  margin: 0;
  padding: 6px;
}

.collapsed-popup__item {
  margin-bottom: 2px;
}

.collapsed-popup__link {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding: 0 10px;
  border-radius: 8px;
  color: #374151;
  font-size: 13px;
  cursor: pointer;
  text-decoration: none;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.collapsed-popup__link:hover,
.collapsed-popup__item.is-active .collapsed-popup__link {
  background: var(--td-brand-color-1);
  color: var(--td-brand-color-9);
}

.collapsed-popup__dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.5;
  flex-shrink: 0;
}

.menu-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 18px 16px 14px 20px;
}

.menu-header__title {
  font-size: 16px;
  font-weight: 700;
  color: #111827;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.menu-collapse-btn {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  border: none;
  background: #f3f4f6;
  color: #374151;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.menu-collapse-btn:hover {
  background: var(--td-brand-color-1);
  color: var(--color-primary);
}

/* 移动端专用关闭按钮（桌面隐藏，移动端媒体查询中显示） */
.menu-close-btn {
  display: none;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  border: none;
  background: #f3f4f6;
  color: #374151;
  cursor: pointer;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.menu-close-btn:hover {
  background: var(--td-brand-color-1);
  color: var(--color-primary);
}

.menu-list {
  list-style: none;
  margin: 0;
  padding: 0 12px;
  flex: 1;
  overflow-y: auto;
}

.menu-item {
  margin-bottom: 4px;
}

.menu-item-inner {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 40px;
  padding: 0 12px;
  border-radius: 10px;
  color: #374151;
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.menu-item-inner:hover,
.menu-item.is-active > .menu-item-inner {
  background: var(--td-brand-color-1);
  color: var(--td-brand-color-9);
}

.menu-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  color: inherit;
}

.menu-title {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  font-weight: 500;
}

.menu-arrow {
  width: 16px;
  height: 16px;
  transition: transform 0.2s ease;
}

.menu-arrow.is-open {
  transform: rotate(90deg);
}

.submenu-list {
  list-style: none;
  margin: 4px 0 8px;
  padding: 0 0 0 12px;
}

.submenu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 36px;
  padding: 0 12px 0 20px;
  border-radius: 10px;
  color: #4b5563;
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.submenu-item:hover,
.submenu-item.is-active {
  background: var(--td-brand-color-1);
  color: var(--td-brand-color-9);
}

.submenu-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.6;
}

.submenu-title {
  font-size: 13px;
}

.main-layout {
  flex: 1;
  min-width: 0;
}

.top-header {
  height: 56px;
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 14px 0 24px;
  position: sticky;
  top: 0;
  z-index: 5;
}

.top-header__left {
  display: flex;
  align-items: center;
}

.top-header__right {
  display: flex;
  align-items: center;
  gap: 4px;
}

.user-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px 4px 4px;
  border-radius: 20px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.user-chip:hover {
  background: var(--td-brand-color-1);
}

.user-chip__avatar {
  background: var(--color-primary) !important;
  color: #ffffff !important;
  font-weight: 600;
  font-size: 13px;
}

.user-chip__name {
  font-size: 13px;
  font-weight: 500;
  color: #374151;
}

.user-chip__caret {
  color: #9ca3af;
}

.content-area {
  padding: 10px;
}

.content-inner {
  background: #ffffff;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
  padding: 19px;
  min-height: calc(100vh - 118px);
}

/* ===== 收缩切换按钮（头部） ===== */
.sidebar-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: #f3f4f6;
  border: none;
  color: #374151;
  cursor: pointer;
  padding: 0;
  margin-right: 15px;
  transition:
    background-color 0.2s ease,
    color 0.2s ease;
}

.sidebar-toggle:hover {
  background: var(--td-brand-color-1);
  color: var(--color-primary);
}

/* ===== 移动端遮罩 ===== */
.sidebar-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  z-index: 50;
  backdrop-filter: blur(2px);
}

.overlay-fade-enter-active,
.overlay-fade-leave-active {
  transition: opacity 0.25s ease;
}

.overlay-fade-enter-from,
.overlay-fade-leave-to {
  opacity: 0;
}

/* ===== 移动端菜单按钮 ===== */
.mobile-menu-btn {
  display: none;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: #f3f4f6;
  border: 0px solid #d1d5db;
  color: #374151;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  margin-right: 10px;
  transition: background-color 0.2s ease, border-color 0.2s ease;
}

.mobile-menu-btn:hover {
  background: #e5e7eb;
  border-color: #9ca3af;
}

/* ===== 收缩态二级悬浮弹出（popup-fade 过渡） ===== */
.popup-fade-enter-active,
.popup-fade-leave-active {
  transition: opacity 0.15s ease;
}

.popup-fade-enter-from,
.popup-fade-leave-to {
  opacity: 0;
}

/* ===== 响应式：移动端 ===== */
@media (max-width: 768px) {
  .mobile-menu-btn {
    display: inline-flex;
  }

  .sidebar-wrapper {
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    width: 320px !important;
    z-index: 60;
    transform: translateX(-100%);
    transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 4px 0 20px rgba(0, 0, 0, 0.10);
  }

  .sidebar-wrapper.is-mobile-open {
    transform: translateX(0);
  }

  .sidebar-wrapper.is-collapsed {
    width: 320px !important;
  }

  .sidebar-wrapper.is-collapsed .menu-sidebar {
    width: 232px;
    padding: 0 0 12px;
    opacity: 1;
    border-right-color: #e5e7eb;
  }

  .sidebar-wrapper.is-collapsed .group-label {
    opacity: 1;
    width: auto;
    margin: initial;
    overflow: visible;
  }

  /* 移动端强制展开二级菜单内容 */
  .sidebar-wrapper.is-collapsed .menu-sidebar.is-closed .menu-header {
    display: flex;
  }

  .sidebar-wrapper.is-collapsed .menu-sidebar.is-closed .menu-title,
  .sidebar-wrapper.is-collapsed .menu-sidebar.is-closed .menu-arrow {
    display: flex;
  }

  .sidebar-wrapper.is-collapsed .menu-sidebar.is-closed .menu-item-inner {
    justify-content: flex-start;
    width: auto;
    min-height: 40px;
    padding: 0 12px;
    margin: 0;
    border-radius: 10px;
  }

  .sidebar-wrapper.is-collapsed .menu-sidebar.is-closed .menu-icon {
    width: 18px;
    height: 18px;
  }

  /* 移动端隐藏悬浮弹出 */
  .collapsed-popup {
    display: none !important;
  }

  .sidebar-toggle {
    display: none;
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slide-enter-active {
  transition:
    opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1) 0.05s,
    transform 0.25s cubic-bezier(0.4, 0, 0.2, 1) 0.05s;
  will-change: opacity, transform;
}

.slide-leave-active {
  transition:
    opacity 0.15s cubic-bezier(0.4, 0, 0.2, 1),
    transform 0.15s cubic-bezier(0.4, 0, 0.2, 1);
  will-change: opacity, transform;
}

.slide-enter-from,
.slide-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.slide-enter-to,
.slide-leave-from {
  opacity: 1;
  transform: translateY(0);
}

/* ===== 导航搜索框 ===== */
.navbar-search {
  position: relative;
  display: flex;
  align-items: center;
  width: 260px;
  margin-right: 12px;
}

.navbar-search__icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  color: #9ca3af;
  pointer-events: none;
}

.navbar-search__input {
  width: 100%;
  height: 34px;
  padding: 0 12px 0 34px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f8fafc;
  color: #374151;
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.navbar-search__input::placeholder {
  color: #9ca3af;
}

.navbar-search__input:focus {
  border-color: var(--color-primary);
  background: #ffffff;
  box-shadow: 0 0 0 2px rgba(22, 163, 74, 0.12);
}

.navbar-search__dropdown {
  position: absolute;
  top: 40px;
  left: 0;
  right: 0;
  z-index: 40;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.1);
  overflow: hidden;
  max-height: 320px;
  overflow-y: auto;
}

.navbar-search__item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  font-size: 13px;
  color: #374151;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.navbar-search__item:hover {
  background: var(--td-brand-color-1);
  color: var(--td-brand-color-9);
}

.navbar-search__item-icon {
  display: inline-flex;
  width: 18px;
  height: 18px;
  color: var(--color-primary);
}

.navbar-search__empty {
  padding: 12px;
  font-size: 12.5px;
  color: #9ca3af;
  text-align: center;
}

/* ===== 已打开页面标签栏 ===== */
.tabs-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 42px;
  padding: 0 12px;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}

.tabs-bar__scroll {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  overflow-x: auto;
  scrollbar-width: none;
}

.tabs-bar__scroll::-webkit-scrollbar {
  display: none;
}

.tabs-bar__tab {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 8px 0 12px;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
  background: #f8fafc;
  color: #4b5563;
  font-size: 13px;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.tabs-bar__tab:hover {
  border-color: var(--color-primary);
  color: var(--td-brand-color-9);
}

.tabs-bar__tab.is-active {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #ffffff;
}

.tabs-bar__title {
  white-space: nowrap;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tabs-bar__pin,
.tabs-bar__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: none;
  background: transparent;
  color: inherit;
  cursor: pointer;
  padding: 0;
  opacity: 0.75;
}

.tabs-bar__pin {
  opacity: 0.35;
}

.tabs-bar__tab:hover .tabs-bar__pin {
  opacity: 0.8;
}

.tabs-bar__pin.is-pinned {
  opacity: 1;
  color: var(--color-primary);
}

.tabs-bar__tab.is-active .tabs-bar__pin.is-pinned {
  color: #ffffff;
}

.tabs-bar__pin:hover,
.tabs-bar__close:hover {
  opacity: 1;
  background: rgba(0, 0, 0, 0.08);
}

.tabs-bar__tab.is-active .tabs-bar__pin,
.tabs-bar__tab.is-active .tabs-bar__close {
  color: #ffffff;
}

.tabs-bar__more {
  flex-shrink: 0;
  color: #4b5563;
}

/* ===== 移动端：导航搜索自适应 + 用户区只留头像 ===== */
@media (max-width: 768px) {
  /* 收紧导航栏左右内边距，用户头像更靠右 */
  .top-header {
    padding: 0 10px;
  }

  .navbar-search {
    width: auto;
    flex: 1;
    min-width: 0;
    margin-right: 8px;
  }

  .navbar-search__dropdown {
    position: fixed;
    left: 16px;
    right: 16px;
    top: 60px;
  }

  /* 用户信息只显示头像，点头像展开菜单 */
  .user-chip {
    padding: 4px;
  }

  .user-chip__name,
  .user-chip__caret {
    display: none;
  }

  /* 抽屉菜单：隐藏 PC 收缩按钮，只保留移动端关闭按钮 */
  .menu-collapse-btn {
    display: none;
  }

  .menu-close-btn {
    display: inline-flex;
  }

  .sidebar-wrapper.is-collapsed .menu-sidebar.is-closed .menu-header {
    display: flex;
  }
}

/* ===== 深色主题（壳层 chrome，纯黑基调） ===== */
.dark .admin-layout {
  background: #000000;
}

.dark .sidebar-wrapper,
.dark .group-sidebar,
.dark .menu-sidebar,
.dark .top-header,
.dark .tabs-bar,
.dark .content-inner {
  background: #000000;
  border-color: #262626;
}

.dark .menu-header__title,
.dark .brand-name,
.dark .group-item,
.dark .menu-title,
.dark .submenu-title,
.dark .user-chip__name,
.dark .collapsed-popup__link {
  color: #e5e7eb;
}

.dark .group-item:hover,
.dark .group-item.is-active,
.dark .menu-item-inner:hover,
.dark .menu-item.is-active > .menu-item-inner,
.dark .submenu-item:hover,
.dark .submenu-item.is-active,
.dark .collapsed-popup__link:hover {
  background: #161616;
  color: var(--td-brand-color-4);
}

.dark .menu-item-inner,
.dark .collapsed-popup__link {
  color: #d4d4d4;
}

.dark .menu-collapse-btn,
.dark .menu-close-btn,
.dark .sidebar-toggle,
.dark .mobile-menu-btn {
  background: #161616;
  color: #d4d4d4;
}

.dark .navbar-search__input {
  background: #111111;
  border-color: #262626;
  color: #e5e7eb;
}

.dark .navbar-search__dropdown,
.dark .collapsed-popup {
  background: #0a0a0a;
  border-color: #262626;
}

.dark .navbar-search__item {
  color: #e5e7eb;
}

.dark .navbar-search__item:hover {
  background: #161616;
  color: var(--td-brand-color-4);
}

.dark .tabs-bar__tab {
  background: #111111;
  border-color: #262626;
  color: #d4d4d4;
}

.dark .tabs-bar__tab:hover {
  border-color: var(--td-brand-color-6);
  color: var(--td-brand-color-4);
}

.dark .tabs-bar__tab.is-active {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #ffffff;
}

.dark .user-chip:hover {
  background: #161616;
}
</style>
