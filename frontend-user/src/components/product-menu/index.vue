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
              <template v-if="featuredProducts.length">
                <h4 class="pm-rail__title">热门产品</h4>
                <button
                  v-for="p in featuredProducts"
                  :key="p.id"
                  class="pm-rail__link"
                  @click="select({ label: p.name, path: `/shop?product=${p.id}` })"
                >
                  {{ p.name }}
                </button>
              </template>
              <p v-else class="pm-rail__hint">
                暂无标记为推荐的商品；运营可在管理端把商品设为推荐后在此展示。
              </p>
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
            <!-- 全部产品：按在售商品的品类分组 -->
            <template v-if="activeRail === 'products'">
              <div v-if="loading" class="pm-loading"><t-loading size="small" text="加载中…" /></div>

              <template v-else-if="productGroups.length">
                <section
                  v-for="group in productGroups"
                  :key="group.title"
                  class="pm-block"
                  :data-cat="group.title"
                >
                  <h3 class="pm-block__title">{{ group.title }}</h3>
                  <div class="pm-products">
                    <button
                      v-for="p in group.items"
                      :key="p.id"
                      class="pm-product"
                      @click="select({ label: p.name, path: `/shop?product=${p.id}` })"
                    >
                      <span class="pm-product__name">{{ p.name }}</span>
                      <span class="pm-product__price">{{ priceLabel(p) }}</span>
                      <span v-if="p.featured" class="pm-product__badge">推荐</span>
                    </button>
                  </div>
                </section>
              </template>

              <t-empty v-else description="暂无可售产品">
                <template #description>
                  <p class="pm-empty">暂无可售产品</p>
                </template>
              </t-empty>
            </template>

            <!-- 我的资源：来自控制台菜单树，与侧边栏同一份数据 -->
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
              <t-empty v-if="!resourceGroups.length" description="暂无可用资源入口" />
            </template>

            <!-- 最近访问页面：来自控制台访问记录 -->
            <template v-else>
              <section class="pm-block">
                <h3 class="pm-block__title">最近访问页面</h3>
                <div v-if="recentPages.length" class="pm-recent">
                  <button
                    v-for="item in recentPages"
                    :key="item.path"
                    class="pm-recent__item"
                    @click="select({ label: item.title, path: item.path })"
                  >
                    {{ item.title }}
                  </button>
                </div>
                <p v-else class="pm-empty">还没有访问记录，进入任一功能页后会自动记录。</p>
              </section>
            </template>
          </div>
        </section>

        <!-- ============ 右侧分类索引 ============ -->
        <aside v-if="activeRail === 'products' && productGroups.length" class="pm-index">
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
import { computed, onBeforeUnmount, ref, watch, type Component } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ChevronDownIcon,
  ChevronRightIcon,
  ChevronUpIcon,
  CloseIcon,
  GridViewIcon,
  SearchIcon,
  ServerIcon,
  TimeIcon,
  ToolsIcon,
} from 'tdesign-icons-vue-next'

import { getProducts, type ProductInfo } from '@/api/shop'
import { useMenuStore, type FlatMenu } from '@/store'
import { getRecentPages } from '@/utils/recent'
import { openSite, siteUrlConfigured } from '@/utils/site'

defineOptions({ name: 'UserProductMenu' })

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const router = useRouter()
const route = useRoute()
const menuStore = useMenuStore()

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

interface MenuItem {
  label: string
  path?: string
  /** 门户链接：点击后新窗口打开官网 */
  sitePath?: string
}

// ===== 在售商品（真实目录，按品类分组） =====
// 原先这里是照搬某云厂商的 18 类产品导航（含 JoyAgent/CodeBuddy 等平台根本不卖的东西），
// 点进去一律落到 /shop。现在只展示运营在管理端上架的商品，品类标签取 product_type。
const products = ref<ProductInfo[]>([])
const loading = ref(false)
const loaded = ref(false)

/** 品类编码 → 展示名。管理端新增品类时在这里补一行，未命中的编码原样展示。 */
const PRODUCT_TYPE_LABELS: Record<string, string> = {
  cloud_host: '云主机',
  cloud_disk: '云硬盘',
  bandwidth: '带宽',
  other: '其他产品',
}

async function loadProducts() {
  if (loaded.value) return
  loading.value = true
  try {
    const { data } = await getProducts({ page: 1, page_size: 100 })
    products.value = data?.items ?? []
    loaded.value = true
  } catch {
    products.value = []
  } finally {
    loading.value = false
  }
}

