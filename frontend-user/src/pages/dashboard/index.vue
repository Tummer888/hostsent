<template>
  <div class="console-page">
    <!-- ============ 左主区 ============ -->
    <div class="console-main">
      <!-- 上层：欢迎卡（标题 + 搜索 + 背景图占位） -->
      <section class="welcome-hero">
        <div class="hero-card__text">
          <h2 class="hero-title">
            欢迎使用<span class="hero-title__name">{{ brandStore.name }}</span>
          </h2>
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
        <div class="hero-card__art" aria-hidden="true">
          <!-- 背景图占位：后续替换为实际插画 -->
          <div class="hero-art-placeholder">
            <span class="hero-art-placeholder__text">背景图占位</span>
          </div>
        </div>
      </section>

      <!-- 下层：最近访问 + 自定义快捷入口 -->
      <section class="welcome-panel">
        <div class="welcome-block">
          <h4 class="welcome-block__title">最近访问</h4>
          <div class="recent-row">
            <button
              v-for="r in recentVisits"
              :key="r.title"
              class="recent-chip"
              @click="go(r.path)"
            >
              {{ r.title }}
            </button>
          </div>
        </div>

        <div class="welcome-block">
          <h4 class="welcome-block__title">自定义快捷入口</h4>
          <div class="entry-grid">
            <button
              v-for="e in customEntries"
              :key="e.title"
              class="entry-chip"
              @click="go(e.path)"
            >
              {{ e.title }}
            </button>
            <button class="entry-chip entry-chip--add" @click="onAddEntry">
              <AddIcon size="14" />
              添加入口
            </button>
          </div>
        </div>
      </section>

      <!-- 资源概览 -->
      <section class="panel">
        <header class="panel__head">
          <h3 class="panel__title">我的资源</h3>
          <button class="panel__more" @click="go('/cloud/instances')">
            查看全部 <ChevronRightIcon size="14" />
          </button>
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

      <!-- 运维监控 + 安全监测 -->
      <div class="monitor-row">
        <section class="panel monitor-card">
          <header class="panel__head">
            <h3 class="panel__title">运维监控</h3>
          </header>
          <div class="monitor-stats">
            <div v-for="m in monitorStats" :key="m.label" class="monitor-stat">
              <span class="monitor-stat__label">{{ m.label }}</span>
              <span class="monitor-stat__value">{{ m.value }}</span>
            </div>
          </div>
          <div class="monitor-note">
            <div class="monitor-note__head">
              <InfoCircleFilledIcon size="14" />
              <span>暂未设置资源监控</span>
            </div>
            <p class="monitor-note__desc">
              设置云监控可以及时快速处理预警情况，保障业务平稳运行。
              <button class="link-btn" @click="go('/cloud/instances')">立即设置</button>
            </p>
          </div>
        </section>

        <section class="panel monitor-card monitor-card--security">
          <header class="panel__head">
            <h3 class="panel__title">安全监测</h3>
          </header>
          <div class="security-cols">
            <div class="security-cols__text">
              <p class="security-tip">
                了解更多安全风险信息，查看
                <button class="link-btn" @click="go('/profile')">安全合规中心</button>
              </p>
              <p class="security-score-line">
                您当前的安全评分为 <strong>安全</strong>
              </p>
              <ul class="security-list">
                <li v-for="s in securityItems" :key="s.label" class="security-list__item">
                  <component :is="s.icon" size="14" :class="s.ok ? 'is-ok' : 'is-warn'" />
                  <span>{{ s.label }}</span>
                </li>
              </ul>
              <t-button theme="primary" size="small" class="security-action" @click="go('/profile')">
                立即处理
              </t-button>
            </div>
            <div class="sec-gauge">
              <svg viewBox="0 0 160 92" class="sec-gauge__svg" aria-hidden="true">
                <path d="M14 84 A66 66 0 0 1 146 84" class="sec-gauge__track" />
                <path
                  d="M14 84 A66 66 0 0 1 146 84"
                  class="sec-gauge__value"
                  :stroke-dasharray="gaugeDash"
                />
              </svg>
              <div class="sec-gauge__center">
                <span class="sec-gauge__score">{{ securityScore }}</span>
              </div>
              <span class="sec-gauge__time">{{ nowText }}</span>
            </div>
          </div>
        </section>
      </div>

      <!-- 服务推荐 -->
      <section class="panel">
        <header class="panel__head">
          <h3 class="panel__title">服务推荐</h3>
        </header>
        <div class="promo-tabs">
          <button
            v-for="p in promoList"
            :key="p.key"
            class="promo-tab"
            :class="{ 'is-active': activePromo === p.key }"
            @click="activePromo = p.key"
          >
            {{ p.tab }}
          </button>
        </div>
        <div class="promo">
          <div class="promo__body">
            <h4 class="promo__title">{{ activePromoItem.title }}</h4>
            <p class="promo__desc">{{ activePromoItem.desc }}</p>
            <div class="promo__tags">
              <span v-for="tag in activePromoItem.tags" :key="tag" class="promo__tag">{{ tag }}</span>
            </div>
            <t-button theme="primary" size="small" @click="go(activePromoItem.path)">
              {{ activePromoItem.cta }}
            </t-button>
          </div>
          <div class="promo__art" aria-hidden="true">
            <div class="promo__art-mock">
              <div class="promo__art-mock__side">
                <span v-for="n in 4" :key="n" class="promo__art-mock__dot"></span>
              </div>
              <div class="promo__art-mock__main">
                <span class="promo__art-mock__bar promo__art-mock__bar--lg"></span>
                <span class="promo__art-mock__bar"></span>
                <span class="promo__art-mock__bar promo__art-mock__bar--sm"></span>
                <span class="promo__art-mock__label">产品界面占位</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 学习与开发者资源 -->
      <section class="learn-section">
        <h3 class="learn-section__title">学习与开发者资源</h3>
        <div class="learn-card">
          <div class="learn-col">
            <h4 class="learn-col__title">文档中心</h4>
            <p class="learn-col__desc">
              畅享海量文档、丰富示例代码及专业教程，利用 AI 云构建与管理卓越应用
            </p>
            <div class="learn-grid">
              <button
                v-for="l in docLinks"
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

          <div class="learn-col">
            <h4 class="learn-col__title">开发者资源</h4>
            <p class="learn-col__desc">开发者所需的任何资源都在这，快速掌握云产品专业玩法。</p>
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
              <span class="account-card__badge">主账号</span>
            </div>
            <div class="account-card__verify">
              <span class="verify-item is-ok"><CheckCircleIcon size="13" /> 已认证</span>
              <span class="verify-item is-ok"><CheckCircleIcon size="13" /> 已绑定</span>
              <span class="verify-item is-off"><CloseCircleIcon size="13" /> 未绑定</span>
            </div>
            <span class="account-card__sub">账号 ID：{{ accountId }}</span>
          </div>
        </div>

        <!-- 未绑定邮箱提醒 -->
        <div class="account-warning">
          <span class="account-warning__left">
            <InfoCircleFilledIcon size="14" />
            未绑定邮箱
          </span>
          <button class="account-warning__action" @click="onBindEmail">立即绑定</button>
        </div>
        <div class="account-card__stats">
          <div class="mini-stat">
            <span class="mini-stat__value">0</span>
            <span class="mini-stat__label">待支付</span>
          </div>
          <div class="mini-stat">
            <span class="mini-stat__value">0</span>
            <span class="mini-stat__label">待续费</span>
          </div>
          <div class="mini-stat">
            <span class="mini-stat__value">0</span>
            <span class="mini-stat__label">我的工单</span>
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
            <span class="fee-head__value">¥ 0.00</span>
          </div>
          <t-button v-if="memberStore.has('billing:recharge')" theme="primary" size="small" @click="go('/billing/balance')">充值</t-button>
        </div>
        <div class="fee-tiles">
          <div class="fee-tile">
            <span class="fee-label">可开票金额</span>
            <span class="fee-tile__value">¥ 49.00</span>
          </div>
          <div class="fee-tile">
            <span class="fee-label">代金券金额</span>
            <span class="fee-tile__value">¥ 0.00</span>
          </div>
        </div>
      </section>

      <!-- 访问控制 -->
      <section class="side-card">
        <header class="side-card__head">
          <h4 class="side-card__title">访问控制</h4>
          <div class="side-card__actions">
            <button class="panel__more" @click="go('/profile')">创建子用户</button>
            <span class="side-card__sep"></span>
            <button class="panel__more" @click="go('/profile')">权限管理</button>
          </div>
        </header>
        <div class="access-url">
          <span class="access-url__label">子用户登录：</span>
          <span class="access-url__value">{{ subAccountUrl }}</span>
        </div>
        <div class="access-stats">
          <div v-for="a in accessStats" :key="a.label" class="access-stat">
            <span class="access-stat__label">{{ a.label }}</span>
            <span class="access-stat__value">{{ a.value }}</span>
          </div>
        </div>
      </section>

      <!-- 最新公告 -->
      <section class="side-card">
        <header class="side-card__head">
          <h4 class="side-card__title">最新公告</h4>
          <button class="panel__more" @click="go('/support')">更多 <ChevronRightIcon size="13" /></button>
        </header>
        <ul class="announce-list">
          <li v-for="a in announcements" :key="a.title" class="announce-item">
            <div class="announce-item__row">
              <span class="announce-item__tag" :class="`is-${a.type}`">{{ a.typeLabel }}</span>
              <span class="announce-item__title">{{ a.title }}</span>
            </div>
            <span class="announce-item__time">{{ a.date }}</span>
          </li>
        </ul>
      </section>

      <!-- 常用工具 -->
      <section class="side-card">
        <header class="side-card__head">
          <h4 class="side-card__title">常用工具</h4>
        </header>
        <div class="tool-grid">
          <button v-for="t in tools" :key="t.label" class="tool-item" @click="go(t.path)">
            {{ t.label }}
          </button>
        </div>
      </section>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  AddIcon,
  ApiIcon,
  BookOpenIcon,
  BrowseIcon,
  CartIcon,
  CheckCircleIcon,
  CheckCircleFilledIcon,
  CloseCircleIcon,
  CodeIcon,
  InfoCircleFilledIcon,
  ChevronRightIcon,
  ErrorCircleFilledIcon,
  FileIcon,
  JumpIcon,
  LayersIcon,
  PlayCircleIcon,
  RefreshIcon,
  SearchIcon,
  ServerIcon,
  ToolsIcon,
  VideoIcon,
} from 'tdesign-icons-vue-next'

