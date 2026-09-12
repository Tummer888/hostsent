<template>
  <transition name="pm-open">
    <div v-if="open" class="pm-root">
      <button class="pm-backdrop" aria-label="关闭菜单" @click="close"></button>

      <div class="pm-panel" role="dialog" aria-label="全部云产品">
      <!-- ============ 左侧导航栏 ============ -->
      <aside class="pm-rail">
        <nav class="pm-rail__nav">
          <button
            v-for="r in railItems"
            :key="r.key"
            class="pm-rail__item"
            :class="{ 'is-active': activeRail === r.key }"
            @click="activeRail = r.key"
          >
            <component :is="r.icon" size="17" />
            <span class="pm-rail__label">{{ r.label }}</span>
            <ChevronRightIcon size="15" class="pm-rail__arrow" />
          </button>

          <template v-if="activeRail === 'products'">
            <p class="pm-rail__hint">
              <StarIcon size="13" class="pm-rail__hint-icon" />
              暂无收藏产品。您可在全部云产品菜单中点击星号，将常用产品添加收藏。
            </p>

            <h4 class="pm-rail__title">
              热门产品推荐 <span class="pm-rail__fire">🔥</span>
            </h4>
            <button
              v-for="h in hotProducts"
              :key="h.label"
              class="pm-rail__link"
              @click="select(h)"
            >
              {{ h.label }}
            </button>
          </template>
        </nav>

        <div class="pm-rail__footer">
          <button
            class="pm-rail__tools"
            :class="{ 'is-open': toolsOpen }"
            @click="toolsOpen = !toolsOpen"
          >
            <ToolsIcon size="17" />
            <span class="pm-rail__label">平台工具</span>
            <ChevronUpIcon v-if="toolsOpen" size="15" class="pm-rail__arrow" />
            <ChevronDownIcon v-else size="15" class="pm-rail__arrow" />
          </button>
          <transition name="pm-tools">
            <div v-if="toolsOpen" class="pm-rail__tools-list">
              <button
                v-for="t in platformTools"
                :key="t.label"
                class="pm-rail__link"
                @click="select(t)"
              >
                {{ t.label }}
              </button>
            </div>
          </transition>
        </div>
      </aside>

      <!-- ============ 中间内容 ============ -->
      <section class="pm-main">
        <div class="pm-search">
          <input
            v-model="keyword"
            class="pm-search__input"
            type="text"
            placeholder="请输入关键字搜索产品"
            @keyup.enter="onSearch"
          />
          <SearchIcon size="17" class="pm-search__icon" />
        </div>

        <div ref="scrollRef" class="pm-scroll">
          <!-- 全部云产品 -->
          <template v-if="activeRail === 'products'">
            <section class="pm-block">
              <h3 class="pm-block__title">最近访问产品</h3>
              <div class="pm-recent">
                <button
                  v-for="r in recentProducts"
                  :key="r.label"
                  class="pm-recent__item"
                  @click="select(r)"
                >
                  {{ r.label }}
                </button>
              </div>
            </section>

            <div class="pm-cols">
              <div v-for="(col, ci) in productColumns" :key="ci" class="pm-col">
                <section
                  v-for="cat in col"
                  :key="cat.title"
                  class="pm-cat"
                  :data-cat="cat.title"
                >
                  <h3 class="pm-cat__title">{{ cat.title }}</h3>
                  <div v-for="group in cat.groups" :key="group.title" class="pm-group">
                    <h4 class="pm-group__title">{{ group.title }}</h4>
                    <button
                      v-for="p in group.items"
                      :key="p.label"
                      class="pm-link"
                      @click="select(p)"
                    >
                      {{ p.label }}
                    </button>
                  </div>
                </section>
              </div>
            </div>
          </template>

          <!-- 我的资源 -->
          <template v-else-if="activeRail === 'resources'">
            <section
              v-for="group in resourceGroups"
              :key="group.title"
              class="pm-block"
            >
              <h3 class="pm-block__title">{{ group.title }}</h3>
              <div class="pm-recent">
                <button
                  v-for="item in group.items"
                  :key="item.label"
                  class="pm-recent__item"
                  @click="select(item)"
                >
                  {{ item.label }}
                </button>
              </div>
            </section>
          </template>

          <!-- 最近访问页面 -->
          <template v-else>
            <section class="pm-block">
              <h3 class="pm-block__title">最近访问页面</h3>
              <div class="pm-recent">
                <button
                  v-for="item in recentPages"
                  :key="item.label"
                  class="pm-recent__item"
                  @click="select(item)"
                >
                  {{ item.label }}
                </button>
              </div>
            </section>
          </template>
        </div>
      </section>

      <!-- ============ 右侧分类索引 ============ -->
      <aside v-if="activeRail === 'products'" class="pm-index">
        <button
          v-for="c in categoryIndex"
          :key="c"
          class="pm-index__item"
          :class="{ 'is-active': activeCategory === c }"
          @click="scrollToCategory(c)"
        >
          {{ c }}
        </button>
      </aside>

      <button class="pm-close" aria-label="关闭" @click="close">
        <CloseIcon size="20" />
      </button>
    </div>

    </div>
  </transition>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  ChevronDownIcon,
  ChevronRightIcon,
  ChevronUpIcon,
  CloseIcon,
  GridViewIcon,
  SearchIcon,
  ServerIcon,
  StarIcon,
  TimeIcon,
  ToolsIcon,
} from 'tdesign-icons-vue-next'

