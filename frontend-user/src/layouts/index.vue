<template>
  <div class="user-layout">
    <!-- 顶部导航：汉堡 + logo + 一级菜单 | 搜索 + 文本链接 + 图标 + 用户名 -->
    <header class="top-header">
      <div class="header-left">
        <!-- 汉堡菜单：桌面折叠侧边栏，移动端打开抽屉 -->
        <button
          class="hamburger-btn"
          aria-label="切换导航"
          @click="toggleNav"
        >
          <MenuUnfoldIcon size="20" />
        </button>

        <!-- logo：品牌名与图标来自管理端站点配置（brandStore） -->
        <div class="logo" @click="router.push('/dashboard')">
          <img v-if="brandStore.logo" class="logo-img" :src="brandStore.logo" :alt="brandStore.name" />
          <span v-else class="logo-icon">{{ brandStore.logoMark }}</span>
          <span class="logo-text">{{ brandStore.name }}</span>
        </div>

        <!-- 全部云产品：打开产品大菜单（产品导航，与左侧账户导航职责不同） -->
        <button class="all-products-btn" @click="productMenuOpen = true">
          <GridViewIcon size="16" />
          <span>全部云产品</span>
        </button>

      </div>

      <div class="header-right">
        <!-- 搜索框（图标在右） -->
        <div class="header-search">
          <input
            v-model="headerSearch"
            class="header-search__input"
            type="text"
            placeholder="搜索产品、资源或文档"
            @keyup.enter="onHeaderSearch"
          />
          <button class="header-search__btn" aria-label="搜索" @click="onHeaderSearch">
            <SearchIcon size="16" />
          </button>
        </div>

        <!-- 文本导航：指向各功能的真实落点，不再统一指向工单页 -->
        <nav class="header-links">
          <button class="header-link" @click="router.push('/billing')">费用</button>
          <button class="header-link" @click="router.push('/support/tickets')">客户支持</button>
          <button class="header-link" @click="onSiteEntry">官网</button>
          <button class="header-link" @click="router.push('/referral/materials')">渠道合作</button>
        </nav>

        <!-- 图标组 -->
        <div class="header-icons">
          <t-tooltip content="购物车" placement="bottom">
            <button class="icon-btn" aria-label="购物车" @click="router.push('/cart')">
              <CartIcon size="18" />
              <span v-if="cartCount > 0" class="icon-btn__badge">{{ cartCount > 99 ? '99+' : cartCount }}</span>
            </button>
          </t-tooltip>

          <t-tooltip content="我的消息" placement="bottom">
            <button class="icon-btn" aria-label="我的消息" @click="router.push('/profile/messages')">
              <MailIcon size="18" />
              <span v-if="unreadCount > 0" class="icon-btn__dot"></span>
            </button>
          </t-tooltip>

          <t-tooltip content="提交工单" placement="bottom">
            <button class="icon-btn" aria-label="提交工单" @click="router.push('/support/tickets')">
              <HelpCircleIcon size="18" />
            </button>
          </t-tooltip>

          <t-tooltip content="地区与语言" placement="bottom">
            <button class="icon-btn" aria-label="地区与语言" @click="onRegionClick">
              <EarthIcon size="18" />
            </button>
          </t-tooltip>

          <t-tooltip content="主题设置" placement="bottom">
            <button class="icon-btn" aria-label="主题设置" @click="settingsVisible = true">
              <ContrastIcon size="18" />
            </button>
          </t-tooltip>
        </div>

        <!-- 用户名：点开账户面板 -->
        <t-popup
          v-model:visible="userMenuVisible"
          trigger="click"
          placement="bottom-right"
          overlay-class-name="user-menu-popup"
        >
          <span class="header-username">
            {{ userName }}
            <span v-if="memberStore.isSub" class="header-username__sub">
              {{ memberStore.ownerName ? `${memberStore.ownerName} 的子账号` : '子账号' }}
            </span>
          </span>
          <template #content>
            <div class="user-menu">
              <div class="user-menu__head">
                <span class="user-menu__avatar">{{ userInitial }}</span>
                <div class="user-menu__meta">
                  <strong>{{ userName }}</strong>
                  <span>@{{ userStore.userInfo?.username || '-' }}</span>
                </div>
              </div>

              <div class="user-menu__body">
                <button
                  v-for="m in userMenuItems"
                  :key="m.key"
                  class="user-menu__item"
                  @click="handleUserMenuClick(m)"
                >
                  {{ m.title }}
                </button>
              </div>
              <div class="user-menu__foot">
                <button class="user-menu__logout" @click="handleLogout">退出登录</button>
              </div>
            </div>
          </template>
        </t-popup>
      </div>
    </header>

    <!-- 主体：左侧控制台导航 + 右侧内容区 -->
    <div class="body-wrapper">
      <SideNav ref="sideNavRef" v-model:open="sideNavOpen" />

      <main class="content-area">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>

    <!-- 左侧展开的产品大菜单 -->
    <ProductMenu v-model:open="productMenuOpen" />

    <!-- 主题设置抽屉 -->
    <SettingsPanel v-model:visible="settingsVisible" />

    <!-- 右侧漂浮浮窗 -->
    <div class="float-dock" :class="{ 'is-open': dockOpen }">
      <transition name="dock-pop">
        <div v-if="dockPanelOpen" class="dock-assistant">
          <div class="dock-assistant__head">
            <span class="dock-assistant__badge">
              <span class="dock-face dock-face--sm">
                <i class="dock-face__eye"></i>
                <i class="dock-face__eye"></i>
                <i class="dock-face__mouth"></i>
              </span>
            </span>
            <div class="dock-assistant__meta">
              <strong>智能助手</strong>
              <span>在线 · 随时为你解答</span>
            </div>
            <button class="dock-assistant__close" aria-label="关闭" @click="dockPanelOpen = false">
              <CloseIcon size="15" />
            </button>
          </div>
          <div class="dock-assistant__body">
            <p>你好，我是{{ brandStore.name }}智能助手，可以帮你查资源、看账单、找文档。</p>
            <div class="dock-assistant__quick">
              <button
                v-for="q in dockQuick"
                :key="q"
                class="dock-quick"
                @click="onDockQuick(q)"
              >
                {{ q }}
              </button>
            </div>
          </div>
        </div>
      </transition>

      <div class="dock-actions">
        <button
          class="dock-btn dock-btn--ai"
          :aria-label="dockPanelOpen ? '收起智能助手' : '打开智能助手'"
          title="智能助手"
          @click="toggleDockPanel"
        >
          <span class="dock-face">
            <i class="dock-face__eye"></i>
            <i class="dock-face__eye"></i>
            <i class="dock-face__mouth"></i>
          </span>
        </button>

        <button
          v-for="d in dockItems"
          :key="d.key"
          class="dock-btn dock-btn--action"
          :aria-label="d.label"
          :title="d.label"
          @click="onDockAction(d)"
        >
          <component :is="d.icon" size="19" />
        </button>

        <button
          class="dock-btn dock-btn--toggle"
          :aria-label="dockOpen ? '收起浮窗' : '展开浮窗'"
          @click="dockOpen = !dockOpen"
        >
          <ChevronRightIcon v-if="dockOpen" size="19" />
          <ChevronLeftIcon v-else size="19" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import {
  BookIcon,
  CartIcon,
  ChatBubbleIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CloseIcon,
  ContrastIcon,
  EarthIcon,
  GridViewIcon,
  HelpCircleIcon,
  MailIcon,
  MenuUnfoldIcon,
  SearchIcon,
  ServiceIcon,
} from 'tdesign-icons-vue-next'

