<template>
  <div class="console-page">
    <!-- ============ 左主区 ============ -->
    <div class="console-main">
      <!-- 上层：欢迎卡（问候 + 搜索 + 真实日期） -->
      <section class="welcome-hero">
        <div class="hero-card__text">
          <h2 class="hero-title">
            {{ greeting }}，<span class="hero-title__name">{{ displayName }}</span>
          </h2>
          <p class="hero-sub">{{ today }} · 欢迎回到{{ brandStore.name }}控制台</p>
          <div class="hero-search">
            <SearchIcon size="16" class="hero-search__icon" />
            <input
              v-model="heroKeyword"
              class="hero-search__input"
              type="text"
              placeholder="请输入关键词，如云主机"
              @keyup.enter="onHeroSearch"
            />
          </div>
        </div>
        <div class="hero-card__art">
          <div class="hero-stat">
            <span class="hero-stat__value">{{ resourceStats.instance }}</span>
            <span class="hero-stat__label">在管云主机</span>
          </div>
          <div class="hero-stat">
            <span class="hero-stat__value">{{ resourceStats.renewal }}</span>
            <span class="hero-stat__label">待处理续费</span>
          </div>
        </div>
      </section>

      <!-- 下层：最近访问 + 快捷入口 -->
      <section class="welcome-panel">
        <div class="welcome-block">
          <h4 class="welcome-block__title">最近访问</h4>
          <div v-if="recentPages.length" class="recent-row">
            <button
              v-for="r in recentPages"
              :key="r.path"
              class="recent-chip"
              @click="go(r.path)"
            >
              {{ r.title }}
            </button>
          </div>
          <p v-else class="welcome-empty">
            还没有访问记录，去
            <button class="link-btn" @click="go('/shop')">云主机选购</button>
            或
            <button class="link-btn" @click="go('/cloud/instances')">我的云主机</button>
            看看。
          </p>
        </div>

        <div class="welcome-block">
          <h4 class="welcome-block__title">快捷入口</h4>
          <div class="entry-grid">
            <button
              v-for="e in quickEntries"
              :key="e.path"
              class="entry-chip"
              @click="go(e.path)"
            >
              <component :is="e.icon" size="14" class="entry-chip__icon" />
              {{ e.title }}
            </button>
          </div>
        </div>
      </section>

      <!-- 资源概览 -->
      <section class="panel">
        <header class="panel__head">
          <h3 class="panel__title">我的资源</h3>
          <span class="panel__hint">截至 {{ refreshedText }}</span>
        </header>
        <div class="res-grid">
          <div
            v-for="item in resources"
            :key="item.key"
            class="res-item"
            @click="go(item.path)"
          >
            <span class="res-item__icon"><component :is="item.icon" size="20" /></span>
            <div class="res-item__body">
              <span class="res-item__value">{{ item.value }}</span>
              <span class="res-item__label">{{ item.label }}</span>
            </div>
          </div>
        </div>
      </section>

      <!-- 服务推荐（后台在售商品中标记「推荐」的） -->
      <section v-if="promoList.length" class="panel">
        <header class="panel__head">
          <h3 class="panel__title">服务推荐</h3>
          <button class="panel__more" @click="go('/shop')">
            全部产品 <ChevronRightIcon size="14" />
          </button>
        </header>
        <div v-if="promoList.length > 1" class="promo-tabs">
          <button
            v-for="p in promoList"
            :key="p.id"
            class="promo-tab"
            :class="{ 'is-active': activePromoId === p.id }"
            @click="activePromoId = p.id"
          >
            {{ p.name }}
          </button>
        </div>
        <div v-if="activePromoItem" class="promo">
          <div class="promo__body">
            <h4 class="promo__title">{{ activePromoItem.name }}</h4>
            <p class="promo__desc">{{ activePromoItem.description || promoFallbackDesc }}</p>
            <div class="promo__tags">
              <span class="promo__tag">{{ priceLabel(activePromoItem) }}</span>
              <span v-if="activePromoItem.skus?.length" class="promo__tag">
                {{ activePromoItem.skus.length }} 个规格可选
              </span>
              <span v-else class="promo__tag">标准规格</span>
            </div>
            <t-button theme="primary" size="small" @click="goShopProduct(activePromoItem.id)">
              立即选购
            </t-button>
          </div>
        </div>
      </section>

      <!-- 帮助与支持（内容由官网门户承载） -->
      <section class="learn-section">
        <h3 class="learn-section__title">帮助与支持</h3>
        <div class="learn-card">
          <div class="learn-col">
            <h4 class="learn-col__title">文档与公告</h4>
            <p class="learn-col__desc">
              产品文档、新闻资讯与平台公告在官网门户统一维护，控制台只放入口。
            </p>
            <div class="learn-grid">
              <button
                v-for="l in docLinks"
                :key="l.label"
                class="learn-item"
                @click="onDocClick(l)"
              >
                <span class="learn-item__left">
                  <component :is="l.icon" size="16" class="learn-item__icon" />
                  <span>{{ l.label }}</span>
                </span>
                <JumpIcon size="15" class="learn-item__arrow" />
              </button>
            </div>
          </div>

          <div class="learn-col">
            <h4 class="learn-col__title">自助服务</h4>
            <p class="learn-col__desc">控制台内的常用入口，提交与跟踪问题都在这。</p>
            <div class="learn-grid">
              <button
                v-for="l in devLinks"
                :key="l.label"
                class="learn-item"
                @click="go(l.path)"
              >
                <span class="learn-item__left">
                  <component :is="l.icon" size="16" class="learn-item__icon" />
                  <span>{{ l.label }}</span>
                </span>
                <JumpIcon size="15" class="learn-item__arrow" />
              </button>
            </div>
          </div>
        </div>
      </section>
    </div>

    <!-- ============ 右侧信息栏 ============ -->
    <aside class="console-side">
      <!-- 账户卡 -->
      <section class="side-card account-card">
        <div class="account-card__head">
          <t-avatar size="46" class="account-card__avatar">{{ avatarText }}</t-avatar>
          <div class="account-card__id">
            <div class="account-card__name-row">
              <span class="account-card__name">{{ displayName }}</span>
              <span class="account-card__badge">{{ memberStore.isSub ? '子账号' : '主账号' }}</span>
            </div>
            <div class="account-card__verify">
              <span class="verify-item" :class="realnameOk ? 'is-ok' : 'is-off'">
                <component :is="realnameOk ? CheckCircleIcon : CloseCircleIcon" size="13" />
                实名{{ realnameOk ? '已认证' : '未认证' }}
              </span>
              <span class="verify-item" :class="bounds.phone ? 'is-ok' : 'is-off'">
                <component :is="bounds.phone ? CheckCircleIcon : CloseCircleIcon" size="13" />
                手机{{ bounds.phone ? '已绑定' : '未绑定' }}
              </span>
            </div>
            <span class="account-card__sub">账号 ID：{{ accountId }}</span>
          </div>
        </div>

        <!-- 未绑定邮箱提醒（点击进入账户设置，真实可写） -->
        <div v-if="!bounds.email" class="account-warning">
          <span class="account-warning__left">
            <InfoCircleFilledIcon size="14" />
            未绑定邮箱，将无法接收账单与工单通知
          </span>
          <button class="account-warning__action" @click="go('/profile')">立即绑定</button>
        </div>
        <div class="account-card__stats">
          <div v-for="m in miniStats" :key="m.label" class="mini-stat" @click="go(m.path)">
            <span class="mini-stat__value">{{ m.value }}</span>
            <span class="mini-stat__label">{{ m.label }}</span>
          </div>
        </div>
      </section>

      <!-- 费用信息 -->
      <section class="side-card">
        <header class="side-card__head">
          <h4 class="side-card__title">费用信息</h4>
          <button class="panel__more" @click="go('/billing')">费用中心 <ChevronRightIcon size="13" /></button>
        </header>
        <div class="fee-head">
          <div class="fee-head__info">
            <span class="fee-label">账户余额（元）</span>
            <span class="fee-head__value">{{ money(balances.balance) }}</span>
          </div>
          <t-button
            v-if="memberStore.has('billing:recharge')"
            theme="primary"
            size="small"
            @click="go('/billing/balance')"
          >
            充值
          </t-button>
        </div>
        <div class="fee-tiles">
          <div class="fee-tile">
            <span class="fee-label">待支付订单</span>
            <span class="fee-tile__value">{{ resourceStats.order }} 笔</span>
          </div>
          <div class="fee-tile">
            <span class="fee-label">可开票金额</span>
            <span class="fee-tile__value">{{ money(balances.invoiceable) }}</span>
          </div>
        </div>
      </section>

      <!-- 账户安全（真实取自安全设置接口） -->
      <section class="side-card">
        <header class="side-card__head">
          <h4 class="side-card__title">账户安全</h4>
          <button class="panel__more" @click="go('/profile/security')">
            安全设置 <ChevronRightIcon size="13" />
          </button>
        </header>
        <ul class="sec-list">
          <li v-for="s in securityItems" :key="s.key" class="sec-list__item">
            <component :is="s.icon" size="15" class="sec-list__icon" />
            <span class="sec-list__label">{{ s.label }}</span>
            <span class="sec-list__state" :class="s.ok ? 'is-ok' : 'is-warn'">{{ s.state }}</span>
          </li>
        </ul>
        <p class="sec-note">
          有 {{ forcedSceneCount }} 项关键操作开启了平台强制的二次验证，无法在用户侧关闭。
        </p>
      </section>

      <!-- 成员与协作 -->
      <section v-if="memberStore.isOwner" class="side-card">
        <header class="side-card__head">
          <h4 class="side-card__title">成员与协作</h4>
          <button class="panel__more" @click="go('/member')">成员管理 <ChevronRightIcon size="13" /></button>
        </header>
        <div class="access-url">
          <span class="access-url__label">子账号登录入口</span>
          <span class="access-url__value">{{ loginUrl }}</span>
          <button class="access-url__copy" @click="copyLoginUrl">复制</button>
        </div>
        <div class="access-stats">
          <div class="access-stat">
            <span class="access-stat__label">成员数</span>
            <span class="access-stat__value">{{ memberCount }} / {{ memberQuota || '—' }}</span>
          </div>
          <div class="access-stat">
            <span class="access-stat__label">可用权限项</span>
            <span class="access-stat__value">{{ permissionOptionCount }}</span>
          </div>
        </div>
      </section>

      <!-- 最新公告 -->
      <section class="side-card">
        <header class="side-card__head">
          <h4 class="side-card__title">最新公告</h4>
          <button class="panel__more" @click="go('/profile/messages')">
            更多 <ChevronRightIcon size="13" />
          </button>
        </header>
        <ul v-if="announcements.length" class="announce-list">
          <li
            v-for="a in announcements"
            :key="a.id"
            class="announce-item"
            @click="go('/profile/messages')"
          >
            <div class="announce-item__row">
              <span class="announce-item__tag" :class="`is-${a.level || 'info'}`">
                {{ a.pinned ? '置顶' : levelLabel(a.level) }}
              </span>
              <span class="announce-item__title">{{ a.title }}</span>
            </div>
            <span class="announce-item__time">{{ formatTime(a.publish_at) }}</span>
          </li>
        </ul>
        <p v-else class="welcome-empty">暂无公告</p>
      </section>

      <!-- 常用工具 -->
      <section class="side-card">
        <header class="side-card__head">
          <h4 class="side-card__title">常用工具</h4>
        </header>
        <div class="tool-grid">
          <button
            v-for="t in tools"
            :key="t.label"
            class="tool-item"
            @click="onToolClick(t)"
          >
            {{ t.label }}
          </button>
        </div>
      </section>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  BookOpenIcon,
  BrowseIcon,
  CartIcon,
  CheckCircleIcon,
  ChevronRightIcon,
  CloseCircleIcon,
  FileIcon,
  FingerprintIcon,
  InfoCircleFilledIcon,
  JumpIcon,
  MailIcon,
  MobileIcon,
  OrderIcon,
  RefreshIcon,
  SearchIcon,
  SecuredIcon,
  ServerIcon,
  ServiceIcon,
  ToolsIcon,
  WalletIcon,
} from 'tdesign-icons-vue-next'

