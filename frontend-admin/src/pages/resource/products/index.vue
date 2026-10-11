<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">商品列表</h2>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadProducts">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <FilterCard :columns="3">
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" placeholder="商品名称 / 规格" clearable />
      </div>
      <div class="field">
        <span class="field__label">所属提供商</span>
        <t-select v-model="filters.provider_id" clearable placeholder="全部提供商" :options="providerOptions" />
      </div>
      <div class="field">
        <span class="field__label">状态</span>
        <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
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
        <h3 class="card-title">商品列表</h3>
        <span class="table-card__meta">共 {{ total }} 个商品</span>
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
            <span class="product-name">{{ row.name }}</span>
            <span class="product-sub">{{ row.region || row.zone ? `${row.region}${row.zone ? ' / ' + row.zone : ''}` : '' }}</span>
          </div>
        </template>

        <template #group="{ row }">
          <t-tag v-if="row.group_name" theme="primary" variant="light" size="small" shape="round">{{ row.group_name }}</t-tag>
          <span v-else class="muted">未分类</span>
        </template>

        <template #specs="{ row }">
          <span class="spec-text">{{ row.cpu }}核 / {{ formatMemory(row.memory) }} / {{ formatDisk(row.disk) }} {{ row.bandwidth }}Mbps</span>
        </template>

        <template #os="{ row }">
          <span>{{ row.os || '—' }}</span>
        </template>

        <template #price="{ row }">
          <div class="price-cell">
            <span class="price-sale">标价 ¥{{ formatPrice(row.sale_price) }}</span>
            <span class="price-cost">
              代理价 ¥{{ formatPrice(row.cost_price) }}
              <em v-if="row.sale_price > row.cost_price && row.sale_price > 0" class="price-off">
                {{ Math.round((row.cost_price / row.sale_price) * 100) }}%
              </em>
            </span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 1 ? '上架中' : '已下架' }}
          </t-tag>
        </template>

        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail', theme: 'default' },
                { content: '改价', value: 'price', theme: 'default' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
              <t-link theme="primary" hover="color" @click="openPriceDialog(row)">改价</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无商品数据，请先执行商品同步" />
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

    <t-dialog
      v-model:visible="detailVisible"
      :header="detailProduct ? `商品详情 · ${detailProduct.name}` : '商品详情'"
      width="640px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailProduct" :column="2" bordered size="small">
        <t-descriptions-item label="商品名称">{{ detailProduct.name }}</t-descriptions-item>
        <t-descriptions-item label="上游 ID">{{ detailProduct.upstream_id || '—' }}</t-descriptions-item>
        <t-descriptions-item label="上游分组">
          <t-tag v-if="detailProduct.group_name" theme="primary" variant="light" size="small" shape="round">{{ detailProduct.group_name }}</t-tag>
          <span v-else>未分类</span>
        </t-descriptions-item>
        <t-descriptions-item label="规格">{{ detailProduct.cpu }}核 / {{ formatMemory(detailProduct.memory) }} / {{ formatDisk(detailProduct.disk) }} {{ detailProduct.bandwidth }}Mbps</t-descriptions-item>
        <t-descriptions-item label="磁盘类型">{{ detailProduct.disk_type || '—' }}</t-descriptions-item>
        <t-descriptions-item label="操作系统">{{ detailProduct.os || '—' }}</t-descriptions-item>
        <t-descriptions-item label="区域/可用区">{{ detailProduct.region }}{{ detailProduct.zone ? ' / ' + detailProduct.zone : '' }}</t-descriptions-item>
        <t-descriptions-item label="上游标价">¥{{ formatPrice(detailProduct.sale_price) }}</t-descriptions-item>
        <t-descriptions-item label="代理价（采购价）">¥{{ formatPrice(detailProduct.cost_price) }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="detailProduct.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ detailProduct.status === 1 ? '上架中' : '已下架' }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="更新时间">{{ formatTime(detailProduct.updated_at) }}</t-descriptions-item>
        <t-descriptions-item label="规格明细" :span="2">
          <pre class="raw-pre">{{ detailProduct.specs || '—' }}</pre>
        </t-descriptions-item>
        <t-descriptions-item label="上游原始快照（config_groups / 双价周期 / 描述）" :span="2">
          <pre class="raw-pre">{{ detailProduct.raw_specs || '—' }}</pre>
        </t-descriptions-item>
      </t-descriptions>
    </t-dialog>

    <t-dialog
      v-model:visible="priceVisible"
      header="编辑价格"
      width="460px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSavePrice"
      @close="priceVisible = false"
    >
      <t-form label-align="top" :data="priceForm" @submit.prevent>
        <t-form-item label="上游标价（元）" name="sale_price">
          <t-input-number v-model="priceForm.sale_price" :min="0" :precision="2" theme="column" placeholder="上游公示价" />
        </t-form-item>
        <t-form-item label="代理价（采购价，元）" name="cost_price">
          <t-input-number v-model="priceForm.cost_price" :min="0" :precision="2" theme="column" placeholder="上游标价扣除代理折扣后的采购价" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { onMounted, reactive, ref } from 'vue'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProductDetail, getProductList, getProviderList, updateProductPrice } from '@/api/admin'
import type { ProductInfo, ProviderInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ResourceProducts' })

const productList = ref<ProductInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])
const statusOptions = [
  { label: '上架中', value: 1 },
  { label: '已下架', value: 0 },
]

