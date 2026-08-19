<template>
  <div class="user-layout">
    <!-- 移动端遮罩 -->
    <div
      v-if="!isCollapsed && isMobile"
      class="sidebar-overlay"
      @click="isCollapsed = true"
    ></div>

    <!-- 侧边栏 -->
    <aside class="sidebar" :class="{ 'is-collapsed': isCollapsed }">
      <div class="sidebar-header">
        <div class="logo">
          <span class="logo-icon">H</span>
          <span class="logo-text" v-show="!isCollapsed">宿派云控</span>
        </div>
      </div>
      
      <nav class="sidebar-nav">
        <template v-if="menuStore.hasMenu">
          <!-- 动态菜单 -->
          <template v-for="menu in menuStore.sidebarMenus" :key="menu.path || menu.name">
            <!-- 有子菜单 -->
            <div
              v-if="menu.children && menu.children.length > 0"
              class="nav-group"
              :class="{ 'is-expanded': expandedGroups.has(menu.path || menu.name) }"
            >
              <div
                class="nav-item"
                :class="{ 'is-active': isGroupActive(menu) }"
                @click="toggleGroup(menu)"
              >
                <component :is="menu.icon" v-if="menu.icon" class="nav-icon" />
                <span class="nav-label" v-show="!isCollapsed">{{ menu.name }}</span>
                <ChevronRightIcon class="nav-arrow" :class="{ 'is-rotated': expandedGroups.has(menu.path || menu.name) }" />
              </div>
              <div class="sub-menu" v-show="expandedGroups.has(menu.path || menu.name) && !isCollapsed">
                <router-link
                  v-for="child in menu.children"
                  :key="child.path || child.name"
                  :to="child.path ? '/' + child.path : '#'"
                  class="sub-menu-item"
                  :class="{ 'is-active': route.path === '/' + child.path }"
                >
                  <span class="sub-menu-dot"></span>
                  <span class="sub-menu-label">{{ child.name }}</span>
                </router-link>
              </div>
            </div>
            
            <!-- 无子菜单 -->
            <router-link
              v-else
              :to="menu.path ? '/' + menu.path : '#'"
              class="nav-item"
              :class="{ 'is-active': route.path === '/' + menu.path }"
            >
              <component :is="menu.icon" v-if="menu.icon" class="nav-icon" />
              <span class="nav-label" v-show="!isCollapsed">{{ menu.name }}</span>
            </router-link>
          </template>
        </template>
        
        <!-- 降级静态菜单 -->
        <template v-else>
          <template v-for="item in fallbackMenus" :key="item.path">
            <router-link
              :to="item.path || '/'"
              class="nav-item"
              :class="{ 'is-active': route.path === item.path }"
            >
              <component :is="item.icon" class="nav-icon" />
              <span class="nav-label">{{ item.name }}</span>
            </router-link>
          </template>
        </template>
      </nav>
      
      <div class="sidebar-footer" v-show="!isCollapsed">
        <div class="user-mini-info">
          <t-avatar :size="32" class="user-avatar">
            {{ userInitial }}
          </t-avatar>
          <div class="user-detail">
            <span class="user-name">{{ userName }}</span>
            <span class="user-role">普通用户</span>
          </div>
        </div>
      </div>
    </aside>
    
    <!-- 主内容区 -->
    <div class="main-wrapper">
      <!-- 顶部导航 -->
      <header class="top-header">
        <div class="header-left">
          <t-button
            variant="text"
            shape="square"
            aria-label="折叠菜单"
            @click="toggleSidebar"
            class="collapse-btn"
          >
            <template #icon>
              <MenuIcon />
            </template>
          </t-button>
          
          <t-breadcrumb separator="/" class="breadcrumb">
            <t-breadcrumb-item :to="{ path: '/' }">首页</t-breadcrumb-item>
            <t-breadcrumb-item v-if="currentMenuTitle">{{ currentMenuTitle }}</t-breadcrumb-item>
          </t-breadcrumb>
        </div>
        
        <div class="header-right">
          <t-tooltip content="刷新" placement="bottom">
            <t-button variant="text" shape="square" aria-label="刷新" @click="refreshMenus">
              <template #icon>
                <RefreshIcon />
              </template>
            </t-button>
          </t-tooltip>
          
          <t-dropdown trigger="click">
            <div class="user-dropdown-trigger">
              <t-avatar :size="36" class="header-avatar">
                {{ userInitial }}
              </t-avatar>
              <ChevronDownIcon class="dropdown-caret" />
            </div>
            <template #dropdown>
              <t-dropdown-menu>
                <t-dropdown-item value="profile">
                  <template #icon><UserIcon /></template>
                  个人中心
                </t-dropdown-item>
                <t-dropdown-item value="billing">
                  <template #icon><WalletIcon /></template>
                  费用中心
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
      </header>
      
      <!-- 内容区 -->
      <main class="content-area">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  ChevronRightIcon,
  ChevronDownIcon,
  MenuIcon,
  RefreshIcon,
  UserIcon,
  WalletIcon,
  PoweroffIcon,
  DashboardIcon,
  CloudIcon,
  OrderIcon,
  LayersIcon,
  TicketIcon,
} from 'tdesign-icons-vue-next'