const productGroups = computed(() => {
  const groups = new Map<string, ProductInfo[]>()
  for (const p of products.value) {
    const key = PRODUCT_TYPE_LABELS[p.product_type] || p.product_type || '其他产品'
    const list = groups.get(key) ?? []
    list.push(p)
    groups.set(key, list)
  }
  return [...groups.entries()].map(([title, items]) => ({ title, items }))
})

const categoryIndex = computed(() => productGroups.value.map((g) => g.title))

const featuredProducts = computed(() => products.value.filter((p) => p.featured))

function priceLabel(p: ProductInfo): string {
  const unit: Record<string, string> = {
    monthly: '元/月',
    quarterly: '元/季',
    annually: '元/年',
    onetime: '元',
  }
  return `${p.price} ${unit[p.price_model] || '元'}`
}

// ===== 我的资源：直接用控制台菜单树，与左侧边栏同源，不重复维护一份 =====
const resourceGroups = computed(() => {
  const buckets: Array<{ title: string; items: MenuItem[] }> = []

  // 一级「云产品」目录的子菜单（我的云主机 / 镜像管理 / 续费管理）
  const cloudDir = menuStore.menus.find((m) => m.path === '/cloud')
  if (cloudDir?.children?.length) {
    buckets.push({
      title: cloudDir.name,
      items: cloudDir.children
        .filter((c) => !!c.path)
        .map((c) => ({ label: c.name, path: c.path })),
    })
  }

  // 资源之外，把交易与费用、工单等一级入口也列出来，方便从菜单直接进
  const directPaths = ['/shop', '/order', '/billing', '/points']
  const directMenus = flattenMenus(menuStore.menus).filter(
    (m) => m.path && directPaths.includes(m.path),
  )
  if (directMenus.length) {
    buckets.push({
      title: '交易与费用',
      items: directMenus.map((m) => ({ label: m.name, path: m.path })),
    })
  }

  return buckets.filter((b) => b.items.length > 0)
})

function flattenMenus(nodes: FlatMenu[]): FlatMenu[] {
  const out: FlatMenu[] = []
  for (const node of nodes) {
    out.push(node)
    if (node.children?.length) out.push(...flattenMenus(node.children))
  }
  return out
}

// ===== 最近访问：来自布局层统一记录的本地记录 =====
const recentPages = computed(() => (props.open ? getRecentPages() : []))

// ===== 平台工具：全部指向可到达的真实落点 =====
const platformTools = computed<MenuItem[]>(() => {
  const items: MenuItem[] = [
    { label: '云主机选购', path: '/shop' },
    { label: '费用中心', path: '/billing' },
    { label: '提交工单', path: '/support/tickets/create' },
    { label: '工单列表', path: '/support/tickets' },
    { label: '实名认证', path: '/profile' },
    { label: '安全设置', path: '/profile/security' },
  ]
  // 帮助文档只在官网地址已配置时出现（未配置时点了只能报错）
  if (siteUrlConfigured) items.push({ label: '帮助文档', sitePath: '/help' })
  return items
})

// ===== 行为 =====
function close() {
  emit('update:open', false)
}

function select(item: MenuItem) {
  if (item.sitePath) {
    openSite(item.sitePath)
    close()
    return
  }
  if (item.path) {
    router.push(item.path)
    close()
    return
  }
  // 没有落地目标就不渲染成可点项 —— 见模板里对 path/sitePath 的筛选
  close()
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
  async (open) => {
    if (open) {
      document.body.style.overflow = 'hidden'
      window.addEventListener('keydown', onKeydown)
      await loadProducts()
      // 每次打开时若当前路由有品类，滚到对应区块
      activeCategory.value = categoryIndex.value[0] ?? ''
      return
    }
    document.body.style.overflow = ''
    window.removeEventListener('keydown', onKeydown)
  },
)

// 路由变化时清掉搜索词：从店铺页回到控制台再打开菜单，不该还留着上次的关键字
watch(
  () => route.path,
  () => {
    keyword.value = ''
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
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  width: 200px;
  flex-shrink: 0;
  padding: 18px 12px 14px;
  background: #f7f8fa;
  border-right: 1px solid #eef1f5;
  overflow-y: auto;
}

.pm-rail__item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  margin-bottom: 4px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #475569;
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.pm-rail__item:hover {
  background: #eef2f7;
  color: #1e293b;
}

.pm-rail__item.is-active {
  background: #ffffff;
  color: var(--color-primary);
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06);
}