const filters = reactive<{ keyword: string | undefined; provider_id: number | undefined; status: number | undefined }>({
  keyword: undefined,
  provider_id: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
function formatMemory(value: number): string {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function formatDisk(value: number): string {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const columns: PrimaryTableCol<ProductInfo>[] = [
  { colKey: 'name', title: '商品', minWidth: 180 },
  { colKey: 'group', title: '分类', width: 130 },
  { colKey: 'specs', title: '规格', minWidth: 200 },
  { colKey: 'os', title: '系统', width: 100 },
  { colKey: 'price', title: '价格', width: 150 },
  { colKey: 'status', title: '状态', width: 100 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 130,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadProducts() {
  loading.value = true
  try {
    const data = await getProductList({
      keyword: filters.keyword,
      provider_id: filters.provider_id,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    productList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载商品列表失败')
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 提供商下拉加载失败不阻塞列表 */
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadProducts()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}

function handleSearch() {
  pagination.current = 1
  loadProducts()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.provider_id = undefined
  filters.status = undefined
  pagination.current = 1
  loadProducts()
}

const detailVisible = ref(false)
const detailProduct = ref<ProductInfo | null>(null)
const detailLoading = ref(false)

/**
 * 详情弹窗按需拉单个商品：列表接口已不带 raw_specs（单条 10KB 量级，整表会放大到 MB），
 * 上游原始快照只有详情才需要。
 */
async function openDetailDialog(row: ProductInfo) {
  detailProduct.value = row
  detailVisible.value = true
  detailLoading.value = true
  try {
    detailProduct.value = await getProductDetail(row.id)
  } catch {
    // 详情拉取失败时保留列表行数据（仍能看基础字段），不打断查看。
  } finally {
    detailLoading.value = false
  }
}

const priceVisible = ref(false)
const priceForm = reactive<{ cost_price: number; sale_price: number }>({ cost_price: 0, sale_price: 0 })

function openPriceDialog(row: ProductInfo) {
  priceForm.cost_price = row.cost_price
  priceForm.sale_price = row.sale_price
  priceVisible.value = true
}

async function handleSavePrice() {
  if (!detailProduct.value) return
  try {
    await updateProductPrice(detailProduct.value.id, {
      cost_price: priceForm.cost_price,
      sale_price: priceForm.sale_price,
    })
    MessagePlugin.success('价格已更新')
    priceVisible.value = false
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新价格失败')
  }
}

onMounted(() => {
  loadProviders()
  loadProducts()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: ProductInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetailDialog(row)
      break
    case 'price':
      openPriceDialog(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">

/* 桌面列数由 FilterCard 的 columns 属性给出3 列，见组件内 .filter-card--cols-*。 */

.product-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.product-name {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.product-sub {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.spec-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.price-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.price-sale {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.price-cost {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* 代理折扣标签：代理价 ÷ 标价的百分比，一眼看出上游给的折扣力度 */
.price-off {
  margin-left: 4px;
  padding: 0 5px;
  border-radius: 8px;
  background: rgba(43, 164, 113, 0.12);
  color: #2ba471;
  font-size: 11px;
  font-style: normal;
}

.raw-pre {
  margin: 0;
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
  line-height: 1.5;
}
</style>