import { useMenuStore, useUserStore } from '@/store'
import type { FlatMenu } from '@/store/modules/menu'

defineOptions({ name: 'UserLayout' })

const router = useRouter()
const route = useRoute()
const menuStore = useMenuStore()
const userStore = useUserStore()

const isCollapsed = ref(false)
const expandedGroups = ref<Set<string>>(new Set())

// ========== 响应式检测 ==========
const isMobile = ref(false)

function checkMobile() {
  isMobile.value = window.innerWidth <= 768
  if (isMobile.value) {
    isCollapsed.value = true
  }
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', checkMobile)
})

// ========== 用户信息 ==========
const userName = computed(() => userStore.displayName || '用户')
const userInitial = computed(() => {
  const base = userStore.userInfo?.name || userStore.userInfo?.username || 'U'
  return base.slice(0, 1).toUpperCase()
})

// ========== 降级菜单 ==========
const fallbackMenus: FlatMenu[] = [
  { id: 1, parentId: 0, path: '/dashboard', name: '控制台', icon: DashboardIcon },
  { id: 2, parentId: 0, path: '/cloud/instances', name: '我的云主机', icon: CloudIcon },
  { id: 3, parentId: 0, path: '/order', name: '我的订单', icon: OrderIcon },
  { id: 4, parentId: 0, path: '/billing', name: '费用中心', icon: WalletIcon },
  { id: 5, parentId: 0, path: '/support/tickets', name: '我的工单', icon: TicketIcon },
  { id: 6, parentId: 0, path: '/profile', name: '个人中心', icon: UserIcon },
]

// ========== 菜单操作 ==========
function toggleSidebar() {
  isCollapsed.value = !isCollapsed.value
}

function toggleGroup(menu: FlatMenu) {
  const key = menu.path || menu.name
  if (expandedGroups.value.has(key)) {
    expandedGroups.value.delete(key)
  } else {
    expandedGroups.value.add(key)
  }
}

function isGroupActive(menu: FlatMenu): boolean {
  const basePath = '/' + (menu.path || '')
  return route.path.startsWith(basePath)
}

const currentMenuTitle = computed(() => {
  const path = route.path
  if (path === '/') return '控制台'
  
  // 从路由 meta 获取标题
  const matched = route.matched
  for (const m of matched) {
    if (m.meta?.title && m.path && path.startsWith(m.path)) {
      return m.meta.title as string
    }
  }
  
  // 从菜单树查找
  const findInMenu = (menus: FlatMenu[]): string | null => {
    for (const menu of menus) {
      if ('/' + (menu.path || '') === path) return menu.name
      if (menu.children) {
        const found = findInMenu(menu.children)
        if (found) return found
      }
    }
    return null
  }
  
  return findInMenu(menuStore.sidebarMenus) || findInMenu(fallbackMenus) || ''
})

// ========== 初始化 ==========
onMounted(async () => {
  try {
    if (!menuStore.loaded) {
      await menuStore.loadMenus('user')
    }
    // 默认展开第一个有子菜单的分组
    if (menuStore.sidebarMenus.length > 0) {
      const firstWithChildren = menuStore.sidebarMenus.find(
        m => m.children && m.children.length > 0
      )
      if (firstWithChildren) {
        expandedGroups.value.add(firstWithChildren.path || firstWithChildren.name)
      }
    }
  } catch (e) {
    console.warn('Using fallback menu:', e)
  }
})

async function refreshMenus() {
  try {
    await menuStore.loadMenus('user')
    MessagePlugin.success('菜单已刷新')
  } catch (e) {
    console.error('Refresh failed:', e)
  }
}

function handleDropdownClick(value: string) {
  if (value === 'logout') {
    userStore.logout()
    MessagePlugin.success('已退出登录')
    router.replace('/login')
  } else if (value === 'profile') {
    router.push('/profile')
  } else if (value === 'billing') {
    router.push('/billing')
  }
}

// 监听路由变化，自动展开对应菜单
watch(() => route.path, (newPath) => {
  // 找到当前路径对应的一级菜单并展开
  const menus = menuStore.sidebarMenus.length > 0 
    ? menuStore.sidebarMenus 
    : fallbackMenus
  
  for (const menu of menus) {
    if (menu.children && menu.children.length > 0) {
      const hasMatchingChild = menu.children.some(
        (child) => '/' + (child.path || '') === newPath
      )
      if (hasMatchingChild) {
        expandedGroups.value.add(menu.path || menu.name)
        break
      }
    }
  }
})
</script>

<style scoped>
.user-layout {
  display: flex;
  min-height: 100vh;
  background: #F8FAFC;
}

/* ============ 侧边栏 ============ */
.sidebar {
  width: 240px;
  background: #fff;
  border-right: 1px solid #E2E8F0;
  display: flex;
  flex-direction: column;
  transition: width 0.3s;
  flex-shrink: 0;
}