import { useUserStore } from '@/store'
import { useMemberStore } from '@/store/modules/member'
import { useBrandStore } from '@/store/modules/brand'
import { listInstances } from '@/api/cloud'
import { getMyOrders, getProducts, type ProductInfo } from '@/api/shop'
import { getRenewalsView } from '@/api/lifecycle'
import { getMyTickets, getTicketCategories } from '@/api/support'
import { getBalance, getMyBills, type WalletInfo } from '@/api/finance'
import { getMyAnnouncements, type AnnouncementInfo } from '@/api/notification'
import { getMemberList } from '@/api/member'
import { getSecuritySettings } from '@/api/security'
import { formatTime } from '@/pages/support/constants'
import { getRecentPages, type RecentPage } from '@/utils/recent'
import { openSite, siteUrlConfigured } from '@/utils/site'

defineOptions({ name: 'UserConsole' })

const router = useRouter()
const userStore = useUserStore()
const memberStore = useMemberStore()
const brandStore = useBrandStore()

function go(path: string) {
  router.push(path)
}

function money(value: number | undefined): string {
  const n = Number(value ?? 0)
  return `¥ ${n.toFixed(2)}`
}

const displayName = computed(() => userStore.displayName || '用户')
const avatarText = computed(() => displayName.value.slice(0, 1).toUpperCase())
// 账号 ID 直接取真实用户 ID：此前用 padStart(12,'0') 把 2 补成「000000000002」，
// 与个人中心显示的 ID 不一致，看起来像两个账号。
const accountId = computed(() => String(userStore.userInfo?.id ?? '-'))