import { useUserStore } from '@/store'
import { useMemberStore } from '@/store/modules/member'
import { useBrandStore } from '@/store/modules/brand'

defineOptions({ name: 'UserConsole' })

const router = useRouter()
const userStore = useUserStore()
const memberStore = useMemberStore()
const brandStore = useBrandStore()

function go(path: string) {
  router.push(path)
}

const displayName = computed(() => userStore.displayName || '用户')
const avatarText = computed(() => displayName.value.slice(0, 1).toUpperCase())
const accountId = computed(() => String(userStore.userInfo?.id ?? '100000000000').padStart(12, '0'))

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

// ===== 静态占位数据（后续接入后端接口时替换） =====
const resources = [
  { key: 'instance', label: '云主机', value: 0, icon: ServerIcon, path: '/cloud/instances' },
  { key: 'image', label: '镜像', value: 0, icon: LayersIcon, path: '/cloud/images' },
  { key: 'renewal', label: '待续费', value: 0, icon: RefreshIcon, path: '/cloud/renewals' },
  { key: 'order', label: '订单', value: 0, icon: CartIcon, path: '/order' },
]

const recentVisits = [
  { title: '轻量云主机', path: '/cloud/instances' },
]

// 自定义快捷入口（静态占位；后续可持久化到用户偏好）
const customEntries = [
  { title: '云主机 CVM', path: '/shop' },
  { title: '原生容器', path: '/shop' },
  { title: '私有网络', path: '/cloud/instances' },
  { title: '云硬盘', path: '/cloud/instances' },
  { title: '对象存储', path: '/cloud/images' },
  { title: '云搜索 Elasticsearch', path: '/shop' },
  { title: '分布式缓存(兼容Redis)', path: '/shop' },
]