.sidebar.is-collapsed {
  width: 72px;
}

.sidebar-header {
  padding: 20px 16px;
  border-bottom: 1px solid #F1F5F9;
}

.logo {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo-icon {
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #2563EB 0%, #3B82F6 100%);
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
  font-size: 18px;
  flex-shrink: 0;
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  color: #1E293B;
  white-space: nowrap;
}

.sidebar-nav {
  flex: 1;
  padding: 16px 12px;
  overflow-y: auto;
}

.nav-group {
  margin-bottom: 4px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-radius: 8px;
  cursor: pointer;
  color: #475569;
  text-decoration: none;
  font-size: 14px;
  transition: all 0.2s;
  white-space: nowrap;
}

.nav-item:hover {
  background: #F1F5F9;
  color: #1E293B;
}

.nav-item.is-active {
  background: linear-gradient(135deg, #2563EB 0%, #3B82F6 100%);
  color: #fff;
}

.nav-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.nav-label {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-arrow {
  font-size: 14px;
  transition: transform 0.2s;
}

.nav-arrow.is-rotated {
  transform: rotate(90deg);
}

.sub-menu {
  margin-top: 4px;
  margin-bottom: 4px;
  padding-left: 20px;
}

.sub-menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  color: #64748B;
  text-decoration: none;
  font-size: 13px;
  transition: all 0.2s;
}

.sub-menu-item:hover {
  background: #F1F5F9;
  color: #1E293B;
}

.sub-menu-item.is-active {
  background: #EFF6FF;
  color: #2563EB;
  font-weight: 500;
}

.sub-menu-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #CBD5E1;
  flex-shrink: 0;
  transition: all 0.2s;
}

.sub-menu-item:hover .sub-menu-dot,
.sub-menu-item.is-active .sub-menu-dot {
  background: #2563EB;
}

.sub-menu-label {
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 侧边栏底部 */
.sidebar-footer {
  padding: 16px;
  border-top: 1px solid #F1F5F9;
}

.user-mini-info {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  background: #F8FAFC;
  border-radius: 10px;
}

.user-avatar {
  background: linear-gradient(135deg, #2563EB 0%, #3B82F6 100%) !important;
  color: #fff !important;
  font-weight: 600;
  flex-shrink: 0;
}

.user-detail {
  flex: 1;
  overflow: hidden;
}

.user-name {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: #1E293B;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-role {
  display: block;
  font-size: 12px;
  color: #64748B;
}

/* ============ 主内容区 ============ */
.main-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

/* 顶部导航 */
.top-header {
  height: 60px;
  background: #fff;
  border-bottom: 1px solid #E2E8F0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 10;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.collapse-btn {
  font-size: 20px;
}

.breadcrumb {
  font-size: 14px;
}

.breadcrumb :deep(.t-breadcrumb-item) {
  color: #64748B;
}

.breadcrumb :deep(.t-breadcrumb-item--active) {
  color: #1E293B;
  font-weight: 500;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.user-dropdown-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px 4px 4px;
  border-radius: 24px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.user-dropdown-trigger:hover {
  background: #F1F5F9;
}

.header-avatar {
  background: linear-gradient(135deg, #2563EB 0%, #3B82F6 100%) !important;
  color: #fff !important;
  font-weight: 600;
  font-size: 14px;
}

.dropdown-caret {
  font-size: 14px;
  color: #94A3B8;
}

/* 内容区 */
.content-area {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* ============ 响应式 ============ */
.sidebar-overlay {
  display: none;
}

@media (max-width: 768px) {
  .sidebar {
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 100;
    transform: translateX(-100%);
    box-shadow: 4px 0 24px rgba(0, 0, 0, 0.1);
    width: 260px !important;
  }
  
  .sidebar:not(.is-collapsed) {
    transform: translateX(0);
  }

  .sidebar-overlay {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 90;
    backdrop-filter: blur(2px);
  }
  
  .content-area {
    padding: 16px;
  }
  
  .top-header {
    padding: 0 16px;
    height: 56px;
  }

  .breadcrumb {
    max-width: 160px;
    overflow: hidden;
  }

  .breadcrumb :deep(.t-breadcrumb-item) {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .user-dropdown-trigger:hover {
    background: transparent;
  }

  .header-right .t-tooltip {
    display: none;
  }
}

@media (max-width: 480px) {
  .header-right .header-avatar {
    width: 32px !important;
    height: 32px !important;
  }

  .user-dropdown-trigger {
    padding: 2px 8px 2px 2px;
  }

  .dropdown-caret {
    display: none;
  }
}

/* 滚动条样式 */
.sidebar-nav::-webkit-scrollbar {
  width: 4px;
}

.sidebar-nav::-webkit-scrollbar-track {
  background: transparent;
}

.sidebar-nav::-webkit-scrollbar-thumb {
  background: #CBD5E1;
  border-radius: 2px;
}

.sidebar-nav::-webkit-scrollbar-thumb:hover {
  background: #94A3B8;
}
</style>