const today = new Date().toLocaleDateString('zh-CN', {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  weekday: 'long',
})

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

// ========== 资源统计（真实接口） ==========
const resourceStats = ref({ instance: 0, renewal: 0, order: 0, ticket: 0 })
const balances = ref<{ balance: number; invoiceable: number }>({ balance: 0, invoiceable: 0 })
const refreshedText = ref('—')

/** 待续费口径与续费管理页一致：已到期、宽限期、暂停，或 30 天内到期。 */
function countRenewals(items: Array<{ stage?: string; days_left?: number }>): number {
  return items.filter((i) => {
    if (i.stage && i.stage !== 'active') return true
    return typeof i.days_left === 'number' && i.days_left <= 30
  }).length
}

async function loadOverview() {
  // 每个数字独立失败：单接口异常只让对应卡片保持 0，不拖垮整页
  const [instances, renewals, orders, tickets] = await Promise.allSettled([
    listInstances(),
    getRenewalsView(),
    getMyOrders({ page: 1, page_size: 1 }),
    getMyTickets({ page: 1, page_size: 1 }),
  ])
  resourceStats.value = {
    instance: instances.status === 'fulfilled' ? (instances.value.data?.items?.length ?? 0) : 0,
    renewal: renewals.status === 'fulfilled' ? countRenewals(renewals.value.data?.items ?? []) : 0,
    order: orders.status === 'fulfilled' ? (orders.value.data?.total ?? 0) : 0,
    ticket: tickets.status === 'fulfilled' ? (tickets.value.data?.meta?.total ?? 0) : 0,
  }
  refreshedText.value = formatTime(new Date().toISOString())
}