import { useMenuStore, useUserStore, useSettingsStore, useCartStore, useBrandStore } from '@/store'
import { useMemberStore } from '@/store/modules/member'
import { getUnreadCount } from '@/api/notification'
import ProductMenu from '@/components/product-menu/index.vue'
import SideNav from '@/components/side-nav/index.vue'
import SettingsPanel from '@/components/settings-panel/index.vue'

defineOptions({ name: 'UserLayout' })

const router = useRouter()
const route = useRoute()
const menuStore = useMenuStore()
const userStore = useUserStore()
const memberStore = useMemberStore()
const settingsStore = useSettingsStore()
const cartStore = useCartStore()
const cartCount = computed(() => cartStore.count)
const brandStore = useBrandStore()

// 主题设置抽屉
const settingsVisible = ref(false)

// 两个导航是两件事，各用一个开关：
// - sideNavOpen：控制台侧边栏（桌面常驻/移动端抽屉），由汉堡按钮切换
// - productMenuOpen：产品大菜单浮层（全屏面板），由导航里的「全部云产品」入口打开
const sideNavOpen = ref(false)
const productMenuOpen = ref(false)
const isMobile = ref(false)
/** 桌面端汉堡要能折叠侧边栏，折叠状态由 SideNav 自己持有，这里拿它的开关。 */
const sideNavRef = ref<InstanceType<typeof SideNav> | null>(null)