const heroKeyword = ref('')

function onHeroSearch() {
  const q = heroKeyword.value.trim()
  if (!q) return
  router.push({ path: '/shop', query: { keyword: q } })
}

function onAddEntry() {
  MessagePlugin.info('添加快捷入口开发中')
}

function onBindEmail() {
  MessagePlugin.info('邮箱绑定开发中')
}

const securityScore = 97
const securityItems = [
  { label: '暂无告警', ok: true, icon: CheckCircleFilledIcon },
  { label: '存在 0 台主机未安装防护 Agent', ok: false, icon: ErrorCircleFilledIcon },
  { label: '暂无漏洞', ok: true, icon: CheckCircleFilledIcon },
]

// 半圆仪表盘：弧长为半径 66 的半个圆周
const gaugeLen = Math.PI * 66
const gaugeDash = computed(() => `${(securityScore / 100) * gaugeLen} ${gaugeLen}`)

const nowText = new Date()
  .toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
  .replace(/\//g, '-')

// 运维监控统计（静态占位）
const monitorStats = [
  { label: '正在报警', value: 0 },
  { label: '云资源监控', value: 0 },
  { label: '自定义监控', value: 0 },
]

// 服务推荐：按产品切换的 tab 内容（静态占位，后续接入推荐接口）
const promoList = [
  {
    key: 'cvm',
    tab: '云主机',
    title: '云主机 CVM',
    desc: '高性能、可弹性伸缩的计算服务，支持按量付费与包年包月，适配建站、开发测试、企业应用等多种场景。',
    tags: ['弹性伸缩', '高可用', '安全隔离'],
    cta: '立即选购',
    path: '/shop',
  },
  {
    key: 'lighthouse',
    tab: '轻量云主机',
    title: '轻量云主机',
    desc: '开箱即用的轻量应用服务器，固定套餐、流量包月，适合个人开发者与中小企业快速搭建业务。',
    tags: ['开箱即用', '固定套餐', '成本可控'],
    cta: '立即选购',
    path: '/shop',
  },
  {
    key: 'joyagent',
    tab: 'JoyAgent',
    title: 'JoyAgent 智能体平台',
    desc: '面向企业的一站式智能体开发平台，支持多模型接入与可视化编排，快速构建专属 AI 应用。',
    tags: ['智能编排', '多模型', '低代码'],
    cta: '立即体验',
    path: '/shop',
  },
  {
    key: 'joycode',
    tab: 'JoyCode',
    title: 'JoyCode 智能编码',
    desc: 'AI 驱动的智能编码助手，提供代码补全、单元测试生成与代码审查，显著提升研发效率。',
    tags: ['AI 补全', '代码审查', '团队协作'],
    cta: '立即体验',
    path: '/shop',
  },
  {
    key: 'joybuilder',
    tab: 'JoyBuilder',
    title: 'JoyBuilder 模型开发平台',
    desc: 'JoyBuilder 模型开发平台为开发者提供从数据准备、模型训练到推理部署的一站式双工作流 AI 开发服务，支持多种主流框架与高性能分布式训练。',
    tags: ['一站式', '多框架', '高性能'],
    cta: '立即体验',
    path: '/shop',
  },
]

const activePromo = ref('joybuilder')
const activePromoItem = computed(
  () => promoList.find((p) => p.key === activePromo.value) ?? promoList[0],
)

// 学习与开发者资源（静态占位）
const docLinks = [
  { label: '入门指南', icon: BookOpenIcon, path: '/support' },
  { label: '课程中心', icon: PlayCircleIcon, path: '/support' },
  { label: '云智公开课', icon: VideoIcon, path: '/support' },
  { label: '解决方案实践', icon: FileIcon, path: '/support' },
]

const devLinks = [
  { label: 'OpenAPI', icon: ApiIcon, path: '/support' },
  { label: 'SDK 中心', icon: BrowseIcon, path: '/support' },
  { label: '示例代码', icon: CodeIcon, path: '/support' },
  { label: '自助工具', icon: ToolsIcon, path: '/support' },
]

const announcements = [
  { type: 'update', typeLabel: '产品公告', title: '增强型网络负载均衡全量开放及计费说明', date: '2026-09-09 18:04' },
  { type: 'update', typeLabel: '产品公告', title: '【重要通知】平台 14 款模型自部署服务下线公告', date: '2026-09-04 10:39' },
  { type: 'update', typeLabel: '产品公告', title: '【重要通知】DeepSeek-V4-Pro 自部署服务下线及迁移安排', date: '2026-08-27 14:47' },
  { type: 'update', typeLabel: '产品公告', title: '【重要通知】DeepSeek-V4-Flash-Preview 自部署服务下线及迁移安排', date: '2026-08-11 18:12' },
  { type: 'notice', typeLabel: '活动公告', title: '备案升级公告', date: '2024-09-02 13:19' },
]

// 访问控制（静态占位）
const subAccountUrl = `${window.location.origin}/subaccount/login/308972543164`
const accessStats = [
  { label: '用户数', value: 0 },
  { label: '群组', value: 0 },
  { label: '角色', value: 3 },
  { label: '策略', value: 0 },
]

const tools = computed(() =>
  [
    { label: '工单', path: '/support' },
    { label: '价格计算器', path: '/shop' },
    { label: '消息中心', path: '/profile' },
    { label: 'API 密钥', path: '/profile' },
    { label: '实名认证', path: '/profile', ownerOnly: true },
    { label: '备案管理', path: '/profile', ownerOnly: true },
    { label: '账户设置', path: '/profile' },
    { label: '帮助文档', path: '/support' },
  ].filter((item) => !item.ownerOnly || memberStore.isOwner),
)
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
  margin: 0 0 12px;
  font-size: 23px;
  font-weight: 700;
  letter-spacing: 0.01em;
  color: #1d4ed8;
}