const resources = computed(() => [
  { key: 'instance', label: '云主机', value: resourceStats.value.instance, icon: ServerIcon, path: '/cloud/instances' },
  { key: 'renewal', label: '待续费', value: resourceStats.value.renewal, icon: RefreshIcon, path: '/cloud/renewals' },
  { key: 'order', label: '订单', value: resourceStats.value.order, icon: CartIcon, path: '/order' },
  { key: 'ticket', label: '工单', value: resourceStats.value.ticket, icon: ServiceIcon, path: '/support/tickets' },
])

const miniStats = computed(() => [
  { label: '待支付订单', value: pendingOrderCount.value, path: '/order?status=pending' },
  { label: '待续费', value: resourceStats.value.renewal, path: '/cloud/renewals' },
  { label: '我的工单', value: resourceStats.value.ticket, path: '/support/tickets' },
])

// ========== 待支付订单数（迷你统计用真实状态过滤） ==========
const pendingOrderCount = ref(0)

async function loadPendingOrders() {
  try {
    const { data } = await getMyOrders({ status: 'pending', page: 1, page_size: 1 })
    pendingOrderCount.value = data?.total ?? 0
  } catch {
    pendingOrderCount.value = 0
  }
}

// ========== 费用：余额 + 可开票金额 ==========
async function loadBalances() {
  const [wallet, bills] = await Promise.allSettled([
    getBalance(),
    // 可开票口径与后端开票规则一致：账单已结清且未开票（status=paid + invoice_status=none）
    getMyBills({ status: 'paid', page: 1, page_size: 100 }),
  ])
  const balance = wallet.status === 'fulfilled' ? (wallet.value.data as WalletInfo | undefined)?.balance : 0
  const invoiceable =
    bills.status === 'fulfilled'
      ? (bills.value.data?.items ?? [])
          .filter((b) => b.invoice_status !== 'issued')
          .reduce((sum, b) => sum + Number(b.total_amount || 0), 0)
      : 0
  balances.value = { balance: Number(balance || 0), invoiceable }
}

// ========== 推荐商品（后台标记 featured 的在售商品） ==========
const promoList = ref<ProductInfo[]>([])
const activePromoId = ref<number | null>(null)
const activePromoItem = computed(
  () => promoList.value.find((p) => p.id === activePromoId.value) ?? promoList.value[0],
)
const promoFallbackDesc = computed(
  () => `${brandStore.name}在售云主机产品，支持在线购买、按周期计费与自助续费。`,
)

async function loadPromotions() {
  try {
    const { data } = await getProducts({ featured: true, page: 1, page_size: 6 })
    promoList.value = data?.items ?? []
    activePromoId.value = promoList.value[0]?.id ?? null
  } catch {
    promoList.value = []
  }
}

function priceLabel(p: ProductInfo): string {
  const unit: Record<string, string> = { monthly: '元/月', quarterly: '元/季', annually: '元/年', onetime: '元' }
  return `${p.price} ${unit[p.price_model] || '元'}`
}

/** 带商品 ID 跳选购页，由 shop 页的「官网跳转意图」逻辑直接拉起下单面板。 */
function goShopProduct(id: number) {
  router.push({ path: '/shop', query: { product: String(id) } })
}

// ========== 账户绑定与安全（真实接口） ==========
const bounds = ref({ phone: false, email: false })
const securityItems = ref<Array<{ key: string; label: string; icon: Component; ok: boolean; state: string }>>([])
const forcedSceneCount = ref(0)

async function loadSecurity() {
  try {
    const { data } = await getSecuritySettings()
    if (!data) return
    bounds.value = { phone: data.phone_bound, email: data.email_bound }
    forcedSceneCount.value = (data.scenes || []).filter((s) => s.platform_forced && s.otp_required).length
    securityItems.value = [
      {
        key: 'phone',
        label: '绑定手机',
        icon: MobileIcon,
        ok: data.phone_bound,
        state: data.phone_bound ? data.phone_masked || '已绑定' : '未绑定',
      },
      {
        key: 'email',
        label: '绑定邮箱',
        icon: MailIcon,
        ok: data.email_bound,
        state: data.email_bound ? data.email_masked || '已绑定' : '未绑定',
      },
      {
        key: 'mfa',
        label: '虚拟 MFA',
        icon: FingerprintIcon,
        ok: data.mfa_enabled,
        state: data.mfa_enabled ? '已开启' : '未开启',
      },
      {
        key: 'protect',
        label: '关键操作保护',
        icon: SecuredIcon,
        ok: forcedSceneCount.value > 0,
        state: forcedSceneCount.value > 0 ? `${forcedSceneCount.value} 项已启用` : '未启用',
      },
    ]
  } catch {
    // 安全设置拿不到时不渲染假状态：保留空列表，卡片自身显示提示
    securityItems.value = []
  }
}