/** 汉堡：窄屏开侧边抽屉，宽屏折叠/展开常驻侧边栏（折叠态持久化在 SideNav 内）。 */
function toggleNav() {
  if (isMobile.value) {
    sideNavOpen.value = !sideNavOpen.value
    return
  }
  sideNavRef.value?.toggleCollapse()
}

function checkMobile() {
  isMobile.value = window.innerWidth <= 768
}

// ========== 顶栏搜索 ==========
const headerSearch = ref('')

function onHeaderSearch() {
  const q = headerSearch.value.trim()
  if (!q) return
  headerSearch.value = ''
  router.push({ path: '/shop', query: { keyword: q } })
}

// ========== 用户信息 ==========
const userName = computed(() => userStore.displayName || '用户')
const userInitial = computed(() => (userStore.displayName || '用').slice(0, 1).toUpperCase())

function onRegionClick() {
  MessagePlugin.info('地区与语言设置开发中')
}

// ========== 官网门户入口 ==========
// user 是已登录控制台，官网是另一个独立站点（site，Nuxt SSR，默认 3003）。
// 未配置 VITE_SITE_URL 时只提示，不做跳转 —— 硬编码一个可能不存在的地址比不跳更糟。
const siteUrl = import.meta.env.VITE_SITE_URL || ''

function onSiteEntry() {
  if (!siteUrl) {
    MessagePlugin.info('官网地址未配置（VITE_SITE_URL）')
    return
  }
  window.open(siteUrl, '_blank', 'noopener')
}

// ========== 未读消息 / 购物车角标 ==========
const unreadCount = ref(0)

async function loadUnreadCount() {
  try {
    const { data } = await getUnreadCount()
    unreadCount.value = data?.count ?? 0
  } catch {
    // 角标是装饰性信息，取不到就不显示，不打断页面
  }
}

// 深色快捷切换（main.ts 已 init，这里只负责按钮行为）
function toggleTheme() {
  settingsStore.toggleDark()
}

// ========== 头像/昵称账户菜单 ==========
const userMenuVisible = ref(false)

interface UserMenuItem {
  key: string
  title: string
  to: string
  ownerOnly?: boolean
}

const userMenuItems = computed<UserMenuItem[]>(() => {
  const items: UserMenuItem[] = [
    { key: 'profile', title: '个人中心', to: '/profile' },
    { key: 'member', title: '成员管理', to: '/member', ownerOnly: true },
    { key: 'referral', title: '推广邀请', to: '/referral/overview' },
    { key: 'billing', title: '费用中心', to: '/billing' },
  ]
  // 子账号不展示「成员管理」（仅主账号可管理子账号）
  return items.filter((m) => !m.ownerOnly || memberStore.isOwner)
})

function closeUserMenu() {
  userMenuVisible.value = false
}

function handleUserMenuClick(item: UserMenuItem) {
  closeUserMenu()
  router.push(item.to)
}

function handleLogout() {
  const dialog = DialogPlugin.confirm({
    header: '退出登录',
    body: '确认退出当前账号？退出后需要重新登录。',
    confirmBtn: { content: '退出登录', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: () => {
      dialog.destroy()
      closeUserMenu()
      userStore.logout()
      MessagePlugin.success('已退出登录')
      router.replace('/login')
    },
    onClose: () => dialog.destroy(),
  })
}

// ========== 右侧漂浮浮窗 ==========
const dockOpen = ref(true)
const dockPanelOpen = ref(false)

const dockItems = [
  { key: 'service', label: '在线客服', icon: ServiceIcon },
  { key: 'chat', label: '在线咨询', icon: ChatBubbleIcon },
  { key: 'doc', label: '帮助文档', icon: BookIcon },
]

const dockQuick = ['云主机怎么选型？', '如何查看账单？', '提交工单']

function toggleDockPanel() {
  dockPanelOpen.value = !dockPanelOpen.value
  if (dockPanelOpen.value) dockOpen.value = true
}

function onDockAction(item: { key: string; label: string }) {
  if (item.key === 'doc') {
    router.push('/support')
    return
  }
  MessagePlugin.info(`${item.label}开发中`)
}

function onDockQuick(question: string) {
  MessagePlugin.info(`智能助手：${question}`)
}

// ========== 初始化 ==========
onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  settingsStore.init()
  loadUnreadCount()
  try {
    if (!menuStore.loaded) {
      await menuStore.loadMenus('user')
    }
  } catch (e) {
    console.warn('Using fallback menu:', e)
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', checkMobile)
})

