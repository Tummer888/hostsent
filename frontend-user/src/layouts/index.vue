<template>
  <div class="user-layout">
    <div class="main-wrapper">
      <!-- 顶部导航：汉堡 + logo + 一级菜单 | 搜索 + 文本链接 + 图标 + 用户名 -->
      <header class="top-header">
        <div class="header-left">
          <!-- 汉堡菜单（最左） -->
          <button class="hamburger-btn" aria-label="菜单" @click="toggleNav">
            <MenuUnfoldIcon v-if="!navOpen" size="20" />
            <MenuFoldIcon v-else size="20" />
          </button>

          <!-- logo -->
          <div class="logo" @click="router.push('/dashboard')">
            <span class="logo-icon">H</span>
            <span class="logo-text">宿派云控</span>
          </div>

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

          <!-- 文本导航 -->
          <nav class="header-links">
            <button class="header-link" @click="router.push('/billing')">费用</button>
            <button class="header-link" @click="router.push('/support')">客户支持</button>
            <button class="header-link" @click="router.push('/support')">备案</button>
            <button class="header-link" @click="router.push('/tickets')">渠道管理</button>
          </nav>

          <!-- 图标组 -->
          <div class="header-icons">
            <t-tooltip content="购物车" placement="bottom">
              <button class="icon-btn" aria-label="购物车" @click="router.push('/shop')">
                <CartIcon size="18" />
              </button>
            </t-tooltip>

            <t-tooltip content="消息通知" placement="bottom">
              <button class="icon-btn" aria-label="消息通知" @click="router.push('/messages')">
                <MailIcon size="18" />
                <span class="icon-btn__dot"></span>
              </button>
            </t-tooltip>

            <t-tooltip content="帮助文档" placement="bottom">
              <button class="icon-btn" aria-label="帮助文档" @click="router.push('/support')">
                <HelpCircleIcon size="18" />
              </button>
            </t-tooltip>

            <t-tooltip content="地区与语言" placement="bottom">
              <button class="icon-btn" aria-label="地区与语言" @click="onRegionClick">
                <EarthIcon size="18" />
              </button>
            </t-tooltip>

            <t-tooltip content="主题" placement="bottom">
              <button class="icon-btn" aria-label="主题" @click="onThemeClick">
                <ContrastIcon size="18" />
              </button>
            </t-tooltip>
          </div>

          <!-- 用户名 -->
          <t-dropdown trigger="click" @click="handleDropdownClick">
            <span class="header-username">{{ userName }}</span>
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

      <!-- 左侧展开的产品大菜单 -->
      <ProductMenu v-model:open="navOpen" />

      <!-- 内容区 -->
      <main class="content-area">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>

      <!-- 页脚 -->
      <footer class="site-footer">
        <!-- 服务保障条 -->
        <div class="footer-promise">
          <div v-for="p in footerPromises" :key="p.title" class="promise-item">
            <span class="promise-item__icon">
              <component :is="p.icon" size="26" />
            </span>
            <div class="promise-item__text">
              <strong>{{ p.title }}</strong>
              <span>{{ p.desc }}</span>
            </div>
          </div>
        </div>

        <div class="footer-main">
          <div class="footer-brand">
            <div class="footer-logo">
              <span class="footer-logo__icon">H</span>
              <span class="footer-logo__text">宿派云控 App</span>
            </div>

            <div class="footer-block">
              <h5 class="footer-col__title">售前咨询热线</h5>
              <p class="footer-hotline">400-800-1234</p>
              <button class="footer-link" @click="router.push('/support')">技术服务咨询</button>
              <button class="footer-link" @click="router.push('/support')">备案服务</button>
              <button class="footer-link" @click="router.push('/shop')">云商店咨询</button>
            </div>

            <div class="footer-block">
              <h5 class="footer-col__title">关注宿派云控</h5>
              <div class="footer-social">
                <button
                  v-for="s in footerSocial"
                  :key="s.label"
                  class="footer-social__icon"
                  :aria-label="s.label"
                  :title="s.label"
                  @click="onFooterSocial(s.label)"
                >
                  <component :is="s.icon" size="18" />
                </button>
                <button class="footer-social__app" @click="onFooterSocial('App')">
                  <MobileIcon size="14" />
                  App
                </button>
              </div>
            </div>
          </div>

          <div class="footer-links">
            <div v-for="col in footerColumns" :key="col.title" class="footer-col">
              <h5 class="footer-col__title">{{ col.title }}</h5>
              <button
                v-for="l in col.links"
                :key="l.label"
                class="footer-link"
                @click="router.push(l.path)"
              >
                {{ l.label }}
              </button>
            </div>
          </div>
        </div>

        <div class="footer-bottom">
          <div class="footer-bottom__legal">
            <p>© 2026 Hostsent.com 版权所有 苏ICP备00000000号-1 苏B2-20260000 苏B2-20260001</p>
            <p>增值电信业务经营许可证：B1-20260000 | 代理域名注册服务机构：示例机构</p>
          </div>
          <div class="footer-bottom__policy">
            <button class="footer-policy" @click="router.push('/support')">法律条文</button>
            <span class="footer-bottom__sep">|</span>
            <button class="footer-policy" @click="router.push('/support')">隐私政策</button>
          </div>
        </div>

        <div class="footer-bottom__badges">
          <span class="footer-badge-item"><CertificateIcon size="14" /> 电子营业执照</span>
          <span class="footer-badge-item"><SecuredIcon size="14" /> 公网安备 0000000000000号</span>
        </div>
      </footer>
    </div>

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
            <p>你好，我是宿派云控智能助手，可以帮你查资源、看账单、找文档。</p>
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
import { MessagePlugin } from 'tdesign-vue-next'
import {
  BookIcon,
  CartIcon,
  CertificateIcon,
  ChatBubbleIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CloseIcon,
  ContrastIcon,
  EarthIcon,
  Edit1Icon,
  HelpCircleIcon,
  LogoGithubIcon,
  LogoQqIcon,
  LogoWechatStrokeIcon,
  LogoYoutubeIcon,
  MailIcon,
  MenuFoldIcon,
  MenuUnfoldIcon,
  MobileIcon,
  PoweroffIcon,
  RollbackIcon,
  SearchIcon,
  SecuredIcon,
  ServiceIcon,
  TimeIcon,
  UserIcon,
  WalletIcon,
} from 'tdesign-icons-vue-next'