// ========== 成员与协作（真实接口，仅主账号） ==========
const memberCount = ref(0)
const memberQuota = ref(0)
const permissionOptionCount = ref(0)
const loginUrl = computed(() => `${window.location.origin}/login`)

async function loadMembers() {
  if (!memberStore.isOwner) return
  try {
    const { data } = await getMemberList({ page: 1, page_size: 1 })
    memberCount.value = data?.meta?.total ?? 0
    memberQuota.value = data?.max_sub_accounts ?? 0
    permissionOptionCount.value = data?.permission_options?.length ?? 0
  } catch {
    memberCount.value = 0
  }
}

async function copyLoginUrl() {
  try {
    await navigator.clipboard.writeText(loginUrl.value)
    MessagePlugin.success('登录入口已复制')
  } catch {
    MessagePlugin.warning('复制失败，请手动选择内容')
  }
}

// ========== 实名状态（复用工单分类接口返回的账号实名标记） ==========
const realnameOk = ref(false)

async function loadRealname() {
  try {
    const { data } = await getTicketCategories()
    realnameOk.value = data?.realname_ok ?? false
  } catch {
    realnameOk.value = false
  }
}

// ========== 公告（真实接口） ==========
const announcements = ref<AnnouncementInfo[]>([])

async function loadAnnouncements() {
  try {
    const { data } = await getMyAnnouncements()
    announcements.value = (data?.list ?? []).slice(0, 5)
  } catch {
    announcements.value = []
  }
}

function levelLabel(level: string): string {
  return { info: '公告', warning: '重要', critical: '紧急' }[level] || '公告'
}

// ========== 最近访问（localStorage，布局层统一记录） ==========
const recentPages = ref<RecentPage[]>([])

// ========== 快捷入口（全部指向控制台内真实页面） ==========
const quickEntries = computed(() => {
  const entries = [
    { title: '云主机选购', path: '/shop', icon: CartIcon },
    { title: '我的云主机', path: '/cloud/instances', icon: ServerIcon },
    { title: '续费管理', path: '/cloud/renewals', icon: RefreshIcon },
    { title: '我的订单', path: '/order', icon: OrderIcon },
    { title: '费用中心', path: '/billing', icon: WalletIcon },
    { title: '提交工单', path: '/support/tickets/create', icon: ServiceIcon },
  ]
  return memberStore.isOwner
    ? [...entries, { title: '成员管理', path: '/member', icon: ToolsIcon }]
    : entries
})

const heroKeyword = ref('')

function onHeroSearch() {
  const q = heroKeyword.value.trim()
  if (!q) return
  router.push({ path: '/shop', query: { keyword: q } })
}

// ========== 文档入口：门户承载，未配置官网地址时明确提示 ==========
interface DocLink {
  label: string
  icon: Component
  /** 门户路径（外链新窗口打开） */
  sitePath?: string
  /** 控制台内路径 */
  path?: string
}

const docLinks: DocLink[] = [
  { label: '帮助中心', icon: BookOpenIcon, sitePath: '/help' },
  { label: '新闻资讯', icon: FileIcon, sitePath: '/news' },
  { label: '服务公告', icon: InfoCircleFilledIcon, sitePath: '/announcements' },
  { label: '用户条款与隐私', icon: SecuredIcon, sitePath: '/terms' },
]

function onDocClick(link: DocLink) {
  if (link.path) {
    go(link.path)
    return
  }
  if (!link.sitePath) return
  if (!siteUrlConfigured) {
    MessagePlugin.info('官网地址未配置，请联系管理员在部署环境设置 VITE_SITE_URL')
    return
  }
  openSite(link.sitePath)
}

const devLinks = [
  { label: '提交工单', icon: ServiceIcon, path: '/support/tickets/create' },
  { label: '我的工单', icon: BrowseIcon, path: '/support/tickets' },
  { label: '我的消息', icon: MailIcon, path: '/profile/messages' },
  { label: '账户设置', icon: ToolsIcon, path: '/profile' },
]

const tools = computed(() => {
  const items: Array<{ label: string; path?: string; sitePath?: string }> = [
    { label: '工单', path: '/support/tickets' },
    { label: '价格计算器', path: '/shop' },
    { label: '消息中心', path: '/profile/messages' },
    { label: '账户设置', path: '/profile' },
    { label: '安全设置', path: '/profile/security' },
  ]
  // 帮助文档只在官网地址已配置时出现：未配置时渲染成不可点的按钮比缺失更糟
  if (siteUrlConfigured) items.push({ label: '帮助文档', sitePath: '/help' })
  return items
})

function onToolClick(item: { label: string; path?: string; sitePath?: string }) {
  if (item.path) {
    go(item.path)
    return
  }
  if (item.sitePath) openSite(item.sitePath)
}