// 路由切换后收起移动端抽屉
watch(() => route.path, () => {
  if (isMobile.value) sideNavOpen.value = false
  closeUserMenu()
})
</script>

<style scoped>
.user-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  background: #F8FAFC;
}

/* 主体：桌面端侧边栏常驻在左、内容在右；移动端侧边栏变抽屉（fixed），不占位。 */
.body-wrapper {
  flex: 1;
  display: flex;
  align-items: flex-start;
  min-width: 0;
}

.main-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

/* ============ 顶部导航 ============ */
.top-header {
  height: 64px;
  background: #fff;
  border-bottom: 1px solid #E2E8F0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 32px;
  position: sticky;
  top: 0;
  z-index: 20;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 28px;
  min-width: 0;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  flex-shrink: 0;
}

.logo-icon {
  width: 34px;
  height: 34px;
  background: linear-gradient(135deg, #2563EB 0%, #3B82F6 100%);
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
  font-size: 17px;
}

/* 后台配置了 Logo 图片时用它替换文字标记，高度与 .logo-icon 对齐 */
.logo-img {
  width: 34px;
  height: 34px;
  object-fit: contain;
  border-radius: 9px;
}

.logo-text {
  font-size: 17px;
  font-weight: 700;
  color: #1E293B;
  white-space: nowrap;
}

/* 「全部云产品」：唯一的产品大菜单入口（原本无入口，菜单状态恒为关闭） */
.all-products-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 12px;
  border: 1px solid #E2E8F0;
  border-radius: 8px;
  background: transparent;
  color: #334155;
  font-size: 13.5px;
  cursor: pointer;
  white-space: nowrap;
  transition: color 0.15s ease, border-color 0.15s ease, background-color 0.15s ease;
}

.all-products-btn:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
  background: #EFF6FF;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.hamburger-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: none;
  background: transparent;
  border-radius: 8px;
  color: #334155;
  cursor: pointer;
  flex-shrink: 0;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.hamburger-btn:hover {
  background: #F1F5F9;
  color: var(--color-primary);
}

/* ===== 顶栏搜索框（图标/按钮在右） ===== */
.header-search {
  position: relative;
  display: flex;
  align-items: center;
  width: 260px;
  max-width: 28vw;
}

.header-search__input {
  width: 100%;
  height: 36px;
  padding: 0 40px 0 14px;
  border: 1px solid #E2E8F0;
  border-radius: 6px;
  background: #ffffff;
  color: #1E293B;
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.header-search__input::placeholder {
  color: #94A3B8;
}

.header-search__input:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.10);
}

.header-search__btn {
  position: absolute;
  right: 1px;
  top: 50%;
  transform: translateY(-50%);
  width: 36px;
  height: 34px;
  border: none;
  background: transparent;
  color: #94A3B8;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0 6px 6px 0;
}

.header-search__btn:hover {
  color: var(--color-primary);
}

/* ===== 顶栏文本导航 ===== */
.header-links {
  display: flex;
  align-items: center;
  gap: 2px;
}

.header-link {
  border: none;
  background: transparent;
  padding: 6px 10px;
  border-radius: 6px;
  font-size: 13.5px;
  color: #475569;
  cursor: pointer;
  white-space: nowrap;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.header-link:hover {
  color: var(--color-primary);
  background: #EFF6FF;
}

/* ===== 顶栏图标组 ===== */
.header-icons {
  display: flex;
  align-items: center;
  gap: 2px;
}

.icon-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: none;
  background: transparent;
  border-radius: 8px;
  color: #475569;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.icon-btn:hover {
  background: #F1F5F9;
  color: var(--color-primary);
}

.icon-btn__dot {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #EF4444;
  border: 1.5px solid #fff;
}

/* 购物车数量角标：比未读小红点多一个数字，样式单独给（位置更靠外） */
.icon-btn__badge {
  position: absolute;
  top: 2px;
  right: 1px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 999px;
  background: #EF4444;
  border: 1.5px solid #fff;
  color: #fff;
  font-size: 10px;
  line-height: 13px;
  text-align: center;
  font-weight: 600;
}