defineOptions({ name: 'UserProductMenu' })

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const router = useRouter()

// ===== 左侧导航 =====
type RailKey = 'products' | 'resources' | 'recent'

const railItems: { key: RailKey; label: string; icon: Component }[] = [
  { key: 'products', label: '全部云产品', icon: GridViewIcon },
  { key: 'resources', label: '我的资源', icon: ServerIcon },
  { key: 'recent', label: '最近访问页面', icon: TimeIcon },
]

const activeRail = ref<RailKey>('products')
const activeCategory = ref('')
const toolsOpen = ref(true)
const keyword = ref('')
const scrollRef = ref<HTMLElement | null>(null)

// ===== 文案数据（静态占位，后续可接入产品目录接口） =====
interface MenuItem {
  label: string
  path?: string
}

const hotProducts: MenuItem[] = [
  { label: '云服务器', path: '/shop' },
  { label: '对象存储', path: '/shop' },
  { label: '内容分发网络 CDN', path: '/shop' },
  { label: 'ICP备案', path: '/support' },
  { label: '轻量应用服务器', path: '/shop' },
  { label: '域名注册', path: '/shop' },
  { label: 'SSL 证书', path: '/support' },
  { label: '短信', path: '/shop' },
]

const platformTools: MenuItem[] = [
  { label: '价格计算器', path: '/shop' },
  { label: '云 API 密钥', path: '/profile' },
  { label: '费用中心', path: '/billing' },
  { label: '提交工单', path: '/support/tickets/create' },
  { label: '帮助文档', path: '/support' },
  { label: '实名认证', path: '/profile' },
]

const recentProducts: MenuItem[] = [
  { label: '边缘安全加速平台 EO', path: '/shop' },
  { label: 'SSL 证书', path: '/support' },
  { label: '消息中心', path: '/profile/messages' },
  { label: 'ICP备案', path: '/support' },
  { label: '集团账号管理', path: '/profile' },
  { label: '人脸核身', path: '/profile' },
]