onMounted(async () => {
  recentPages.value = getRecentPages()
  if (!userStore.loaded) {
    await userStore.fetchUserInfo()
  }
  await Promise.all([
    loadOverview(),
    loadPendingOrders(),
    loadBalances(),
    loadPromotions(),
    loadSecurity(),
    loadMembers(),
    loadRealname(),
    loadAnnouncements(),
  ])
})
</script>

<style scoped>
.console-page {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 560px;
  gap: 16px;
  align-items: start;
}

.console-main,
.console-side {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

/* ---------- 欢迎区 ---------- */
.welcome-hero {
  border-radius: 12px;
  padding: 18px 24px;
  background: linear-gradient(135deg, #e8efff 0%, #e3e9ff 55%, #ece8fd 100%);
  border: 1px solid #dbe4fb;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}

.welcome-panel {
  border-radius: 12px;
  padding: 18px 24px 20px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.hero-card__text {
  min-width: 0;
}

.hero-title {
  margin: 0 0 6px;
  font-size: 23px;
  font-weight: 700;
  letter-spacing: 0.01em;
  color: #1d4ed8;
}

.hero-title__name {
  color: #4f46e5;
}

.hero-sub {
  margin: 0 0 12px;
  font-size: 12.5px;
  color: #64748b;
}

/* 欢迎卡内搜索框 */
.hero-search {
  position: relative;
  display: flex;
  align-items: center;
  width: 420px;
  max-width: 100%;
}

.hero-search__icon {
  position: absolute;
  left: 14px;
  top: 50%;
  transform: translateY(-50%);
  color: #93a4c8;
  pointer-events: none;
}

.hero-search__input {
  width: 100%;
  height: 38px;
  padding: 0 16px 0 40px;
  border: 1px solid #d6e0f5;
  border-radius: 8px;
  background: #ffffff;
  color: #1e293b;
  font-size: 13.5px;
  outline: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.hero-search__input::placeholder {
  color: #9aa8c4;
}

.hero-search__input:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}

/* 欢迎卡右侧：真实资源计数 */
.hero-card__art {
  flex-shrink: 0;
  display: flex;
  gap: 12px;
}

.hero-stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  min-width: 96px;
  padding: 12px 14px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid #dbe4fb;
}

.hero-stat__value {
  font-size: 22px;
  font-weight: 700;
  color: #1d4ed8;
  line-height: 1.1;
}

.hero-stat__label {
  font-size: 12px;
  color: #64748b;
}

/* 最近访问 / 快捷入口 */
.welcome-block__title {
  margin: 0 0 12px;
  font-size: 13px;
  font-weight: 500;
  color: #64748b;
}

.welcome-empty {
  margin: 0;
  font-size: 13px;
  color: #94a3b8;
}

.recent-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.recent-chip {
  border: 1px solid #ffffff;
  background: #ffffff;
  color: #334155;
  font-size: 13px;
  padding: 9px 18px;
  border-radius: 8px;
  cursor: pointer;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
  transition: box-shadow 0.15s ease, color 0.15s ease, transform 0.15s ease;
}

.recent-chip:hover {
  color: var(--color-primary);
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.12);
  transform: translateY(-1px);
}

.entry-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.entry-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  justify-content: flex-start;
  border: 1px solid #ffffff;
  background: #ffffff;
  color: #334155;
  font-size: 13px;
  padding: 12px 16px;
  border-radius: 8px;
  cursor: pointer;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: box-shadow 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.entry-chip__icon {
  flex-shrink: 0;
  color: #94a3b8;
}

.entry-chip:hover {
  color: var(--color-primary);
  border-color: #dbe4ff;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.12);
}

.entry-chip:hover .entry-chip__icon {
  color: var(--color-primary);
}

/* ---------- 通用面板 ---------- */
.panel {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 16px 18px;
}

.panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.panel__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.panel__hint {
  font-size: 12px;
  color: #94a3b8;
}

.panel__more {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  border: none;
  background: transparent;
  color: #64748b;
  font-size: 12.5px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 6px;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.panel__more:hover {
  color: var(--color-primary);
  background: #eff6ff;
}

/* ---------- 资源概览 ---------- */
.res-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.res-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #eef2f7;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
}

.res-item:hover {
  transform: translateY(-2px);
  border-color: #bfdbfe;
  box-shadow: 0 6px 16px rgba(37, 99, 235, 0.1);
}

.res-item__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: #e0edff;
  color: var(--color-primary);
  flex-shrink: 0;
}

.res-item__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.res-item__value {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1.1;
}

.res-item__label {
  font-size: 12.5px;
  color: #64748b;
}

.link-btn {
  border: none;
  background: transparent;
  padding: 0;
  color: var(--color-primary);
  font-size: inherit;
  cursor: pointer;
}

.link-btn:hover {
  text-decoration: underline;
}

/* ---------- 服务推荐 ---------- */
.promo-tabs {
  display: flex;
  align-items: center;
  gap: 22px;
  margin-bottom: 14px;
  border-bottom: 1px solid #f1f5f9;
}

.promo-tab {
  position: relative;
  border: none;
  background: transparent;
  padding: 0 2px 10px;
  font-size: 13.5px;
  color: #64748b;
  cursor: pointer;
  transition: color 0.15s ease;
}

.promo-tab:hover {
  color: var(--color-primary);
}

.promo-tab.is-active {
  color: var(--color-primary);
  font-weight: 600;
}

.promo-tab.is-active::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px;
  height: 2px;
  border-radius: 2px;
  background: var(--color-primary);
}

