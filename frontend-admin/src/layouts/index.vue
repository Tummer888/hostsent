<template>
  <div class="admin-layout">
    <!-- 侧边栏区域 -->
    <div class="sidebar-wrapper">
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
            <span class="group-label">{{ group.name }}</span>
          </li>
        </ul>
      </aside>

      <!-- 2. 主菜单/二三级栏 -->
      <aside class="menu-sidebar">
        <div class="menu-header">{{ currentGroup?.name || '菜单' }}</div>
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
              <!-- 三级菜单 -->
              <transition name="slide">
                <ul v-if="isExpanded(menu)" class="submenu-list">
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

        <!-- 底部按钮组 -->
        <div class="sidebar-footer">
          <div class="footer-btn" :title="useFallbackMenu ? '使用静态菜单（后端不可用）' : '菜单管理'" @click="router.push('/system/menus')">
            <MenuIcon />
            <span v-if="useFallbackMenu" class="fallback-dot" aria-label="降级模式"></span>
          </div>
          <div class="footer-btn" title="返回仪表盘" @click="router.push('/dashboard/base')">
            <DashboardIcon />
          </div>
          <!-- 加载指示器 -->
          <div v-if="menuLoading" class="loading-indicator" aria-label="加载中">
            <div class="loading-spinner"></div>
          </div>
        </div>
      </aside>
    </div>

    <!-- 3. 主内容区 -->
    <t-layout direction="vertical" class="main-layout">
      <t-header class="top-header">
        <div class="top-header__left">
          <t-breadcrumb separator="/">
            <t-breadcrumb-item>首页</t-breadcrumb-item>
            <t-breadcrumb-item v-if="currentGroup">{{ currentGroup.name }}</t-breadcrumb-item>
            <t-breadcrumb-item>{{ currentMenuTitle }}</t-breadcrumb-item>
          </t-breadcrumb>
        </div>
        <div class="top-header__right">
          <!-- 刷新菜单 -->
          <t-tooltip content="刷新菜单" placement="bottom">
            <t-button
              variant="text"
              shape="square"
              aria-label="刷新菜单"
              :loading="menuLoading"
              @click="refreshMenus"
            >
              <template #icon>
                <RefreshIcon />
              </template>
            </t-button>
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
              <t-avatar :size="30" class="user-chip__avatar">
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
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import {
  ChevronDownIcon,
  ChevronRightIcon,
  DashboardIcon,
  MenuIcon,
  NotificationIcon,
  PoweroffIcon,
  RefreshIcon,
  SettingIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { navMenu } from '@/permission'
import { useMenuStore } from '@/store/modules/menu'
import { useUserStore } from '@/store/modules/user'
import type { FlatMenu } from '@/store/modules/menu'

defineOptions({ name: 'AdminLayout' })

const router = useRouter()
const route = useRoute()
const menuStore = useMenuStore()
const userStore = useUserStore()

const expandedKeys = ref<Set<string>>(new Set())
const menuLoading = ref(false)
const useFallbackMenu = ref(false)

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
    icon: undefined,
    children: (child.children || []).map((gc: any) => ({
      id: 0,
      parentId: 0,
      name: gc.title,
      path: gc.path,
      icon: undefined,
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
})

const activeGroup = ref<string>('')

const currentGroup = computed(() => {
  return sidebarItems.value.find((item) => (item.path || item.name) === activeGroup.value)
})

const currentMenuList = computed(() => currentGroup.value?.children || [])

const currentMenuTitle = computed(() => {
  for (const menu of currentMenuList.value) {
    if (menu.path === route.path) return menu.name
    const hit = menu.children?.find((child) => child.path === route.path)
    if (hit) return hit.name
  }
  return '工作台'
})

function isMenuActive(menu: FlatMenu) {
  if (menu.path === route.path) return true
  return menu.children?.some((child) => child.path === route.path) || false
}

function isExpanded(menu: FlatMenu) {
  return expandedKeys.value.has(menu.path || menu.name)
}

function toggleExpand(menu: FlatMenu) {
  const key = menu.path || menu.name
  const next = new Set(expandedKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedKeys.value = next
}

function navigateTo(path?: string) {
  if (!path) return
  router.push(path)
}

function onGroupClick(group: FlatMenu) {
  activeGroup.value = group.path || group.name
  const first = group.children?.[0]
  if (!first) return
  if (first.children?.length) {
    expandedKeys.value = new Set([first.path || first.name])
    navigateTo(first.children[0].path)
    return
  }
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

async function refreshMenus() {
  menuLoading.value = true
  try {
    await menuStore.loadMenus('admin')
    useFallbackMenu.value = false
    initByRoute()
    MessagePlugin.success('菜单已刷新')
  } catch {
    useFallbackMenu.value = true
    MessagePlugin.error('菜单刷新失败，已使用本地菜单')
  } finally {
    menuLoading.value = false
  }
}

function onUserMenuClick(data: { value: string }) {
  if (data.value === 'logout') {
    userStore.logout()
    router.push('/login')
    return
  }
  if (data.value === 'profile') {
    MessagePlugin.info('个人资料开发中')
    return
  }
  if (data.value === 'settings') {
    MessagePlugin.info('账号设置开发中')
  }
}

watch(
  () => route.path,
  () => {
    initByRoute()
  },
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
}

.group-sidebar {
  width: 88px;
  background: linear-gradient(180deg, #14532d 0%, #166534 42%, #15803d 100%);
  color: #ffffff;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 0 12px;
}

.group-logo {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.14);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 20px;
  margin-bottom: 18px;
}

.group-list {
  list-style: none;
  margin: 0;
  padding: 0 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.group-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  min-height: 72px;
  border-radius: 16px;
  color: rgba(255, 255, 255, 0.76);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

.group-item:hover,
.group-item.is-active {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.12);
}

.group-item.is-active {
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.1);
}

.group-icon {
  width: 18px;
  height: 18px;
}

.group-label {
  font-size: 12px;
  line-height: 1.4;
  text-align: center;
}

.menu-sidebar {
  width: 232px;
  background: #ffffff;
  border-right: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  padding: 0 0 12px;
}

.menu-header {
  padding: 22px 20px 16px;
  font-size: 16px;
  font-weight: 700;
  color: #111827;
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
  background: #f0fdf4;
  color: #166534;
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
  background: #f0fdf4;
  color: #166534;
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

.sidebar-footer {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border-top: 1px solid #f3f4f6;
}

.footer-btn {
  position: relative;
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: #f8fafc;
  color: #4b5563;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.footer-btn:hover {
  background: #f0fdf4;
  color: #166534;
}

.fallback-dot {
  position: absolute;
  top: 7px;
  right: 7px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #f59e0b;
}

.loading-indicator {
  position: absolute;
  bottom: 20px;
  right: 20px;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid #e5e7eb;
  border-top-color: #16a34a;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
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
  padding: 0 24px;
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
  background: #f0fdf4;
}

.user-chip__avatar {
  background: #16a34a !important;
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
  min-height: calc(100vh - 56px - 30px);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slide-enter-active,
.slide-leave-active {
  transition: all 0.18s ease;
}

.slide-enter-from,
.slide-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