const productColumns = [
  [
    {
      title: '计算',
      groups: [
        {
          title: '计算',
          items: [
            { label: '云服务器', path: '/shop' },
            { label: '轻量应用服务器', path: '/shop' },
            { label: 'GPU 云服务器', path: '/shop' },
            { label: '裸金属云服务器', path: '/shop' },
            { label: '弹性伸缩', path: '/shop' },
          ],
        },
        {
          title: '高性能计算',
          items: [
            { label: '批量计算', path: '/shop' },
            { label: '高性能计算平台', path: '/shop' },
            { label: '高性能应用服务', path: '/shop' },
          ],
        },
        {
          title: '分布式云',
          items: [
            { label: '本地专用集群', path: '/shop' },
            { label: '专属可用区', path: '/shop' },
          ],
        },
      ],
    },
    {
      title: '人工智能与机器学习',
      groups: [
        {
          title: '腾讯智能体',
          items: [{ label: 'CodeBuddy', path: '/shop' }],
        },
        {
          title: '腾讯大模型',
          items: [
            { label: '大模型服务平台 TokenHub', path: '/shop' },
            { label: '腾讯云智能体开发平台', path: '/shop' },
          ],
        },
        {
          title: '视频服务',
          items: [
            { label: '云直播', path: '/shop' },
            { label: '云点播', path: '/shop' },
            { label: '实时音视频', path: '/shop' },
          ],
        },
      ],
    },
  ],
  [
    {
      title: '存储',
      groups: [
        {
          title: '基础存储服务',
          items: [
            { label: '对象存储', path: '/shop' },
            { label: '文件存储', path: '/shop' },
            { label: '归档存储', path: '/shop' },
            { label: '云 HDFS', path: '/shop' },
            { label: '云硬盘', path: '/shop' },
            { label: '数据加速器 GooseFS', path: '/shop' },
          ],
        },
        {
          title: '存储数据服务',
          items: [
            { label: '日志服务', path: '/shop' },
            { label: '数据万象', path: '/shop' },
            { label: '智能媒资托管', path: '/shop' },
          ],
        },
        {
          title: '数据迁移',
          items: [{ label: '云数据迁移', path: '/shop' }],
        },
        {
          title: '混合云存储',
          items: [{ label: '存储网关', path: '/shop' }],
        },
        {
          title: '智能存储',
          items: [{ label: '智能视图计算平台', path: '/shop' }],
        },
      ],
    },
    {
      title: 'CDN与边缘',
      groups: [
        {
          title: 'CDN与边缘',
          items: [
            { label: '内容分发网络 CDN', path: '/shop' },
            { label: '边缘安全加速平台 EO', path: '/shop' },
            { label: '全站加速网络 ECDN', path: '/shop' },
            { label: '边缘计算机器 ECM', path: '/shop' },
          ],
        },
      ],
    },
  ],
  [
    {
      title: '网络',
      groups: [
        {
          title: '云上网络',
          items: [
            { label: '负载均衡', path: '/shop' },
            { label: '私有网络', path: '/shop' },
            { label: '弹性网卡', path: '/shop' },
            { label: 'NAT 网关', path: '/shop' },
            { label: '弹性公网 IP', path: '/shop' },
            { label: '智能高性能网络', path: '/shop' },
          ],
        },
        {
          title: '混合云网络',
          items: [
            { label: '专线接入', path: '/shop' },
            { label: '云联网', path: '/shop' },
            { label: 'VPN 连接', path: '/shop' },
            { label: '全球应用加速', path: '/shop' },
            { label: 'SD-WAN 接入服务', path: '/shop' },
          ],
        },
      ],
    },
    {
      title: '容器与中间件',
      groups: [
        {
          title: '容器',
          items: [
            { label: '容器服务', path: '/shop' },
            { label: '容器镜像服务', path: '/shop' },
            { label: 'Agent Runtime', path: '/shop' },
          ],
        },
        {
          title: 'Serverless',
          items: [
            { label: '云函数 SCF', path: '/shop' },
            { label: 'Serverless 应用中心', path: '/shop' },
            { label: '弹性微服务', path: '/shop' },
          ],
        },
      ],
    },
  ],
]

const categoryIndex = [
  '计算',
  '人工智能与机器学习',
  '视频服务',
  '云通信与企业服务',
  '物联网',
  '云平台服务',
  '办公协同',
  '存储',
  'CDN与边缘',
  '数据库',
  '大数据',
  '行业应用',
  '服务与营销',
  '网络',
  '容器与中间件',
  '安全',
  '开发与运维',
  '基础',
]

const resourceGroups = [
  {
    title: '计算与存储',
    items: [
      { label: '我的云主机', path: '/cloud/instances' },
      { label: '镜像管理', path: '/cloud/images' },
      { label: '续费管理', path: '/cloud/renewals' },
    ],
  },
  {
    title: '费用与订单',
    items: [
      { label: '费用中心', path: '/billing' },
      { label: '余额与充值', path: '/billing/balance' },
      { label: '资金流水', path: '/billing/transactions' },
      { label: '支付方式', path: '/billing/payment-methods' },
      { label: '我的订单', path: '/order' },
    ],
  },
  {
    title: '支持与服务',
    items: [
      { label: '提交工单', path: '/support/tickets/create' },
      { label: '工单列表', path: '/support/tickets' },
      { label: '帮助文档', path: '/support' },
      { label: '个人中心', path: '/profile' },
    ],
  },
]

