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

// ========== 用户信息 ==========
const userName = computed(() => {
  const { name, username } = userStore.userInfo
  return name || username || '管理员'
})

const userInitial = computed(() => {
  const base = userStore.userInfo.name || userStore.userInfo.username || 'A'
  return base.slice(0, 1).toUpperCase()
})

// ========== 图标渲染 ==========

// 图标解析函数：将各种图标格式统一为可渲染的组件
// 支持格式：TDesign 组件、shallowRef 包装的 render 对象
function resolveMenuIcon(icon: any): any {
  if (!icon) return undefined
  
  // 检查是否是 shallowRef 包装的对象（来自 iconWrapper）
  // shallowRef.value 会返回原始值
  if (icon && typeof icon === 'object' && '__v_isRef' in icon) {
    const inner = icon.value
    if (inner && inner.render && typeof inner.render === 'function') {
      // 返回可以被 <component :is> 渲染的对象
      return { render: inner.render }
    }
    return inner || icon
  }
  
  // 检查是否直接是带 render 方法的对象
  if (icon && typeof icon === 'object' && icon.render && typeof icon.render === 'function') {
    return icon
  }
  
  // 直接的 TDesign 组件（有 __name 或 name 属性）
  return icon
}

// 将静态 navMenu 转换为 FlatMenu 格式作为降级数据
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

// 优先使用后端菜单，失败时降级到静态菜单
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
  // 初始化激活的分组和展开项
  initStateFromRoute()

  // ========== 🎨 彩蛋时间 ==========
  console.log('%c🎉 欢迎来到宿派云控管理平台！', 'color: #16a34a; font-size: 18px; font-weight: bold;')
  console.log('%c💡 小贴士：在控制台输入 "hostsent" 查看隐藏信息', 'color: #6b7280; font-size: 12px;')
  
  // 控制台彩蛋
  const secrets = [
    '🎯 你发现了一个隐藏菜单！继续探索会有更多惊喜...',
    '🚀 宿派云控正在为你保驾护航，让云端管理变得简单！',
    '💎 提示：按 Ctrl+Shift+E 试试，看看会发生什么？',
    '🎪 恭喜你成为宿派云控的优秀管理员！',
    '🌟 代码是诗，你的管理是韵，让我们一起创造云的未来！',
  ]
  
  // @ts-ignore
  window.hostsent = {
    version: '1.0.0',
    get secret() {
      const idx = Math.floor(Math.random() * secrets.length)
      console.log(`%c${secrets[idx]}`, 'color: #16a34a; font-size: 14px;')
      return '🎉 你发现了彩蛋！'
    },
    help: () => {
      console.log('%c📚 宿派云控 API:', 'color: #16a34a; font-weight: bold;')
      console.log('  window.hostsent.secret  - 获取随机彩蛋消息')
      console.log('  window.hostsent.help()  - 显示此帮助')
      console.log('  window.hostsent.version - 查看版本号')
    }
  }
  
  // 键盘快捷键彩蛋
  const konamiCode = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a']
  let konamiIndex = 0
  
  const handleKeydown = (e: KeyboardEvent) => {
    if (e.key === konamiCode[konamiIndex]) {
      konamiIndex++
      if (konamiIndex === konamiCode.length) {
        // 触发彩蛋效果
        document.body.style.transition = 'transform 0.5s'
        document.body.style.transform = 'rotate(360deg)'
        setTimeout(() => {
          document.body.style.transition = 'transform 1s'
          document.body.style.transform = 'rotate(0deg)'
          console.log('%c🎮 金手指激活！宿派云控奖励你一个旋转世界！', 'color: #16a34a; font-size: 16px; font-weight: bold;')
        }, 500)
        konamiIndex = 0
      }
    } else {
      konamiIndex = e.key === konamiCode[0] ? 1 : 0
    }
  }
  window.addEventListener('keydown', handleKeydown)
})

// ========== 状态计算 ==========
const activeGroup = ref<string>('')

const currentGroup = computed<FlatMenu | undefined>(() => {
  return sidebarItems.value.find(
    (item) => item.path === activeGroup.value,
  )
})

const currentMenuList = computed<FlatMenu[]>(() => {
  return currentGroup.value?.children || []
})

const currentMenuTitle = computed(() => {
  const path = route.path
  for (const menu of currentMenuList.value) {
    if (menu.path === path) return menu.name
    if (menu.children) {
      const sub = menu.children.find((s: FlatMenu) => s.path === path)
      if (sub) return sub.name
    }
  }
  return route.meta?.title as string || ''
})