.promo {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 20px;
  border-radius: 10px;
  background: linear-gradient(120deg, #f0f6ff 0%, #eaf1ff 100%);
  border: 1px solid #e0ecff;
}

.promo__body {
  max-width: 640px;
  min-width: 0;
}

.promo__title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}

.promo__desc {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.7;
  color: #64748b;
}

.promo__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}

.promo__tag {
  font-size: 12px;
  color: var(--color-primary);
  background: #ffffff;
  border: 1px solid #dbeafe;
  border-radius: 20px;
  padding: 2px 10px;
}

/* ---------- 帮助与支持 ---------- */
.learn-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.learn-section__title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
}

.learn-card {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 32px;
  padding: 22px 24px 24px;
  border-radius: 12px;
  background: #f7f8fa;
  border: 1px solid #eef1f5;
}

.learn-col {
  min-width: 0;
}

.learn-col__title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.learn-col__desc {
  margin: 0 0 16px;
  font-size: 12.5px;
  line-height: 1.7;
  color: #8b95a8;
}

.learn-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.learn-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 13px 16px;
  border-radius: 8px;
  border: 1px solid #e8ecf2;
  background: #ffffff;
  color: #334155;
  font-size: 13px;
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
}

.learn-item__left {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.learn-item__icon {
  color: #64748b;
  flex-shrink: 0;
}

.learn-item__arrow {
  color: #b6c2d4;
  flex-shrink: 0;
  transition: color 0.15s ease;
}

.learn-item:hover {
  color: var(--color-primary);
  border-color: #bfdbfe;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.1);
}

.learn-item:hover .learn-item__icon,
.learn-item:hover .learn-item__arrow {
  color: var(--color-primary);
}

/* ---------- 右侧信息栏 ---------- */
.side-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 18px;
}

.side-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.side-card__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

/* 账户卡 */
.account-card {
  padding: 18px;
}

.account-card__head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 14px;
  border-bottom: 1px solid #f1f5f9;
}

.account-card__avatar {
  background: var(--color-primary) !important;
  color: #ffffff !important;
  font-weight: 600;
}

.account-card__id {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.account-card__name {
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
}

.account-card__sub {
  font-size: 12px;
  color: #94a3b8;
}

.account-card__name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.account-card__badge {
  font-size: 11px;
  font-weight: 500;
  color: #4f46e5;
  background: #eef2ff;
  border: 1px solid #e0e7ff;
  border-radius: 4px;
  padding: 1px 7px;
  flex-shrink: 0;
}

.account-card__verify {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 4px 0;
}

.verify-item {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 12px;
}

.verify-item.is-ok {
  color: #10b981;
}

.verify-item.is-off {
  color: #94a3b8;
}

.account-warning {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 14px;
  padding: 8px 12px;
  border-radius: 8px;
  background: #fffbeb;
  border: 1px solid #fde68a;
}

.account-warning__left {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: #b45309;
}

.account-warning__action {
  border: none;
  background: transparent;
  color: #d97706;
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  padding: 0;
  flex-shrink: 0;
}

.account-warning__action:hover {
  color: #b45309;
  text-decoration: underline;
}

.account-card__stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  padding-top: 14px;
  margin-top: 14px;
  border-top: 1px solid #f1f5f9;
}

.mini-stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}

.mini-stat__value {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
}

.mini-stat__label {
  font-size: 12px;
  color: #94a3b8;
}

.mini-stat:hover .mini-stat__label {
  color: var(--color-primary);
}

/* 费用信息 */
.fee-label {
  font-size: 12.5px;
  color: #94a3b8;
}

.fee-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid #f1f5f9;
}

.fee-head__info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.fee-head__value {
  font-size: 26px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1;
}

.fee-tiles {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.fee-tile {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  border-radius: 8px;
  background: #f8fafc;
}

.fee-tile__value {
  font-size: 17px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1;
}

/* 账户安全 */
.sec-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sec-list__item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  min-width: 0;
}

.sec-list__icon {
  color: #64748b;
  flex-shrink: 0;
}

.sec-list__label {
  color: #475569;
  flex-shrink: 0;
}

