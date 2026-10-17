<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">商品调价</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadProducts">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" placeholder="产品名称 / 商品编码" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">分类</span>
        <t-select v-model="filters.category_id" clearable placeholder="全部分类" :options="categoryOptions" />
      </div>
      <div class="field">
        <span class="field__label">状态</span>
        <t-select v-model="filters.status" clearable placeholder="全部状态" :options="productStatusOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">价格与状态</h3>
        <span class="table-card__meta">共 {{ pagination.total }} 个产品</span>
      </div>
      <t-table
        row-key="id"
        :data="productList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="product-sub">SKU: {{ row.code }}</span>
          </div>
        </template>

        <template #category="{ row }">
          <span>{{ categoryName(row.category_id) }}</span>
        </template>

        <template #price="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.price) }}</span>
            <span class="price-sub">{{ priceModelLabel(row.price_model) }}</span>
          </div>
        </template>

        <template #cost="{ row }">
          <span class="cell-muted">¥{{ formatPrice(row.cost_price) }}</span>
        </template>

        <template #margin="{ row }">
          <span class="margin-cell">{{ marginOf(row) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTag(row.status).theme" variant="light" size="small" shape="round">
            {{ statusTag(row.status).text }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="
                buildMobileActionOptions([
                  { content: '改价', value: 'price', theme: 'default' },
                  { content: '上架', value: 'publish', hidden: () => row.status === 1, theme: 'success' },
                  { content: '下架', value: 'unpublish', hidden: () => row.status !== 1, theme: 'warning' },
                ])
              "
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openPriceDialog(row)">改价</t-link>
              <t-link v-if="row.status !== 1" theme="primary" hover="color" @click="handlePublish(row)">上架</t-link>
              <t-link v-else theme="warning" hover="color" @click="handleUnpublish(row)">下架</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无产品数据" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <!--
      调价弹窗（自营 / 上游分链路）：
        自营 → 周期价格矩阵（含按 SKU 定价）+ 可选配置项定价，两页签；
        上游 → 复用 UpstreamPriceEditor（加价规则 / 逐周期定价）。

      为什么换掉原来的单价表单（用户反馈「调价功能没完善，还是老的接口」）：
      原弹窗只有「销售价 / 成本价」两个输入，走 products 表的老接口 updateProductPrice ——
      它改的是商品级展示价，与客户真正下单用的 product_prices 周期矩阵**不是一份数据**：
      改完列表上的价变了，客户下单还是老价格；套餐价、各配置项价、周期价一个都改不了。
      现在直接挂权威编辑器，保存即落矩阵与配置项，列表价从矩阵回读。
    -->
    <t-dialog
      v-model:visible="priceVisible"
      :header="priceDialogTitle"
      :width="isMobile ? '94vw' : 'min(1100px, 92vw)'"
      :footer="false"
      @close="priceVisible = false"
    >
      <div v-if="priceProduct" class="price-dialog">
        <t-tabs v-model="priceTab" theme="normal">
          <t-tab-panel value="matrix" :label="isUpstreamProduct ? '加价与周期价' : '周期价格（套餐价）'">
            <UpstreamPriceEditor
              v-if="isUpstreamProduct"
              ref="upstreamPriceRef"
              embedded
              :product-id="priceProduct.id"
              :product-name="priceProduct.name"
              :source-product-id="priceProduct.source_product_id"
              :upstream="priceProduct.upstream || null"
              :initial-markup-type="priceProduct.upstream_markup_type || ''"
              :initial-markup-value="priceProduct.upstream_markup_value || 0"
            />
            <template v-else>
              <!-- 自营可按 SKU 分别定价：其价格优先于商品级（与商品详情页同一口径）。 -->
              <div class="price-scope">
                <span class="price-scope__label">定价范围</span>
                <t-select
                  v-model="priceSpecId"
                  class="price-scope__select"
                  :options="priceSpecOptions"
                  :disabled="!priceSpecOptions.length"
                  @change="reloadPriceMatrix"
                />
                <span class="price-scope__hint">
                  选具体规格则只维护该 SKU 的周期价（优先于商品级）；未拆 SKU 的商品只有商品级。
                </span>
              </div>
              <PriceMatrixEditor ref="matrixRef" :product-id="priceProduct.id" :spec-id="priceSpecId" />
            </template>
          </t-tab-panel>

          <t-tab-panel v-if="!isUpstreamProduct" value="options" label="配置项加价">
            <!--
              配置项加价是"套餐价之外的部分"：客户把内存从基线升到 8G、
              把数据盘从 50G 调到 100G，超出的钱在这里维护。
              与周期矩阵一样，改的是下单真正读取的数据。
            -->
            <ConfigOptionsEditor ref="configRef" :product-id="priceProduct.id" />
          </t-tab-panel>
        </t-tabs>

        <div class="price-dialog__footer">
          <span v-if="priceDirtyHint" class="price-dialog__hint">{{ priceDirtyHint }}</span>
          <t-button variant="outline" @click="priceVisible = false">取消</t-button>
          <t-button theme="primary" :loading="priceSaving" @click="handleSaveAllPrices">保存</t-button>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref } from 'vue'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProductList, getProductDetail, getProductSpecs, publishProduct, unpublishProduct } from '@/api/product'
import ConfigOptionsEditor from '@/pages/product/products/ConfigOptionsEditor.vue'
import PriceMatrixEditor from '@/pages/product/products/PriceMatrixEditor.vue'
import UpstreamPriceEditor from '@/pages/product/products/UpstreamPriceEditor.vue'
import { formatPrice, priceModelLabel, productStatusOptions, statusTag } from '@/pages/product/constants'
import type { SaleProductInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { useMobilePagination } from '@/composables/useMobilePagination'

defineOptions({ name: 'ProductPricing' })

const productList = ref<SaleProductInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const { categoryOptions, categoryName, loadCategories } = useCategoryOptions()

const filters = reactive<{ keyword: string | undefined; category_id: number | undefined; status: number | undefined }>({
  keyword: undefined,
  category_id: undefined,
  status: undefined,
})

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(loadProducts)

const columns: PrimaryTableCol<SaleProductInfo>[] = [
  { colKey: 'name', title: '产品', minWidth: 180 },
  { colKey: 'category', title: '分类', width: 120 },
  { colKey: 'price', title: '销售价', width: 140 },
  { colKey: 'cost', title: '成本价', width: 100 },
  { colKey: 'margin', title: '毛利', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 160,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

function marginOf(row: SaleProductInfo): string {
  const margin = row.price - row.cost_price
  const ratio = row.price > 0 ? ((margin / row.price) * 100).toFixed(1) : '0.0'
  return `¥${formatPrice(margin)} (${ratio}%)`
}

async function loadProducts() {
  loading.value = true
  try {
    const data = await getProductList({
      keyword: filters.keyword,
      category_id: filters.category_id,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    productList.value = data.items
    applyTotal(data.meta.total)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载产品列表失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  resetPage()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.category_id = undefined
  filters.status = undefined
  resetPage()
}

async function handlePublish(row: SaleProductInfo) {
  try {
    await publishProduct(row.id)
    MessagePlugin.success('已上架')
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '上架失败')
  }
}

async function handleUnpublish(row: SaleProductInfo) {
  try {
    await unpublishProduct(row.id)
    MessagePlugin.success('已下架')
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '下架失败')
  }
}

// ===== 调价弹窗 =====
const priceVisible = ref(false)
const priceSaving = ref(false)
const priceTab = ref<'matrix' | 'options'>('matrix')
/** 弹窗内的商品（进弹窗时重新拉详情：列表行缺 upstream/config 等字段）。 */
const priceProduct = ref<SaleProductInfo | null>(null)
const priceSpecId = ref<number | undefined>(undefined)
const priceSpecOptions = ref<{ label: string; value: number | undefined }[]>([])
const matrixRef = ref<{ save: () => Promise<boolean>; reload: () => void } | null>(null)
const configRef = ref<{ save: () => Promise<boolean> } | null>(null)
const upstreamPriceRef = ref<{ save: () => Promise<boolean> } | null>(null)

const isUpstreamProduct = computed(() => priceProduct.value?.source_mode === 'upstream')
const priceDialogTitle = computed(() =>
  priceProduct.value ? `调价 · ${priceProduct.value.name}` : '调价',
)
const priceDirtyHint = computed(() =>
  isUpstreamProduct.value
    ? '加价规则与逐周期价都会立即作用于客户下单价。'
    : '周期价与配置项加价分两个页签，保存时一并提交。',
)

/**
 * 打开调价弹窗。
 *
 * 为什么要重新拉详情而不是直接用列表行：列表接口只回商品级展示价，
 * 而调价要编辑的是周期矩阵与配置项 —— 需要 source_mode / upstream 等字段，
 * 直接复用行数据会拿到 undefined，弹窗按错误链路渲染。
 */
async function openPriceDialog(row: SaleProductInfo) {
  priceTab.value = 'matrix'
  priceSpecId.value = undefined
  priceSpecOptions.value = []
  priceProduct.value = null
  priceVisible.value = true
  try {
    const detail = await getProductDetail(row.id)
    priceProduct.value = detail
    if (detail.source_mode !== 'upstream') {
      await loadPriceSpecs(detail.id)
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载商品详情失败')
    priceVisible.value = false
  }
}

/** 定价范围选项：商品级（不区分规格）+ 各启用 SKU。 */
async function loadPriceSpecs(productId: number) {
  try {
    const specs = await getProductSpecs(productId)
    const enabled = specs.filter((s) => s.status === 1)
    priceSpecOptions.value = [
      { label: '商品级（所有规格共用）', value: undefined },
      ...enabled.map((s) => ({ label: `${s.name}（${s.spec_code}）`, value: s.id })),
    ]
  } catch {
    priceSpecOptions.value = [{ label: '商品级（所有规格共用）', value: undefined }]
  }
}

function reloadPriceMatrix() {
  matrixRef.value?.reload()
}

/**
 * 保存调价：按「周期矩阵 → 配置项」顺序落库，任一失败即停并说明已存了什么。
 *
 * 为什么串行而不是并发：两者都写 product_config_options 相关数据，
 * 并发时失败的那一半会让人分不清"到底存了没"。串行 + 明确提示，
 * 运营知道下一步该重试哪个页签。
 */
async function handleSaveAllPrices() {
  if (priceSaving.value) return
  priceSaving.value = true
  try {
    if (isUpstreamProduct.value) {
      const ok = await upstreamPriceRef.value?.save()
      if (ok === false) return
      MessagePlugin.success('价格已保存')
      priceVisible.value = false
      loadProducts()
      return
    }
    const matrixOK = await matrixRef.value?.save()
    if (matrixOK === false) {
      MessagePlugin.error('周期价格未保存成功，配置项加价未提交')
      return
    }
    const configOK = await configRef.value?.save()
    if (configOK === false) {
      MessagePlugin.warning('周期价格已保存，但配置项加价未保存成功；请修正后再试')
      return
    }
    MessagePlugin.success('周期价格与配置项加价已保存')
    priceVisible.value = false
    loadProducts()
  } finally {
    priceSaving.value = false
  }
}

onMounted(() => {
  loadCategories()
  loadProducts()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, unknown>, row: SaleProductInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'price':
      openPriceDialog(row)
      break
    case 'publish':
      void handlePublish(row)
      break
    case 'unpublish':
      void handleUnpublish(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style lang="css" scoped>
.margin-cell {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-primary);
  font-variant-numeric: tabular-nums;
}

/* 调价弹窗：页签内容可滚动，底部保存条常驻 */
.price-dialog {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 72vh;
}

.price-dialog :deep(.t-tabs__content) {
  overflow-y: auto;
  max-height: calc(72vh - 96px);
  padding-right: 4px;
}

.price-dialog__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--td-component-stroke, #e7e7e7);
}

.price-dialog__hint {
  margin-right: auto;
  font-size: 12px;
  color: var(--color-muted-foreground, #6b7280);
}

/* 定价范围选择：与商品详情页同一形态，避免两处口径看起来不一样 */
.price-scope {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.price-scope__label {
  font-size: 13px;
  color: var(--td-text-color-primary, #333);
}

.price-scope__select {
  width: 260px;
}

.price-scope__hint {
  font-size: 12px;
  color: var(--color-muted-foreground, #6b7280);
}
</style>