// ========== 方法 ==========
function initStateFromRoute() {
  const path = route.path
  
  // 调试日志
  if (import.meta.env.DEV) {
    console.log('[Menu Debug] initStateFromRoute:', {
      currentPath: path,
      sidebarItemsCount: sidebarItems.value.length,
      menus: sidebarItems.value.map(item => ({ name: item.name, path: item.path, childrenCount: item.children?.length || 0 }))
    })
  }
  
  // 查找当前路由所属的顶级菜单
  for (const group of sidebarItems.value) {
    if (group.path && (path === group.path || path.startsWith(group.path + '/'))) {
      activeGroup.value = group.path
      // 默认展开包含当前路由的子菜单
      if (group.children) {
        for (const child of group.children) {
          if (child.children?.some((c: FlatMenu) => c.path && (c.path === path || path.startsWith(c.path)))) {
            expandedKeys.value.add(child.path || child.name)
          }
        }
      }
      return
    }
  }
  // 默认选中第一个分组
  if (sidebarItems.value.length > 0) {
    const firstGroup = sidebarItems.value[0]
    activeGroup.value = firstGroup.path || firstGroup.name
    
    // 如果第一个分组有子菜单，自动跳转到第一个可见的子菜单
    if (firstGroup.children && firstGroup.children.length > 0) {
      const target = findFirstLeaf(firstGroup)
      if (target?.path && target.path !== path) {
        // 不在 initStateFromRoute 中自动跳转，避免无限循环
        // 只设置 activeGroup，让 onGroupClick 处理跳转
      }
    }
  }
}

function onGroupClick(group: FlatMenu) {
  // 一级菜单只切换激活状态，不跳转路由
  activeGroup.value = group.path || group.name
}

async function refreshMenus() {
  menuLoading.value = true
  try {
    await menuStore.loadMenus('admin')
    useFallbackMenu.value = false
    MessagePlugin.success('菜单已同步')
  } catch (e) {
    console.error('Failed to refresh menus:', e)
    useFallbackMenu.value = true
    MessagePlugin.warning('后端不可用，当前使用静态菜单')
  } finally {
    menuLoading.value = false
  }
}

async function onUserMenuClick(data: { value?: string }) {
  const value = data?.value
  if (value === 'logout') {
    await userStore.logout()
    MessagePlugin.success('已退出登录')
    router.replace('/login')
  } else if (value === 'profile') {
    MessagePlugin.info('个人资料模块即将上线')
  } else if (value === 'settings') {
    MessagePlugin.info('账号设置模块即将上线')
  }
}

function findFirstLeaf(menu: FlatMenu): FlatMenu | undefined {
  if (!menu.children || menu.children.length === 0) {
    return menu
  }
  for (const child of menu.children) {
    const leaf = findFirstLeaf(child)
    if (leaf) return leaf
  }
  return undefined
}

function isExpanded(menu: FlatMenu): boolean {
  return expandedKeys.value.has(menu.path || menu.name)
}

function toggleExpand(menu: FlatMenu) {
  const key = menu.path || menu.name
  if (expandedKeys.value.has(key)) {
    expandedKeys.value.delete(key)
  } else {
    expandedKeys.value.add(key)
  }
  // 触发 Vue 响应式更新
  expandedKeys.value = new Set(expandedKeys.value)
}

function isMenuActive(menu: FlatMenu): boolean {
  if (route.path === menu.path) return true
  if (menu.children?.some((c: FlatMenu) => c.path === route.path)) return true
  return false
}

function navigateTo(path?: string | number) {
  if (path) {
    router.push(String(path))
  }
}

// 路由变化时同步状态
watch(() => route.path, () => {
  // 仅在初始化后或路由变化时同步选中项
  if (activeGroup.value) {
    const group = sidebarItems.value.find((item) =>
      item.path && route.path.startsWith(item.path)
    )
    if (group && activeGroup.value !== (group.path || group.name)) {
      activeGroup.value = group.path || group.name
    }
  }
}, { immediate: false })

// 确保菜单加载后初始化状态
watch(sidebarItems, () => {
  if (!activeGroup.value && sidebarItems.value.length > 0) {
    initStateFromRoute()
  }
}, { immediate: true })

</script>

<style scoped>
.admin-layout {
  display: flex;
  flex-direction: row;
  min-height: 100vh;
  background: #f5f7fa;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
}