.hero-title__name {
  color: #4f46e5;
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

/* 背景图占位 */
.hero-card__art {
  flex-shrink: 0;
}

.hero-art-placeholder {
  width: 168px;
  height: 92px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  border: 1px dashed #b9c7ea;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.6), rgba(224, 231, 255, 0.5));
}

.hero-art-placeholder__text {
  font-size: 12px;
  color: #8a9bc4;
  letter-spacing: 0.04em;
}

/* 最近访问 / 自定义快捷入口 */
.welcome-block__title {
  margin: 0 0 12px;
  font-size: 13px;
  font-weight: 500;
  color: #64748b;
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

.entry-chip:hover {
  color: var(--color-primary);
  border-color: #dbe4ff;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.12);
}

.entry-chip--add {
  color: #64748b;
  border-style: dashed;
  border-color: #c7d4ef;
  background: transparent;
  box-shadow: none;
}

.entry-chip--add:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
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

/* ---------- 运维监控 + 安全监测 ---------- */
.monitor-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px;
  align-items: stretch;
}

.monitor-card {
  display: flex;
  flex-direction: column;
}

.monitor-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.monitor-stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 12px 8px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #eef2f7;
}

.monitor-stat__label {
  font-size: 12px;
  color: #64748b;
}

.monitor-stat__value {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1.1;
}