/* ===== 用户名 ===== */
.header-username {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13.5px;
  color: #334155;
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 6px;
  white-space: nowrap;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.header-username__sub {
  padding: 1px 6px;
  border-radius: 999px;
  background: #EFF6FF;
  color: var(--color-primary, #2563eb);
  font-size: 11px;
  line-height: 16px;
}

/* ===== 头像/昵称账户菜单 ===== */
.user-menu {
  width: 220px;
  background: #fff;
  border-radius: 10px;
  box-shadow: 0 12px 40px rgba(15, 23, 42, 0.14);
  overflow: hidden;
}

.user-menu__head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px;
  background: linear-gradient(135deg, #eff6ff 0%, #f8fafc 100%);
  border-bottom: 1px solid #eef2f7;
}

.user-menu__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  color: #fff;
  font-size: 15px;
  font-weight: 700;
  flex-shrink: 0;
}

.user-menu__meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.user-menu__meta strong {
  font-size: 14.5px;
  color: #1e293b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-menu__meta span {
  font-size: 12px;
  color: #8b95a8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-menu__body {
  display: flex;
  flex-direction: column;
  padding: 8px;
}

.user-menu__item {
  width: 100%;
  padding: 9px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #334155;
  font-size: 13.5px;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.user-menu__item:hover {
  background: #f1f5f9;
  color: var(--color-primary, #2563eb);
}

.user-menu__foot {
  padding: 6px;
  border-top: 1px solid #eef2f7;
}

.user-menu__logout {
  width: 100%;
  padding: 9px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #ef4444;
  font-size: 13.5px;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease;
}

.user-menu__logout:hover {
  background: #fef2f2;
}

.dark .user-menu {
  background: #141414;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
}

.dark .user-menu__head {
  background: linear-gradient(135deg, #10192e 0%, #0f1420 100%);
  border-bottom-color: #262626;
}

.dark .user-menu__meta strong {
  color: #e5e7eb;
}

.dark .user-menu__meta span {
  color: #9ca3af;
}

.dark .user-menu__item {
  color: #cbd5e1;
}

.dark .user-menu__item:hover {
  background: #1f1f1f;
  color: #93c5fd;
}

.dark .user-menu__foot {
  border-top-color: #262626;
}

.dark .user-menu__logout:hover {
  background: #2a1212;
}

.dark .header-username__sub {
  background: #1e293b;
  color: #93c5fd;
}

.header-username:hover {
  color: var(--color-primary);
  background: #EFF6FF;
}

.dark .header-search__input {
  background: #111111;
  border-color: #262626;
  color: #e5e7eb;
}

.dark .hamburger-btn,
.dark .icon-btn,
.dark .header-link,
.dark .header-username,
.dark .all-products-btn {
  color: #cbd5e1;
}

.dark .all-products-btn {
  border-color: #2a2a2a;
}

.dark .all-products-btn:hover {
  background: #1e1e1e;
  border-color: var(--td-brand-color-4);
  color: var(--td-brand-color-4);
}

.dark .icon-btn__badge {
  border-color: #141414;
}

.dark .icon-btn:hover,
.dark .header-link:hover,
.dark .header-username:hover,
.dark .hamburger-btn:hover {
  background: #1e1e1e;
  color: var(--td-brand-color-4);
}

/* ============ 内容区 ============ */
.content-area {
  flex: 1;
  min-width: 0;
  padding: 24px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* ============ 右侧漂浮浮窗 ============ */
.float-dock {
  position: fixed;
  right: 20px;
  bottom: 72px;
  z-index: 30;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.dock-actions {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.dock-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  padding: 0;
  border: 1px solid #e8eef7;
  border-radius: 50%;
  background: #ffffff;
  color: #475569;
  cursor: pointer;
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.1);
  transition: color 0.2s ease, border-color 0.2s ease, transform 0.2s ease,
    box-shadow 0.2s ease, opacity 0.25s ease;
}

.dock-btn:hover {
  color: var(--color-primary);
  border-color: #bfdbfe;
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(37, 99, 235, 0.18);
}

.dock-btn--ai {
  width: 48px;
  height: 48px;
  border: none;
  background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 55%, #4f46e5 100%);
  box-shadow: 0 8px 22px rgba(99, 102, 241, 0.35);
}

.dock-btn--ai:hover {
  transform: translateY(-2px) scale(1.03);
  box-shadow: 0 10px 26px rgba(99, 102, 241, 0.45);
}

.dock-btn--toggle {
  color: #64748b;
}

/* 浮窗折叠：仅保留智能助手与展开按钮 */
.float-dock:not(.is-open) .dock-btn--action {
  opacity: 0;
  transform: translateX(14px) scale(0.9);
  max-height: 0;
  height: 0;
  border-width: 0;
  box-shadow: none;
  pointer-events: none;
  overflow: hidden;
}

/* 卡通表情 */
.dock-face {
  position: relative;
  display: block;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.92);
}

.dock-face--sm {
  width: 22px;
  height: 22px;
}

.dock-face__eye {
  position: absolute;
  top: 34%;
  width: 4px;
  height: 6px;
  border-radius: 50%;
  background: #6d28d9;
}

.dock-face__eye:first-child {
  left: 27%;
}

.dock-face__eye:nth-child(2) {
  right: 27%;
}

.dock-face__mouth {
  position: absolute;
  left: 50%;
  bottom: 24%;
  width: 10px;
  height: 5px;
  transform: translateX(-50%);
  border-bottom: 2px solid #6d28d9;
  border-radius: 0 0 8px 8px;
}

/* 智能助手面板 */
.dock-assistant {
  width: 260px;
  border-radius: 14px;
  background: #ffffff;
  border: 1px solid #e8eef7;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.16);
  overflow: hidden;
}

.dock-assistant__head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px;
  background: linear-gradient(135deg, #f5f3ff 0%, #eef2ff 100%);
}

.dock-assistant__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: linear-gradient(135deg, #8b5cf6 0%, #4f46e5 100%);
  flex-shrink: 0;
}

.dock-assistant__meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.dock-assistant__meta strong {
  font-size: 13.5px;
  color: #1e293b;
}

.dock-assistant__meta span {
  font-size: 11.5px;
  color: #8b95a8;
}

.dock-assistant__close {
  border: none;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  padding: 2px;
  border-radius: 6px;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.dock-assistant__close:hover {
  color: #475569;
  background: rgba(148, 163, 184, 0.16);
}

.dock-assistant__body {
  padding: 14px;
}

.dock-assistant__body p {
  margin: 0 0 12px;
  font-size: 12.5px;
  line-height: 1.7;
  color: #475569;
}

.dock-assistant__quick {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.dock-quick {
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  color: #475569;
  font-size: 12px;
  padding: 5px 10px;
  border-radius: 14px;
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease, background-color 0.15s ease;
}

.dock-quick:hover {
  color: var(--color-primary);
  border-color: #bfdbfe;
  background: #eff6ff;
}

.dock-pop-enter-active,
.dock-pop-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.dock-pop-enter-from,
.dock-pop-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.96);
}

/* 深色模式 */
.dark .dock-btn {
  background: #141414;
  border-color: #2a2a2a;
  color: #cbd5e1;
}

.dark .dock-assistant {
  background: #141414;
  border-color: #2a2a2a;
}

.dark .dock-assistant__head {
  background: linear-gradient(135deg, #1e1b3a 0%, #171c33 100%);
}

.dark .dock-assistant__meta strong {
  color: #e5e7eb;
}

.dark .dock-assistant__body p {
  color: #cbd5e1;
}

.dark .dock-quick {
  background: #0f1424;
  border-color: #2a3350;
  color: #cbd5e1;
}

/* ============ 响应式 ============ */
@media (max-width: 768px) {
  .top-header {
    padding: 0 12px;
    height: 56px;
  }

  /* 移动端：隐藏搜索框、文本导航与产品大菜单入口，保留汉堡 + logo + 图标 + 用户名 */
  .header-search,
  .header-links,
  .all-products-btn {
    display: none;
  }

  .top-header {
    padding: 0 12px;
  }

  .logo-text {
    display: none;
  }

  .content-area {
    padding: 16px;
  }

  .float-dock {
    right: 12px;
    bottom: 88px;
    gap: 10px;
  }

  .dock-actions {
    gap: 10px;
  }

  .dock-btn {
    width: 38px;
    height: 38px;
  }

  .dock-btn--ai {
    width: 44px;
    height: 44px;
  }

  .dock-assistant {
    width: min(260px, calc(100vw - 44px));
  }
}

@media (max-width: 480px) {
  .header-username {
    max-width: 72px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
</style>