/* ============ 侧边栏容器 ============ */
.sidebar-wrapper {
  display: flex;
  height: 100vh;
  position: sticky;
  top: 0;
  flex-shrink: 0;
  z-index: 10;
}

/* ============ 分组栏 ============ */
.group-sidebar {
  width: 80px;
  background: #ffffff;
  border-right: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 0;
}

.group-logo {
  width: 36px;
  height: 36px;
  background: #16a34a; /* 绿色 Logo */
  color: #fff;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
  font-size: 14px;
  margin-bottom: 24px;
}

.group-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  align-items: center;
}

.group-item {
  width: 64px;
  height: 64px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border-radius: 8px;
  cursor: pointer;
  color: #6b7280;
  transition: all 0.2s;
  font-size: 12px;
}

.group-item:hover {
  background: #f0fdf4; /* 浅绿色 Hover */
  color: #15803d;
}

.group-item.is-active {
  background: #16a34a; /* 绿色选中背景 */
  color: #ffffff;
  box-shadow: 0 2px 4px rgba(22, 163, 74, 0.2);
}

.group-icon {
  font-size: 24px;
}

/* ============ 主菜单栏 ============ */
.menu-sidebar {
  width: 240px;
  background: #ffffff;
  border-right: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.menu-header {
  height: 56px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  font-size: 16px;
  font-weight: 600;
  color: #111827;
  border-bottom: 1px solid #e5e7eb;
}

.menu-list {
  list-style: none;
  padding: 12px 8px;
  margin: 0;
  flex: 1;
  overflow-y: auto;
}

/* 一级菜单 */
.menu-item {
  margin-bottom: 4px;
  border-radius: 6px;
  overflow: hidden;
}

.menu-item-inner {
  display: flex;
  align-items: center;
  height: 40px;
  padding: 0 12px;
  border-radius: 6px;
  cursor: pointer;
  color: #374151;
  font-size: 14px;
  transition: all 0.2s;
  gap: 10px;
}

.menu-item-inner:hover {
  background: #f0fdf4; /* 绿色 Hover */
  color: #15803d;
}

.menu-item.is-active > .menu-item-inner {
  background: #dcfce7; /* 浅绿选中背景 */
  color: #166534;
  font-weight: 500;
}

.menu-icon {
  font-size: 18px;
  display: flex;
  align-items: center;
}

.menu-title {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.menu-arrow {
  font-size: 12px;
  color: #9ca3af;
  transition: transform 0.2s;
}

.menu-arrow.is-open {
  transform: rotate(90deg);
  color: #16a34a;
}

/* 二级/三级子菜单 */
.submenu-list {
  list-style: none;
  padding: 4px 12px 4px 44px;
  margin: 0;
}

.submenu-item {
  height: 34px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  border-radius: 6px;
  cursor: pointer;
  color: #4b5563;
  font-size: 13px;
  transition: all 0.2s;
}

.submenu-item:hover {
  background: #f0fdf4;
  color: #15803d;
}

.submenu-item.is-active {
  background: #dcfce7;
  color: #166534;
  font-weight: 500;
}

.submenu-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #d1d5db;
  transition: all 0.2s;
}

.submenu-item:hover .submenu-dot,
.submenu-item.is-active .submenu-dot {
  background: #16a34a;
}

/* 动画 */
.slide-enter-active,
.slide-leave-active {
  transition: all 0.2s ease;
  overflow: hidden;
}
.slide-enter-from,
.slide-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* ============ 底部按钮 ============ */
.sidebar-footer {
  padding: 16px;
  border-top: 1px solid #e5e7eb;
  display: flex;
  gap: 12px;
  justify-content: center;
}

.footer-btn {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6b7280;
  cursor: pointer;
  transition: all 0.2s;
}

.footer-btn:hover {
  background: #f0fdf4;
  color: #16a34a;
  position: relative;
}

/* 降级模式指示器 */
.fallback-dot {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #f59e0b;
  box-shadow: 0 0 4px rgba(245, 158, 11, 0.5);
  animation: pulse-dot 1.5s ease-in-out infinite;
}

@keyframes pulse-dot {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(0.8); }
}

/* 加载指示器 */
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
  to { transform: rotate(360deg); }
}

.sidebar-footer {
  position: relative;
}

/* ============ 主内容区 ============ */
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

/* 用户卡片 */
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
  padding: 20px;
}

.content-inner {
  background: #ffffff;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
  padding: 24px;
  min-height: calc(100vh - 56px - 40px);
}
</style>