.monitor-note {
  margin-top: auto;
  padding: 12px 14px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #eef2f7;
}

.monitor-note__head {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: #475569;
}

.monitor-note__head svg {
  color: #f59e0b;
}

.monitor-note__desc {
  margin: 6px 0 0;
  font-size: 12.5px;
  line-height: 1.7;
  color: #94a3b8;
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

.security-cols {
  display: flex;
  align-items: stretch;
  justify-content: space-between;
  gap: 16px;
  flex: 1;
}

.security-cols__text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.security-tip {
  margin: 0 0 6px;
  font-size: 12.5px;
  color: #94a3b8;
}

.security-score-line {
  margin: 0 0 12px;
  font-size: 13px;
  color: #475569;
}

.security-score-line strong {
  color: #10b981;
  font-weight: 600;
}

.security-action {
  align-self: flex-start;
  margin-top: auto;
}

.sec-gauge {
  position: relative;
  width: 150px;
  flex-shrink: 0;
  padding-bottom: 18px;
}

.sec-gauge__svg {
  width: 150px;
  height: 86px;
  overflow: visible;
}

.sec-gauge__track {
  fill: none;
  stroke: #eef2f7;
  stroke-width: 10;
  stroke-linecap: round;
}

.sec-gauge__value {
  fill: none;
  stroke: #10b981;
  stroke-width: 10;
  stroke-linecap: round;
}

.sec-gauge__center {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 18px;
  display: flex;
  align-items: baseline;
  justify-content: center;
}

.sec-gauge__score {
  font-size: 30px;
  font-weight: 700;
  color: #10b981;
  line-height: 1;
}

.sec-gauge__time {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  text-align: center;
  font-size: 11px;
  color: #cbd5e1;
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
  max-width: 560px;
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

.promo__art {
  flex-shrink: 0;
}

.promo__art-mock {
  display: flex;
  width: 208px;
  height: 118px;
  border-radius: 10px;
  overflow: hidden;
  background: #ffffff;
  border: 1px solid #dbe7fb;
  box-shadow: 0 6px 18px rgba(37, 99, 235, 0.08);
}

.promo__art-mock__side {
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 12px 10px;
  background: #f1f6ff;
  border-right: 1px solid #e3ecfb;
}

.promo__art-mock__dot {
  width: 22px;
  height: 6px;
  border-radius: 3px;
  background: #c7dcfb;
}

.promo__art-mock__main {
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 12px;
  flex: 1;
}

.promo__art-mock__bar {
  height: 8px;
  border-radius: 4px;
  background: #e2ecfc;
  width: 100%;
}

.promo__art-mock__bar--lg {
  height: 26px;
  background: #d3e4fd;
}

.promo__art-mock__bar--sm {
  width: 56%;
}

.promo__art-mock__label {
  margin-top: auto;
  font-size: 11px;
  color: #9db3d6;
}

/* ---------- 学习与开发者资源 ---------- */
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

/* 访问控制 */
.side-card__actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.side-card__sep {
  width: 1px;
  height: 11px;
  background: #e2e8f0;
}

.access-url {
  display: flex;
  align-items: center;
  gap: 4px;
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
}

.access-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
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

/* 安全 */
.security-list {
  list-style: none;
  margin: 0 0 14px;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.security-list__item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: #475569;
}

.security-list__item .is-ok {
  color: #10b981;
}

.security-list__item .is-warn {
  color: #f59e0b;
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

.announce-item__tag.is-update {
  background: #f1f5f9;
  color: #64748b;
}

.announce-item__tag.is-notice {
  background: #ecfdf5;
  color: #059669;
}

.announce-item__tag.is-security {
  background: #fffbeb;
  color: #d97706;
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

.dark .entry-chip--add {
  background: transparent;
  border-color: #2a3350;
}

/* ---------- 深色模式（账户/费用补充） ---------- */
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
.dark .access-stat__value {
  color: #e5e7eb;
}

.dark .fee-tile,
.dark .access-url,
.dark .access-stat,
.dark .tool-item {
  background: #0f1424;
}

.dark .access-url__value,
.dark .tool-item {
  color: #cbd5e1;
}

.dark .side-card__sep {
  background: #2a3350;
}

.dark .announce-item__tag,
.dark .announce-item__tag.is-update {
  background: #232a44;
  color: #94a3b8;
}

.dark .announce-item__title {
  color: #cbd5e1;
}

/* ---------- 深色模式（监控 / 推荐） ---------- */
.dark .monitor-stat,
.dark .monitor-note {
  background: #0f1424;
  border-color: #232a44;
}

.dark .monitor-stat__value {
  color: #e5e7eb;
}

.dark .monitor-note__head {
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

.dark .promo__art-mock {
  background: #0f1424;
  border-color: #232a44;
  box-shadow: none;
}

.dark .promo__art-mock__side {
  background: #131a2e;
  border-color: #232a44;
}

.dark .promo__art-mock__dot,
.dark .promo__art-mock__bar {
  background: #2a3350;
}

.dark .promo__art-mock__bar--lg {
  background: #33405f;
}

.dark .sec-gauge__track {
  stroke: #232a44;
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
    display: none;
  }

  .res-grid,
  .entry-grid,
  .monitor-row {
    grid-template-columns: 1fr;
  }

  .access-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .learn-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .security-cols {
    flex-direction: column;
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