const recentPages: MenuItem[] = [
  { label: '控制台首页', path: '/dashboard' },
  { label: '我的云主机', path: '/cloud/instances' },
  { label: '购买云主机', path: '/shop' },
  { label: '费用中心', path: '/billing' },
  { label: '工单列表', path: '/support/tickets' },
  { label: '账户设置', path: '/profile' },
]

// ===== 行为 =====
function close() {
  emit('update:open', false)
}

function select(item: MenuItem) {
  if (item.path) {
    router.push(item.path)
    close()
    return
  }
  MessagePlugin.info(`${item.label} 开发中`)
}

function onSearch() {
  const q = keyword.value.trim()
  if (!q) return
  router.push({ path: '/shop', query: { keyword: q } })
  close()
}

function scrollToCategory(title: string) {
  activeCategory.value = title
  const el = scrollRef.value?.querySelector(`[data-cat="${title}"]`)
  el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      document.body.style.overflow = 'hidden'
      window.addEventListener('keydown', onKeydown)
    } else {
      document.body.style.overflow = ''
      window.removeEventListener('keydown', onKeydown)
    }
  },
)

onBeforeUnmount(() => {
  document.body.style.overflow = ''
  window.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.pm-root {
  position: fixed;
  inset: 0;
  z-index: 200;
}

.pm-backdrop {
  position: absolute;
  inset: 0;
  border: none;
  background: rgba(15, 23, 42, 0.45);
  cursor: default;
}

.pm-panel {
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  width: 50%;
  min-width: 640px;
  max-width: 920px;
  display: flex;
  background: #ffffff;
  box-shadow: 20px 0 50px rgba(15, 23, 42, 0.18);
}

/* ============ 左侧导航栏 ============ */
.pm-rail {
  width: 304px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  background: #1c1c1c;
  color: #d1d5db;
  padding: 16px 0 12px;
  overflow: hidden;
}

.pm-rail__nav {
  display: flex;
  flex-direction: column;
  padding: 0 12px;
  overflow-y: auto;
  min-height: 0;
}

.pm-rail__item,
.pm-rail__tools {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 12px 12px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #e5e7eb;
  font-size: 14px;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.pm-rail__item:hover,
.pm-rail__tools:hover {
  background: rgba(255, 255, 255, 0.08);
}

.pm-rail__item.is-active {
  background: rgba(255, 255, 255, 0.12);
  color: #ffffff;
}

.pm-rail__label {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pm-rail__arrow {
  color: #9ca3af;
  flex-shrink: 0;
}

.pm-rail__hint {
  position: relative;
  margin: 14px 4px 18px;
  padding-left: 18px;
  font-size: 12.5px;
  line-height: 1.8;
  color: #9ca3af;
}

.pm-rail__hint-icon {
  position: absolute;
  left: 0;
  top: 3px;
  color: #9ca3af;
}

.pm-rail__title {
  margin: 0 4px 10px;
  font-size: 14px;
  font-weight: 600;
  color: #f3f4f6;
}

.pm-rail__fire {
  font-size: 13px;
}

.pm-rail__link {
  border: none;
  background: transparent;
  text-align: left;
  padding: 8px 12px;
  border-radius: 6px;
  color: #cbd5e1;
  font-size: 13.5px;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.pm-rail__link:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #ffffff;
}

.pm-rail__footer {
  padding: 8px 12px 0;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  margin-top: 12px;
}

.pm-rail__tools-list {
  display: flex;
  flex-direction: column;
  padding: 4px 0 4px 8px;
}

/* ============ 中间内容 ============ */
.pm-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: #ffffff;
}

.pm-search {
  position: relative;
  flex-shrink: 0;
  padding: 22px 40px 14px;
  border-bottom: 1px solid #eef1f5;
}

.pm-search__input {
  width: 100%;
  height: 40px;
  border: none;
  border-bottom: 2px solid var(--td-brand-color, #0052d9);
  background: transparent;
  color: #1e293b;
  font-size: 15px;
  outline: none;
  padding: 0 30px 0 2px;
}

.pm-search__input::placeholder {
  color: #9aa8bc;
}

.pm-search__icon {
  position: absolute;
  right: 42px;
  top: 32px;
  color: #64748b;
  pointer-events: none;
}

.pm-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 24px 40px 48px;
}

.pm-block {
  margin-bottom: 28px;
}

.pm-block__title {
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.pm-recent {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px 24px;
}

.pm-recent__item {
  border: none;
  background: transparent;
  text-align: left;
  padding: 0;
  font-size: 13.5px;
  color: #334155;
  cursor: pointer;
  transition: color 0.15s ease;
}

.pm-recent__item:hover {
  color: var(--td-brand-color, #0052d9);
}

.pm-cols {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 40px;
}

.pm-col {
  display: flex;
  flex-direction: column;
}

.pm-cat {
  padding-bottom: 26px;
  margin-bottom: 26px;
  border-bottom: 1px solid #eef1f5;
}

.pm-cat:last-child {
  border-bottom: none;
  margin-bottom: 0;
}

.pm-cat__title {
  margin: 0 0 18px;
  padding-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--td-brand-color, #0052d9);
  border-bottom: 1px solid #dbeafe;
  display: inline-block;
}

.pm-group {
  margin-bottom: 20px;
}

.pm-group:last-child {
  margin-bottom: 0;
}

.pm-group__title {
  margin: 0 0 10px;
  font-size: 13.5px;
  font-weight: 600;
  color: #1e293b;
}

.pm-link {
  display: block;
  border: none;
  background: transparent;
  text-align: left;
  padding: 5px 0;
  font-size: 13px;
  color: #64748b;
  cursor: pointer;
  transition: color 0.15s ease;
}

.pm-link:hover {
  color: var(--td-brand-color, #0052d9);
}

/* ============ 右侧分类索引 ============ */
.pm-index {
  width: 220px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 36px 16px 24px;
  border-left: 1px solid #eef1f5;
  overflow-y: auto;
}

.pm-index__item {
  border: none;
  background: transparent;
  text-align: left;
  padding: 7px 10px;
  border-radius: 6px;
  font-size: 13.5px;
  color: #64748b;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.pm-index__item:hover,
.pm-index__item.is-active {
  background: #f1f5f9;
  color: var(--td-brand-color, #0052d9);
}

.pm-close {
  position: absolute;
  top: 20px;
  right: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #64748b;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.pm-close:hover {
  background: #f1f5f9;
  color: #1e293b;
}

/* ============ 动画 ============ */
.pm-open-enter-active,
.pm-open-leave-active {
  transition: opacity 0.2s ease;
}

.pm-open-enter-active .pm-panel,
.pm-open-leave-active .pm-panel {
  transition: transform 0.24s ease;
}

.pm-open-enter-from,
.pm-open-leave-to {
  opacity: 0;
}

.pm-open-enter-from .pm-panel,
.pm-open-leave-to .pm-panel {
  transform: translateX(-100%);
}

.pm-tools-enter-active,
.pm-tools-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.pm-tools-enter-from,
.pm-tools-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

/* ============ 深色模式 ============ */
.dark .pm-panel,
.dark .pm-main {
  background: #0a0a0a;
}

.dark .pm-search,
.dark .pm-cat,
.dark .pm-index {
  border-color: #262626;
}

.dark .pm-search__input {
  color: #e5e7eb;
}

.dark .pm-block__title,
.dark .pm-group__title {
  color: #e5e7eb;
}

.dark .pm-recent__item {
  color: #cbd5e1;
}

.dark .pm-link {
  color: #94a3b8;
}

.dark .pm-index__item:hover,
.dark .pm-index__item.is-active {
  background: #1f1f1f;
}

.dark .pm-close:hover {
  background: #1f1f1f;
  color: #e5e7eb;
}

/* ============ 响应式 ============ */
@media (max-width: 1280px) {
  .pm-panel {
    width: 60%;
    min-width: 0;
    max-width: 780px;
  }

  .pm-index {
    display: none;
  }

  .pm-cols {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .pm-panel {
    width: 88vw;
    min-width: 0;
    max-width: none;
  }

  .pm-rail {
    width: 40vw;
    min-width: 150px;
  }

  .pm-rail__item,
  .pm-rail__tools {
    font-size: 13px;
    padding: 11px 8px;
  }

  .pm-search {
    padding: 16px 16px 12px;
  }

  .pm-search__icon {
    right: 18px;
    top: 26px;
  }

  .pm-scroll {
    padding: 18px 16px 40px;
  }

  .pm-cols {
    grid-template-columns: minmax(0, 1fr);
  }

  .pm-recent {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .pm-close {
    top: 12px;
    right: 12px;
  }
}
</style>