.sec-list__state {
  margin-left: auto;
  font-size: 12.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sec-list__state.is-ok {
  color: #10b981;
}

.sec-list__state.is-warn {
  color: #f59e0b;
}

.sec-note {
  margin: 14px 0 0;
  font-size: 12px;
  line-height: 1.7;
  color: #94a3b8;
}

/* 成员与协作 */
.access-url {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  border-radius: 8px;
  background: #f8fafc;
  font-size: 12.5px;
  min-width: 0;
}

.access-url__label {
  color: #94a3b8;
  flex-shrink: 0;
}

.access-url__value {
  color: #475569;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}

.access-url__copy {
  border: none;
  background: transparent;
  color: var(--color-primary);
  font-size: 12.5px;
  cursor: pointer;
  padding: 0;
  flex-shrink: 0;
}

.access-url__copy:hover {
  text-decoration: underline;
}

.access-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.access-stat {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  border-radius: 8px;
  background: #f8fafc;
}

.access-stat__label {
  font-size: 12.5px;
  color: #94a3b8;
}

.access-stat__value {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1;
}

/* 公告 */
.announce-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.announce-item {
  display: flex;
  flex-direction: column;
  gap: 7px;
  min-width: 0;
  cursor: pointer;
}

.announce-item__row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.announce-item__tag {
  flex-shrink: 0;
  font-size: 11px;
  line-height: 1.6;
  padding: 0 6px;
  border-radius: 3px;
  background: #f1f5f9;
  color: #64748b;
}

.announce-item__tag.is-warning {
  background: #fffbeb;
  color: #d97706;
}

.announce-item__tag.is-critical {
  background: #fef2f2;
  color: #dc2626;
}

.announce-item__title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: #334155;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.announce-item:hover .announce-item__title {
  color: var(--color-primary);
}

.announce-item__time {
  font-size: 12px;
  color: #94a3b8;
}

/* 工具 */
.tool-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.tool-item {
  display: flex;
  align-items: center;
  padding: 14px 16px;
  border-radius: 8px;
  border: none;
  background: #f8fafc;
  color: #334155;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.tool-item:hover {
  background: #eff6ff;
  color: var(--color-primary);
}

/* ---------- 深色模式 ---------- */
.dark .welcome-hero {
  background: linear-gradient(135deg, #141a2e 0%, #171c33 55%, #191736 100%);
  border-color: #232a44;
}

.dark .welcome-panel {
  background: #0a0a0a;
  border-color: #262626;
}

.dark .hero-stat {
  background: rgba(15, 20, 36, 0.7);
  border-color: #232a44;
}

.dark .hero-stat__value {
  color: #93b4fd;
}

.dark .hero-search__input {
  background: #0f1424;
  border-color: #2a3350;
  color: #e5e7eb;
}

.dark .recent-chip,
.dark .entry-chip {
  background: #0f1424;
  border-color: #232a44;
  color: #cbd5e1;
}

.dark .account-card__badge {
  color: #a5b4fc;
  background: #1e1b4b;
  border-color: #312e81;
}

.dark .account-warning {
  background: #2a1f0a;
  border-color: #5a420f;
}

.dark .account-warning__left,
.dark .account-warning__action {
  color: #fbbf24;
}

.dark .account-card__stats,
.dark .fee-head {
  border-color: #262626;
}

.dark .fee-head__value,
.dark .fee-tile__value,
.dark .access-stat__value,
.dark .mini-stat__value {
  color: #e5e7eb;
}

.dark .fee-tile,
.dark .access-url,
.dark .access-stat,
.dark .tool-item {
  background: #0f1424;
}

.dark .access-url__value,
.dark .tool-item,
.dark .sec-list__label {
  color: #cbd5e1;
}

.dark .announce-item__tag {
  background: #232a44;
  color: #94a3b8;
}

.dark .announce-item__title {
  color: #cbd5e1;
}

.dark .promo-tabs {
  border-color: #232a44;
}

.dark .promo {
  background: linear-gradient(120deg, #131a2e 0%, #141c33 100%);
  border-color: #232a44;
}

.dark .promo__title {
  color: #e5e7eb;
}

.dark .promo__tag {
  background: #0f1424;
  border-color: #2a3350;
}

.dark .learn-section__title,
.dark .learn-col__title {
  color: #e5e7eb;
}

.dark .learn-card {
  background: #0f1424;
  border-color: #232a44;
}

.dark .learn-item {
  background: #141a2e;
  border-color: #232a44;
  color: #cbd5e1;
}

.dark .learn-item__icon {
  color: #94a3b8;
}

/* ---------- 响应式 ---------- */
@media (max-width: 1280px) {
  .console-page {
    grid-template-columns: minmax(0, 1fr) 440px;
  }
}

@media (max-width: 1024px) {
  .console-page {
    grid-template-columns: 1fr;
  }

  .res-grid,
  .entry-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .learn-card {
    grid-template-columns: minmax(0, 1fr);
    gap: 26px;
  }
}

@media (max-width: 640px) {
  .welcome-hero {
    flex-direction: column;
    align-items: flex-start;
  }

  .hero-card__art {
    width: 100%;
  }

  .hero-stat {
    flex: 1;
  }

  .res-grid,
  .entry-grid {
    grid-template-columns: 1fr;
  }

  .learn-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .promo {
    flex-direction: column;
    align-items: flex-start;
  }

  .promo-tabs {
    gap: 16px;
    overflow-x: auto;
  }
}
</style>
