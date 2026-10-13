<template>
  <div class="page-body console-module shop-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CartIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">云主机选购</h2>
          <p class="page-header__desc">选择商品后配置规格与周期，加入购物车统一结算</p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/cart')">
          购物车
          <template #suffix v-if="cartStore.count > 0">({{ cartStore.count }})</template>
        </t-button>
      </div>
    </header>

    <FilterCard>
      <div class="field">
        <label class="field__label" for="shop-keyword">关键词</label>
        <t-input
          id="shop-keyword"
          v-model="keyword"
          placeholder="搜索商品名称"
          clearable
          @enter="search"
          @clear="search"
        />
      </div>
      <div class="field">
        <label class="field__label">只看推荐</label>
        <t-switch v-model="featuredOnly" @change="search" />
      </div>
      <template #actions>
        <t-button theme="primary" :loading="loading" @click="search">查询</t-button>
        <t-button variant="outline" @click="resetFilter">重置</t-button>
      </template>
    </FilterCard>

    <section v-loading="loading" class="shop-grid">
      <article v-for="p in products" :key="p.id" class="surface-card shop-card">
        <div class="shop-card__cover">
          <img v-if="p.cover_image" :src="p.cover_image" :alt="p.name" />
          <component :is="ServerIcon" v-else size="34" />
          <span v-if="p.featured" class="shop-card__flag">推荐</span>
        </div>

        <div class="shop-card__body">
          <h3 class="shop-card__name">{{ p.name }}</h3>
          <p class="shop-card__desc">{{ p.description || '暂无描述' }}</p>

          <div class="shop-card__meta">
            <t-tag size="small" variant="light">{{ sourceModeLabel(p.source_mode) }}</t-tag>
            <t-tag v-if="p.skus.length" size="small" variant="light" theme="primary">
              {{ p.skus.length }} 种规格
            </t-tag>
          </div>

          <div class="shop-card__foot">
            <div class="price-cell">
              <span class="shop-card__price">¥{{ p.price.toFixed(2) }}</span>
              <span class="shop-card__unit">{{ priceModelLabel(p.price_model) }}</span>
            </div>
            <t-button size="small" theme="primary" @click="openConfig(p)">购买</t-button>
          </div>
        </div>
      </article>

      <div v-if="!loading && !products.length" class="empty-state surface-card">
        <t-empty description="暂无可购买商品" />
      </div>
    </section>

    <t-pagination
      v-if="total > pagination.pageSize"
      v-model:current="pagination.current"
      v-model:page-size="pagination.pageSize"
      :total="total"
      :page-size-options="[12, 24, 48]"
      class="shop-pager"
      @change="loadProducts"
    />

    <!--
      下单链路唯一：列表页只负责「选商品」，配置选择与加购在 /shop/:id 完成，
      结算在购物车。这里不再保留弹窗式购买卡片，避免与购物车形成两个下单入口。
    -->
  </div>
</template>

<script setup lang="ts">
/**
 * 商品列表页：只负责浏览与筛选，以及把「购买」转成一次跳转。
 *
 * 规格 / 周期 / 配置项 / 数量全部在配置选购页（/shop/:id）里选，选完加入购物车，
 * 再在购物车统一结算 —— 下单入口只有一个，用户不会分不清「这次是下单还是加购」。
 */
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CartIcon, ServerIcon } from 'tdesign-icons-vue-next'

import { getProducts, type ProductInfo } from '@/api/shop'
import FilterCard from '@/components/filter-card/index.vue'
import { useCartStore } from '@/store/modules/cart'

defineOptions({ name: 'Shop' })

const route = useRoute()
const router = useRouter()
const cartStore = useCartStore()

// ========== 列表 ==========
const products = ref<ProductInfo[]>([])
const loading = ref(false)
const keyword = ref('')
const featuredOnly = ref(false)
const total = ref(0)
const pagination = ref({ current: 1, pageSize: 12 })

function sourceModeLabel(m: string): string {
  return m === 'upstream' ? '上游直供' : '自营'
}

function priceModelLabel(m: string): string {
  const map: Record<string, string> = {
    monthly: '/月',
    quarterly: '/季',
    annually: '/年',
    hourly: '/小时',
    fixed: '一次性',
    onetime: '一次性',
  }
  return map[m] || ''
}

/** 打开配置选购页：列表项不带 skus/cycles，详情与选配都在目标页拉取。 */
function openConfig(product: ProductInfo) {
  router.push({ name: 'ShopConfig', params: { id: product.id } })
}

// ========== 数据加载 ==========
async function loadProducts() {
  loading.value = true
  try {
    const { data } = await getProducts({
      keyword: keyword.value || undefined,
      featured: featuredOnly.value || undefined,
      page: pagination.value.current,
      page_size: pagination.value.pageSize,
    })
    products.value = data?.items || []
    total.value = data?.total || 0
  } catch {
    products.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function search() {
  pagination.value.current = 1
  loadProducts()
}

function resetFilter() {
  keyword.value = ''
  featuredOnly.value = false
  search()
}

/**
 * 消费官网意图：带着 spec/cycle/config 跳到配置选购页（预填）。
 *
 * 意图只用来「预填」，不直接下单 —— 支付金额必须由用户确认。
 */
async function consumeIntent() {
  const raw = route.query.product
  const productId = Number(Array.isArray(raw) ? raw[0] : raw)
  if (!productId || Number.isNaN(productId)) return

  const query: Record<string, string> = {}
  for (const key of ['spec', 'cycle', 'config'] as const) {
    const value = route.query[key]
    const text = String(Array.isArray(value) ? value[0] : value ?? '')
    if (text) query[key] = text
  }
  await router.replace({ name: 'ShopConfig', params: { id: productId }, query })
}

onMounted(async () => {
  // 有官网意图时直接进入配置页，列表加载与否都不影响这次跳转。
  if (route.query.product) {
    await consumeIntent()
    return
  }
  await loadProducts()
})
</script>

<style scoped>
/* ============ 商品网格 ============ */
.shop-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(272px, 1fr));
  gap: var(--space-lg);
  min-height: 160px;
}

.shop-grid .empty-state {
  grid-column: 1 / -1;
}

.shop-card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: transform var(--hs-duration-base) var(--hs-ease-out),
    box-shadow var(--hs-duration-base) var(--hs-ease-out);
}

.shop-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--hs-shadow-md);
}

.shop-card__cover {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 116px;
  background: linear-gradient(135deg, var(--hs-surface-3) 0%, var(--hs-surface-2) 100%);
  color: var(--color-muted-foreground);
  overflow: hidden;
}

.shop-card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.shop-card__flag {
  position: absolute;
  top: 8px;
  left: 8px;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--color-primary);
  color: #fff;
  font-size: 11px;
}

.shop-card__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-lg);
  flex: 1;
}

.shop-card__name {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--color-foreground);
}

.shop-card__desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 40px;
}

.shop-card__meta {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  flex-wrap: wrap;
}

.shop-card__foot {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-sm);
  margin-top: auto;
  padding-top: var(--space-sm);
}

.shop-card__price {
  font-size: 20px;
  font-weight: 700;
  color: var(--color-accent);
  font-variant-numeric: tabular-nums;
}

.shop-card__unit {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.shop-pager {
  justify-content: flex-end;
}

/* ============ 响应式 ============ */
@media (max-width: 768px) {
  .shop-grid {
    grid-template-columns: 1fr;
    gap: var(--space-md);
  }

}
</style>