.pm-rail__label {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pm-rail__arrow {
  color: #b6c2d4;
  flex-shrink: 0;
}

.pm-rail__hint {
  display: flex;
  gap: 6px;
  margin: 14px 6px 0;
  font-size: 11.5px;
  line-height: 1.7;
  color: #94a3b8;
}

.pm-rail__hint-icon {
  flex-shrink: 0;
  margin-top: 2px;
  color: #fbbf24;
}

.pm-rail__title {
  margin: 18px 6px 8px;
  font-size: 12.5px;
  font-weight: 600;
  color: #64748b;
}

.pm-rail__fire {
  font-size: 11px;
}

.pm-rail__link {
  display: block;
  width: 100%;
  padding: 7px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #475569;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.pm-rail__link:hover {
  background: #eef2f7;
  color: var(--color-primary);
}

.pm-rail__tools {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #475569;
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
}

.pm-rail__tools:hover {
  background: #eef2f7;
  color: #1e293b;
}

.pm-rail__tools-list {
  padding-left: 8px;
}

/* ============ 中间内容 ============ */
.pm-main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.pm-search {
  position: relative;
  display: flex;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #eef1f5;
}

.pm-search__input {
  width: 100%;
  height: 36px;
  padding: 0 40px 0 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
  color: #1e293b;
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s ease, background-color 0.15s ease;
}

.pm-search__input::placeholder {
  color: #9aa8c4;
}

.pm-search__input:focus {
  border-color: var(--color-primary);
  background: #ffffff;
}

.pm-search__icon {
  position: absolute;
  right: 34px;
  color: #94a3b8;
  pointer-events: none;
}

.pm-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 18px 20px 24px;
}

.pm-loading {
  display: flex;
  justify-content: center;
  padding: 40px 0;
}

.pm-block {
  margin-bottom: 22px;
}

.pm-block__title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.pm-products {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.pm-product {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid #e8ecf2;
  border-radius: 8px;
  background: #ffffff;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.pm-product:hover {
  border-color: #bfdbfe;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.1);
}

.pm-product__name {
  font-size: 13px;
  color: #334155;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pm-product__price {
  flex-shrink: 0;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--color-primary);
}

.pm-product__badge {
  position: absolute;
  top: -7px;
  right: 8px;
  padding: 0 6px;
  border-radius: 3px;
  background: #eef2ff;
  border: 1px solid #e0e7ff;
  color: #4f46e5;
  font-size: 10.5px;
  line-height: 1.6;
}

.pm-recent {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.pm-recent__item {
  padding: 9px 16px;
  border: 1px solid #e8ecf2;
  border-radius: 8px;
  background: #ffffff;
  color: #334155;
  font-size: 13px;
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
}

.pm-recent__item:hover {
  color: var(--color-primary);
  border-color: #bfdbfe;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.1);
}

.pm-empty {
  margin: 0;
  font-size: 13px;
  color: #94a3b8;
}

/* ============ 右侧分类索引 ============ */
.pm-index {
  width: 132px;
  flex-shrink: 0;
  padding: 18px 10px;
  border-left: 1px solid #eef1f5;
  overflow-y: auto;
}

.pm-index__item {
  display: block;
  width: 100%;
  padding: 7px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #64748b;
  font-size: 12.5px;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.pm-index__item:hover {
  background: #f1f5f9;
  color: #1e293b;
}

.pm-index__item.is-active {
  background: #eff6ff;
  color: var(--color-primary);
  font-weight: 600;
}

.pm-close {
  position: absolute;
  top: 14px;
  right: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.pm-close:hover {
  color: #475569;
  background: #f1f5f9;
}

/* ============ 过渡 ============ */
.pm-open-enter-active,
.pm-open-leave-active {
  transition: opacity 0.2s ease;
}

.pm-open-enter-from,
.pm-open-leave-to {
  opacity: 0;
}

.pm-open-enter-active .pm-panel,
.pm-open-leave-active .pm-panel {
  transition: transform 0.24s ease;
}

.pm-open-enter-from .pm-panel,
.pm-open-leave-to .pm-panel {
  transform: translateX(-24px);
}

.pm-tools-enter-active,
.pm-tools-leave-active {
  transition: opacity 0.15s ease;
}

.pm-tools-enter-from,
.pm-tools-leave-to {
  opacity: 0;
}

/* ============ 响应式 ============ */
@media (max-width: 900px) {
  .pm-panel {
    width: 100%;
    min-width: 0;
    max-width: none;
  }

  .pm-index {
    display: none;
  }
}

@media (max-width: 640px) {
  .pm-rail {
    width: 128px;
    padding: 14px 8px;
  }

  .pm-products {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