import { useMenuStore, useUserStore } from '@/store'
import ProductMenu from '@/components/product-menu/index.vue'

defineOptions({ name: 'UserLayout' })

const router = useRouter()
const route = useRoute()
const menuStore = useMenuStore()
const userStore = useUserStore()

// 左侧产品大菜单开合
const navOpen = ref(false)
const isMobile = ref(false)

function toggleNav() {
  navOpen.value = !navOpen.value
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

function onRegionClick() {
  MessagePlugin.info('地区与语言设置开发中')
}

function onThemeClick() {
  MessagePlugin.info('主题设置开发中')
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

// ========== 页脚 ==========
const footerPromises = [
  { title: '7×24', desc: '多渠道服务支持', icon: TimeIcon },
  { title: '备案', desc: '提供免费备案服务', icon: SecuredIcon },
  { title: '专业服务', desc: '云业务全流程支持', icon: ServiceIcon },
  { title: '退订', desc: '享无忧退订服务', icon: RollbackIcon },
  { title: '建议反馈', desc: '优化改进建议', icon: Edit1Icon },
]

const footerSocial = [
  { label: '微信', icon: LogoWechatStrokeIcon },
  { label: 'QQ', icon: LogoQqIcon },
  { label: '开源社区', icon: LogoGithubIcon },
  { label: '视频号', icon: LogoYoutubeIcon },
]

const footerColumns = [
  {
    title: '关于宿派云控',
    links: [
      { label: '了解宿派云控', path: '/support' },
      { label: '云计算概念', path: '/support' },
      { label: '客户案例', path: '/support' },
      { label: '信任中心', path: '/support' },
      { label: '新闻资讯', path: '/support' },
      { label: '视频中心', path: '/support' },
    ],
  },
  {
    title: '热门产品',
    links: [
      { label: '云主机 CVM', path: '/shop' },
      { label: '轻量云主机', path: '/shop' },
      { label: '对象存储', path: '/shop' },
      { label: '云数据库', path: '/shop' },
      { label: '私有网络', path: '/shop' },
      { label: '负载均衡', path: '/shop' },
    ],
  },
  {
    title: '支持与服务',
    links: [
      { label: '自助服务', path: '/support' },
      { label: '服务公告', path: '/support' },
      { label: '支持计划', path: '/support' },
      { label: '联系我们', path: '/support' },
      { label: '举报中心', path: '/support' },
    ],
  },
  {
    title: '实用工具',
    links: [
      { label: '价格计算器', path: '/shop' },
      { label: '云助手', path: '/profile' },
      { label: '服务健康看板', path: '/support' },
      { label: 'API 密钥', path: '/profile' },
    ],
  },
  {
    title: '友情链接',
    links: [
      { label: '宿派云官网', path: '/dashboard' },
      { label: '开发者联盟', path: '/support' },
      { label: '企业业务', path: '/shop' },
      { label: '云商城', path: '/shop' },
    ],
  },
]

function onFooterSocial(label: string) {
  MessagePlugin.info(`${label}开发中`)
}

// ========== 初始化 ==========
onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
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

// 路由切换后收起移动端抽屉
watch(() => route.path, () => {
  if (isMobile.value) navOpen.value = false
})
</script>

<style scoped>
.user-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  background: #F8FAFC;
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

.logo-text {
  font-size: 17px;
  font-weight: 700;
  color: #1E293B;
  white-space: nowrap;
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

/* ===== 用户名 ===== */
.header-username {
  font-size: 13.5px;
  color: #334155;
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 6px;
  white-space: nowrap;
  transition: color 0.15s ease, background-color 0.15s ease;
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
.dark .header-username {
  color: #cbd5e1;
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
  width: 100%;
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

/* ============ 页脚 ============ */
.site-footer {
  background: #ffffff;
  border-top: 1px solid #e8ecf2;
  color: #64748b;
  margin-top: 16px;
  padding: 0 32px;
}

/* 服务保障条 */
.footer-promise {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 24px;
  padding: 28px 0;
  border-bottom: 1px solid #eef1f5;
}

.promise-item {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.promise-item__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #1e293b;
  flex-shrink: 0;
}

.promise-item__text {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.promise-item__text strong {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.promise-item__text span {
  font-size: 12.5px;
  color: #8b95a8;
}

/* 主体 */
.footer-main {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 40px;
  padding: 32px 0;
}

.footer-brand {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding-right: 40px;
  border-right: 1px solid #eef1f5;
  min-width: 0;
}

.footer-logo {
  display: flex;
  align-items: center;
  gap: 10px;
}

.footer-logo__icon {
  width: 32px;
  height: 32px;
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
  font-size: 16px;
}

.footer-logo__text {
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}

.footer-block {
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.footer-hotline {
  margin: 0 0 4px;
  font-size: 22px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1.1;
}

.footer-social {
  display: flex;
  align-items: center;
  gap: 12px;
}

.footer-social__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  border: 1px solid #e2e8f0;
  background: #fff;
  color: #475569;
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.footer-social__icon:hover {
  color: var(--color-primary);
  border-color: #bfdbfe;
}

.footer-social__app {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 38px;
  padding: 0 14px;
  border-radius: 19px;
  border: 1px solid #e2e8f0;
  background: #fff;
  color: #475569;
  font-size: 12.5px;
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.footer-social__app:hover {
  color: var(--color-primary);
  border-color: #bfdbfe;
}

.footer-links {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 32px;
  min-width: 0;
}

.footer-col {
  display: flex;
  flex-direction: column;
  gap: 11px;
  min-width: 0;
}

.footer-col__title {
  margin: 0 0 3px;
  font-size: 13.5px;
  font-weight: 600;
  color: #1e293b;
}

.footer-link {
  border: none;
  background: transparent;
  text-align: left;
  padding: 0;
  font-size: 13px;
  color: #8b95a8;
  cursor: pointer;
  transition: color 0.15s ease;
}

.footer-link:hover {
  color: var(--color-primary);
}

/* 版权信息 */
.footer-bottom {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  padding: 20px 0 8px;
  border-top: 1px solid #eef1f5;
  font-size: 12.5px;
  color: #94a3b8;
}

.footer-bottom__legal {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.footer-bottom__legal p {
  margin: 0;
}

.footer-bottom__policy {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.footer-policy {
  border: none;
  background: transparent;
  padding: 0;
  font-size: 12.5px;
  color: #94a3b8;
  cursor: pointer;
  transition: color 0.15s ease;
}

.footer-policy:hover {
  color: var(--color-primary);
}

.footer-bottom__sep {
  color: #d5dbe5;
}

.footer-bottom__badges {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 20px;
  padding: 8px 0 24px;
}

.footer-badge-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: #94a3b8;
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

/* 深色模式：页脚 */
.dark .site-footer {
  background: #0a0a0a;
  border-top-color: #262626;
}

.dark .footer-promise,
.dark .footer-bottom {
  border-color: #262626;
}

.dark .footer-brand {
  border-color: #262626;
}

.dark .promise-item__icon,
.dark .promise-item__text strong,
.dark .footer-logo__text,
.dark .footer-hotline,
.dark .footer-col__title {
  color: #e5e7eb;
}

.dark .footer-social__icon,
.dark .footer-social__app {
  background: #141414;
  border-color: #2a2a2a;
  color: #cbd5e1;
}

/* ============ 响应式 ============ */
@media (max-width: 768px) {
  .top-header {
    padding: 0 12px;
    height: 56px;
  }

  /* 移动端：隐藏搜索框与文本导航，保留汉堡 + logo + 图标 + 用户名 */
  .header-search,
  .header-links {
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

  .site-footer {
    padding: 0 16px;
  }

  .footer-promise {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 18px 16px;
    padding: 22px 0;
  }

  .footer-main {
    grid-template-columns: 1fr;
    gap: 28px;
    padding: 24px 0;
  }

  .footer-brand {
    padding-right: 0;
    border-right: none;
    padding-bottom: 24px;
    border-bottom: 1px solid #eef1f5;
  }

  .footer-links {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 24px 16px;
  }

  .footer-bottom {
    flex-direction: column;
    gap: 14px;
  }

  .footer-bottom__badges {
    padding-bottom: 20px;
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
