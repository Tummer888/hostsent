<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">上游商品</h2>
          <p class="page-header__sub">
            上游供应商的商品镜像：按分类/服务商查看，标价与代理价（采购价）双口径，可设加价规则同步到已导入商品。
            状态分两列：「上游状态」= 上游商铺是否展示，「本地导入」= 我方是否已导入。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="goImportPage">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          导入为商品
        </t-button>
      </t-space>
    </header>

    <div class="resource-products">
      <aside class="resource-products__aside surface-card">
        <div class="aside-head">
          <span>商品分类</span>
          <span class="aside-head__count">{{ navTotal }}</span>
        </div>
        <ul class="aside-list">
          <li
            class="aside-item"
            :class="{ 'aside-item--active': filters.group_id === undefined }"
            @click="selectGroup(undefined)"
          >
            <span class="aside-item__name">全部商品</span>
            <span class="aside-item__meta">{{ navStats.total }}</span>
          </li>
          <li
            v-for="group in navGroups"
            :key="group.group_id"
            class="aside-item"
            :class="{ 'aside-item--active': filters.group_id === group.group_id }"
            @click="selectGroup(group.group_id)"
          >
            <span class="aside-item__name">
              {{ group.group_name || `分组 #${group.group_id}` }}
              <t-tag v-if="group.group_name_derived" size="small" variant="light-outline" theme="warning">派生</t-tag>
            </span>
            <span class="aside-item__meta">
              在售 {{ group.on_sale }}/{{ group.total }}
            </span>
          </li>
        </ul>
      </aside>

      <div class="resource-products__main">
        <FilterCard :columns="3">
          <div class="field">
            <span class="field__label">关键词</span>
            <t-input v-model="filters.keyword" placeholder="商品名称 / 规格" clearable />
          </div>
          <div class="field">
            <span class="field__label">服务商</span>
            <t-select v-model="filters.provider_id" clearable placeholder="全部服务商" :options="providerOptions" @change="handleProviderChange" />
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
            <h3 class="card-title">
              {{ activeGroupName }}
            </h3>
            <span class="table-card__meta">
              共 {{ total }} 个商品 · 上游正常 {{ onSaleInPage }} 个（本页）
              <template v-if="importedInPage"> · 已导入 {{ importedInPage }} 个（本页）</template>
            </span>
          </div>
          <p class="table-card__hint">
            这是两个独立维度：<b>上游状态</b>看上游商铺是否展示（正常 / 隐藏；悬停可看是分类隐藏、商品隐藏还是已下架，
            隐藏的商品仍可导入与开通）；<b>本地导入</b>看我方是否已导入成本地商品。
            「上游正常 + 未导入」是正常状态，点「未导入 · 去导入」可直接导入。
          </p>
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
                <span class="product-sub">上游 ID {{ row.upstream_id || '—' }}</span>
              </div>
            </template>

            <template #group="{ row }">
              <span class="group-cell">
                <t-tag theme="primary" variant="light" size="small" shape="round">
                  {{ row.group_name || `分组 #${row.group_id}` }}
                </t-tag>
                <t-tag v-if="row.group_name_derived" size="small" variant="light-outline" theme="warning">派生</t-tag>
              </span>
            </template>

            <template #provider="{ row }">
              <span>{{ providerName(row.provider_id) }}</span>
            </template>

            <template #specs="{ row }">
              <span class="spec-text">{{ specText(row) }}</span>
            </template>

            <template #price="{ row }">
              <div class="price-cell">
                <span v-if="row.sale_price > 0" class="price-sale">标价 ¥{{ formatPrice(row.sale_price) }}</span>
                <span v-else class="price-sale price-sale--missing">上游未定价</span>
                <span class="price-cost">
                  代理价 {{ row.cost_price > 0 ? `¥${formatPrice(row.cost_price)}` : '—' }}
                </span>
              </div>
            </template>

            <template #margin="{ row }">
              <div v-if="row.cost_price > 0 && row.sale_price > row.cost_price" class="price-cell">
                <span class="price-margin">¥{{ formatPrice(row.sale_price - row.cost_price) }}</span>
                <span class="price-cost">上游让利 {{ discountRate(row) }}%</span>
              </div>
              <span v-else class="muted">—</span>
            </template>

            <template #status="{ row }">
              <t-tooltip :content="statusTooltip(row)">
                <t-tag :theme="statusTheme(row)" variant="light" size="small" shape="round">
                  {{ statusText(row) }}
                </t-tag>
              </t-tooltip>
            </template>

            <!-- 与「上游状态」是两个维度：上游是否在售 ≠ 我方是否已导入 -->
            <template #imported="{ row }">
              <t-tooltip v-if="row.imported" :content="`已导入为本地商品 #${row.imported_product_id}（${importedStatusText(row)}），点击查看`">
                <t-link theme="success" hover="color" @click="goImportedProduct(row)">已导入</t-link>
              </t-tooltip>
              <t-tooltip v-else content="还没导入为本地商品：导入后才会生成可上架售卖的本地商品">
                <t-link theme="primary" hover="color" @click="goImport(row)">未导入 · 去导入</t-link>
              </t-tooltip>
            </template>

            <template #action="{ row }">
              <div class="action-cell">
                <MobileAction
                  v-if="isMobile"
                  :options="buildMobileActionOptions([
                    { content: '详情', value: 'detail', theme: 'default' },
                    { content: '调价', value: 'markup', theme: 'default' },
                    { content: '订正同步价', value: 'price', theme: 'default' },
                  ])"
                  @select="(value) => handleMobileAction(value, row)"
                />
                <template v-else>
                  <t-link theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
                  <t-link theme="primary" hover="color" @click="openMarkupDialog(row)">调价</t-link>
                  <t-tooltip content="只修正本地镜像里的上游标价/代理价，下次同步会被上游值覆盖；改对客售价请用「调价」">
                    <t-link theme="default" hover="color" @click="openPriceDialog(row)">订正同步价</t-link>
                  </t-tooltip>
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
      </div>
    </div>

    <t-dialog
      v-model:visible="detailVisible"
      :header="detailProduct ? `商品详情 · ${detailProduct.name}` : '商品详情'"
      width="680px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailProduct" :column="2" bordered size="small">
        <t-descriptions-item label="商品名称">{{ detailProduct.name }}</t-descriptions-item>
        <t-descriptions-item label="上游 ID">{{ detailProduct.upstream_id || '—' }}</t-descriptions-item>
        <t-descriptions-item label="服务商">{{ providerName(detailProduct.provider_id) }}</t-descriptions-item>
        <t-descriptions-item label="上游分类">
          <span class="group-cell">
            <t-tag theme="primary" variant="light" size="small" shape="round">
              {{ detailProduct.group_name || `分组 #${detailProduct.group_id}` }}
            </t-tag>
            <t-tag v-if="detailProduct.group_name_derived" size="small" variant="light-outline" theme="warning">派生</t-tag>
          </span>
        </t-descriptions-item>
        <t-descriptions-item label="规格">{{ specText(detailProduct) }}</t-descriptions-item>
        <t-descriptions-item label="磁盘类型">{{ detailProduct.disk_type || '—' }}</t-descriptions-item>
        <t-descriptions-item label="操作系统">{{ detailProduct.os || '—' }}</t-descriptions-item>
        <t-descriptions-item label="区域/可用区">{{ detailProduct.region }}{{ detailProduct.zone ? ' / ' + detailProduct.zone : '' }}</t-descriptions-item>
        <t-descriptions-item label="上游标价">
          {{ detailProduct.sale_price > 0 ? `¥${formatPrice(detailProduct.sale_price)}` : '上游未定价' }}
        </t-descriptions-item>
        <t-descriptions-item label="代理价（采购成本）">
          {{ detailProduct.cost_price > 0 ? `¥${formatPrice(detailProduct.cost_price)}` : '—' }}
        </t-descriptions-item>
        <t-descriptions-item label="上游让利（标价−代理价）">
          {{ detailProduct.cost_price > 0 && detailProduct.sale_price > detailProduct.cost_price ? `¥${formatPrice(detailProduct.sale_price - detailProduct.cost_price)}（${discountRate(detailProduct)}%）` : '—' }}
        </t-descriptions-item>
        <t-descriptions-item label="上游状态">
          <t-tag :theme="statusTheme(detailProduct)" variant="light" size="small" shape="round">
            {{ statusText(detailProduct) }}
          </t-tag>
          <span class="inline-hint">{{ statusTooltip(detailProduct) }}</span>
        </t-descriptions-item>
        <t-descriptions-item label="导入状态">
          <template v-if="detailProduct.imported">
            已导入为本地商品
            <t-link theme="primary" hover="color" @click="goImportedProduct(detailProduct)">
              #{{ detailProduct.imported_product_id }}
            </t-link>
            （{{ importedStatusText(detailProduct) }}）
          </template>
          <span v-else class="muted">未导入（可在「产品管理 → 产品列表 → 导入上游商品」里导入）</span>
        </t-descriptions-item>
        <t-descriptions-item label="价格来源" :span="2">
          <span v-if="detailProduct.price_source === 'catalog'">
            上游前台展示价（该商品价格挂在配置项上，商品周期价为 0，已按上游前台展示价回填）
            <template v-if="detailProduct.catalog_price">
              ：标价 ¥{{ formatPrice(detailProduct.catalog_price) }}
              <template v-if="detailProduct.catalog_sale_price">/ 应付 ¥{{ formatPrice(detailProduct.catalog_sale_price) }}</template>
              <template v-if="detailProduct.catalog_cycle">（{{ cycleLabel(detailProduct.catalog_cycle) }}）</template>
            </template>
          </span>
          <span v-else-if="detailProduct.price_source === 'none'">上游未定价（周期价与前台展示价都为空，导入后需人工定价）</span>
          <span v-else>上游商品周期价</span>
        </t-descriptions-item>
        <t-descriptions-item v-if="detailProduct.group_name_derived" label="分类说明" :span="2">
          上游把该分类隐藏了（不在前台导航里），分类名是按商品名前缀派生的近似标签，供运营辨识用。
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
      v-model:visible="markupVisible"
      :header="markupProduct ? `调价 · ${markupProduct.name}` : '调价'"
      width="560px"
      :confirm-btn="{ content: '保存并按规则重算已导入商品', theme: 'primary', loading: markupSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSaveMarkup"
      @close="markupVisible = false"
    >
      <t-form label-align="top" :data="markupForm" @submit.prevent>
        <div class="markup-summary">
          <span>上游标价 <b>{{ markupProduct && markupProduct.sale_price > 0 ? `¥${formatPrice(markupProduct.sale_price)}` : '未定价' }}</b></span>
          <span>代理价 <b>{{ markupProduct && markupProduct.cost_price > 0 ? `¥${formatPrice(markupProduct.cost_price)}` : '—' }}</b></span>
        </div>
        <t-form-item label="加价方式">
          <t-radio-group v-model="markupForm.markup_type">
            <t-radio value="list_percent">按上游标价 × 百分比（推荐）</t-radio>
            <t-radio value="percent">按代理价 × 百分比</t-radio>
            <t-radio value="fixed">按代理价 + 固定额</t-radio>
            <t-radio value="">不设置（清除规则）</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item v-if="markupForm.markup_type" :label="markupValueLabel">
          <t-input-number
            v-model="markupForm.markup_value"
            :min="0"
            :precision="2"
            theme="column"
            :suffix="markupForm.markup_type === 'fixed' ? '元' : '%'"
          />
          <span class="clone-tip">{{ markupHint }}</span>
        </t-form-item>
        <t-form-item label="调价备注">
          <t-input v-model="markupForm.remark" placeholder="可选，写入商品变更历史" />
        </t-form-item>
      </t-form>

      <div class="markup-preview">
        <span class="markup-preview__title">售价预览</span>
        <div class="markup-preview__grid">
          <span>对客售价</span>
          <b>{{ previewPrice > 0 ? `¥${formatPrice(previewPrice)}` : '—' }}</b>
          <span>单件毛利</span>
          <b :class="previewProfit >= 0 ? 'profit-up' : 'profit-down'">
            {{ previewPrice > 0 && markupProduct && markupProduct.cost_price > 0 ? `¥${formatPrice(previewProfit)}（${previewProfitRate}%）` : '—' }}
          </b>
        </div>
        <p class="markup-preview__note">
          保存后该规则会写入此上游商品已导入的全部商品（售价 + 各周期价格矩阵），并写入商品变更历史；
          后续上游调价时也按此规则自动重算。调价不影响已下订单。
        </p>
      </div>
    </t-dialog>

    <t-dialog
      v-model:visible="priceVisible"
      header="订正同步价（仅修正本地镜像）"
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
      <p class="dialog-note">
        这里改的是本地镜像的上游价格，用于订正同步数据；下次同步会以上游实际价格覆盖。
        要给客户定售价，请用「调价」设定加价规则。
      </p>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { applyUpstreamMarkup, getProductDetail, getProductList, getProviderList, updateProductPrice } from '@/api/admin'
import type { ProductInfo, ProviderInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ResourceProducts' })

const router = useRouter()
const productList = ref<ProductInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])
const providerNameMap = ref<Record<number, string>>({})
const statusOptions = [
  { label: '正常', value: 1 },
  { label: '隐藏', value: 0 },
]

const filters = reactive<{ keyword: string | undefined; provider_id: number | undefined; group_id: number | undefined; status: number | undefined }>({
  keyword: undefined,
  provider_id: undefined,
  group_id: undefined,
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

/** 分类导航：从当前服务商的全量商品聚合（上游商品按分类组织，此前整页混在一起没有分类概念）。 */
type NavGroup = { group_id: number; group_name: string; group_name_derived: boolean; total: number; on_sale: number }
const navGroups = ref<NavGroup[]>([])
const navStats = reactive<{ total: number; onSale: number }>({ total: 0, onSale: 0 })
const navTotal = computed(() => navGroups.value.length)
const activeGroupName = computed(() => {
  if (filters.group_id === undefined) return '商品列表'
  const hit = navGroups.value.find((g) => g.group_id === filters.group_id)
  return hit ? `${hit.group_name || `分组 #${hit.group_id}`}（${hit.total}）` : '商品列表'
})
const onSaleInPage = computed(() => productList.value.filter((row) => isOnSale(row)).length)
const importedInPage = computed(() => productList.value.filter((row) => row.imported).length)

function formatMemory(value: number): string {
  if (!value) return '—'
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function formatDisk(value: number): string {
  if (!value) return '—'
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function specText(row: ProductInfo): string {
  const parts: string[] = []
  if (row.cpu) parts.push(`${row.cpu}核`)
  if (row.memory) parts.push(formatMemory(row.memory))
  if (row.disk) parts.push(formatDisk(row.disk))
  if (row.bandwidth) parts.push(`${row.bandwidth}Mbps`)
  return parts.length ? parts.join(' / ') : '规格未同步'
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

function cycleLabel(cycle: string): string {
  const map: Record<string, string> = {
    hour: '小时付',
    day: '日付',
    monthly: '月付',
    quarterly: '季付',
    semiannually: '半年付',
    annually: '年付',
    biennially: '两年付',
    triennially: '三年付',
    onetime: '一次性',
  }
  return map[cycle] || cycle
}

function providerName(providerId: number): string {
  return providerNameMap.value[providerId] || `#${providerId}`
}

/** 上游让利率：1 − 代理价/标价（上游给我们的折扣力度）。 */
function discountRate(row: ProductInfo): number {
  if (!(row.sale_price > 0) || !(row.cost_price > 0)) return 0
  return Math.round((1 - row.cost_price / row.sale_price) * 100)
}

/** 是否在售：以同步按上游真实可见性写入的 status 为准（隐藏分组/隐藏商品/已下架都不算）。 */
function isOnSale(row: ProductInfo): boolean {
  if (row.visibility) return row.visibility === 'visible'
  return row.status === 1
}

/**
 * 上游状态文案只留两态：正常 / 隐藏。
 * 「未在售」这种说法容易被理解成"我方还没对接"，实际说的是上游商铺是否展示该商品，
 * 故统一改称「正常 / 隐藏」，具体原因放到悬停说明与详情里。
 */
function statusText(row: ProductInfo): string {
  if (isOnSale(row)) return '正常'
  return '隐藏'
}

function statusTheme(row: ProductInfo): 'success' | 'warning' | 'danger' | 'default' {
  if (isOnSale(row)) return 'success'
  return row.visibility === 'retired' ? 'danger' : 'warning'
}

/** 已导入商品在本地商品表里的状态文案。 */
function importedStatusText(row: ProductInfo): string {
  switch (row.imported_status) {
    case 1:
      return '上架中'
    case 2:
      return '已下架'
    default:
      return '草稿'
  }
}

/** 跳到本地商品详情（看这个上游商品被导成了什么、卖多少钱）。 */
function goImportedProduct(row: ProductInfo) {
  if (!row.imported_product_id) return
  router.push(`/product/products/${row.imported_product_id}`)
}

/** 上游状态的含义（鼠标悬停解释是哪种情况：正常 / 分类隐藏 / 商品隐藏 / 已下架）。 */
function statusTooltip(row: ProductInfo): string {
  switch (row.visibility) {
    case 'visible':
      return '正常：上游商铺正在展示这个商品'
    case 'group_hidden':
      return '隐藏：上游把整个分类从商铺隐藏了（商品不展示但仍可下单，导入与开通不受影响）'
    case 'product_hidden':
      return '隐藏：上游单独隐藏了这个商品（不在商铺展示）'
    case 'retired':
      return '隐藏：上游已下架该商品（不建议导入）'
    default:
      return '隐藏：上游商铺未展示'
  }
}

/** 带参数跳到商品列表页并打开导入弹窗（预选这个上游商品）。 */
function goImport(row: ProductInfo) {
  router.push({ path: '/product/products', query: { import: String(row.provider_id || filters.provider_id || ''), preselect: String(row.id) } })
}

const columns: PrimaryTableCol<ProductInfo>[] = [
  { colKey: 'name', title: '商品', minWidth: 190 },
  { colKey: 'group', title: '分类', width: 150 },
  { colKey: 'provider', title: '服务商', width: 120 },
  { colKey: 'specs', title: '规格', minWidth: 190 },
  { colKey: 'price', title: '价格（标价 / 代理价）', width: 160 },
  { colKey: 'margin', title: '毛利空间', width: 120 },
  { colKey: 'status', title: '上游状态', width: 130 },
  { colKey: 'imported', title: '本地导入', width: 120 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 190,
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
      group_id: filters.group_id,
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

/**
 * 分类导航数据：拉当前筛选（关键词/服务商/状态之外只按服务商）的全量商品聚合。
 * 上游商品量级在几百条（列表接口上限 500），一次拉全便于出准确的分类计数。
 */
async function loadNavGroups() {
  try {
    const data = await getProductList({
      provider_id: filters.provider_id,
      page: 1,
      page_size: 500,
    })
    const map = new Map<number, NavGroup>()
    let onSale = 0
    for (const item of data.items) {
      const gid = item.group_id || 0
      if (!map.has(gid)) {
        map.set(gid, {
          group_id: gid,
          group_name: item.group_name || '',
          group_name_derived: Boolean(item.group_name_derived),
          total: 0,
          on_sale: 0,
        })
      }
      const group = map.get(gid)!
      group.total += 1
      if (isOnSale(item)) {
        group.on_sale += 1
        onSale += 1
      }
      if (!group.group_name && item.group_name) group.group_name = item.group_name
    }
    navGroups.value = Array.from(map.values()).sort((a, b) => {
      if (a.group_name_derived !== b.group_name_derived) return a.group_name_derived ? 1 : -1
      return (a.group_name || `#${a.group_id}`).localeCompare(b.group_name || `#${b.group_id}`, 'zh-Hans-CN')
    })
    navStats.total = data.meta.total
    navStats.onSale = onSale
  } catch {
    /* 导航聚合失败不阻塞列表 */
  }
}

async function loadAll() {
  await Promise.all([loadProducts(), loadNavGroups()])
}

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
    const nameMap: Record<number, string> = {}
    for (const item of data.items as ProviderInfo[]) {
      nameMap[item.id] = item.name
    }
    providerNameMap.value = nameMap
  } catch {
    /* 提供商下拉加载失败不阻塞列表 */
  }
}

function selectGroup(groupId: number | undefined) {
  filters.group_id = groupId
  pagination.current = 1
  loadProducts()
}

function handleProviderChange() {
  // 切服务商时分类归属完全变了，回到「全部商品」并刷新导航。
  filters.group_id = undefined
  pagination.current = 1
  loadAll()
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
  filters.group_id = undefined
  filters.status = undefined
  pagination.current = 1
  loadAll()
}

function goImportPage() {
  // 顶部入口：不预选任何商品，只跳到商品列表页打开导入弹窗。
  router.push({ path: '/product/products', query: { import: filters.provider_id ? String(filters.provider_id) : undefined } })
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

// —— 调价（设定加价规则并重算已导入商品售价） ——
const markupVisible = ref(false)
const markupSubmitting = ref(false)
const markupProduct = ref<ProductInfo | null>(null)
const markupForm = reactive<{ markup_type: '' | 'percent' | 'fixed' | 'list_percent'; markup_value: number; remark: string }>({
  markup_type: 'list_percent',
  markup_value: 100,
  remark: '',
})

const markupValueLabel = computed(() => {
  switch (markupForm.markup_type) {
    case 'list_percent':
      return '标价百分比（100 = 按上游原价）'
    case 'percent':
      return '代理价百分比（%）'
    case 'fixed':
      return '加价固定额（元）'
    default:
      return '数值'
  }
})

const markupHint = computed(() => {
  switch (markupForm.markup_type) {
    case 'list_percent':
      return '售价 = 上游标价 × N%。100 即按上游原价卖；>100 加价，<100 让利；成本仍是代理价。'
    case 'percent':
      return '售价 = 代理价（采购成本）× N%。'
    case 'fixed':
      return '售价 = 代理价（采购成本）+ N 元。'
    default:
      return ''
  }
})

/** 预览售价：与后端 applyMarkupWithList 同口径（标价口径缺标价时退回代理价）。 */
const previewPrice = computed(() => {
  const product = markupProduct.value
  if (!product || !markupForm.markup_type) return 0
  const cost = product.cost_price > 0 ? product.cost_price : 0
  const list = product.sale_price > 0 ? product.sale_price : 0
  switch (markupForm.markup_type) {
    case 'list_percent': {
      const base = list > 0 ? list : cost
      return Math.round(base * markupForm.markup_value) / 100
    }
    case 'percent':
      return Math.round(cost * markupForm.markup_value) / 100
    case 'fixed':
      return Math.round((cost + markupForm.markup_value) * 100) / 100
    default:
      return 0
  }
})

const previewProfit = computed(() => {
  const product = markupProduct.value
  if (!product || !(product.cost_price > 0)) return 0
  return Math.round((previewPrice.value - product.cost_price) * 100) / 100
})

const previewProfitRate = computed(() => {
  if (!(previewPrice.value > 0)) return 0
  return Math.round((previewProfit.value / previewPrice.value) * 100)
})

function openMarkupDialog(row: ProductInfo) {
  markupProduct.value = row
  // 默认按标价 100%（=按上游原价卖），这与导入弹窗的默认口径一致。
  markupForm.markup_type = 'list_percent'
  markupForm.markup_value = 100
  markupForm.remark = ''
  markupVisible.value = true
}

async function handleSaveMarkup() {
  const product = markupProduct.value
  if (!product) return
  if (markupForm.markup_type && !(markupForm.markup_value > 0)) {
    MessagePlugin.warning('请填写大于 0 的加价数值')
    return
  }
  markupSubmitting.value = true
  try {
    const resp = await applyUpstreamMarkup({
      resource_product_id: product.id,
      markup_type: markupForm.markup_type,
      markup_value: markupForm.markup_value,
      remark: markupForm.remark,
    })
    if (resp.affected > 0) {
      MessagePlugin.success(resp.message || `已重算 ${resp.affected} 个商品售价`)
    } else {
      MessagePlugin.warning('该上游商品还没有导入成商品；导入时请在导入弹窗里填写同样的定价百分比')
    }
    markupVisible.value = false
  } catch (error) {
    MessagePlugin.error((error as Error).message || '调价失败')
  } finally {
    markupSubmitting.value = false
  }
}

// —— 订正同步价（仅修正本地镜像） ——
const priceVisible = ref(false)
const priceForm = reactive<{ cost_price: number; sale_price: number }>({ cost_price: 0, sale_price: 0 })
const priceProduct = ref<ProductInfo | null>(null)

function openPriceDialog(row: ProductInfo) {
  priceProduct.value = row
  priceForm.cost_price = row.cost_price
  priceForm.sale_price = row.sale_price
  priceVisible.value = true
}

async function handleSavePrice() {
  if (!priceProduct.value) return
  try {
    await updateProductPrice(priceProduct.value.id, {
      cost_price: priceForm.cost_price,
      sale_price: priceForm.sale_price,
    })
    MessagePlugin.success('同步价已订正')
    priceVisible.value = false
    loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新价格失败')
  }
}

onMounted(() => {
  loadProviders()
  loadAll()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: ProductInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetailDialog(row)
      break
    case 'markup':
      openMarkupDialog(row)
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

/* 桌面列数由 FilterCard 的 columns 属性给出 3 列，见组件内 .filter-card--cols-*。 */

.resource-products {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.resource-products__aside {
  width: 232px;
  flex: none;
  padding: 12px;
  position: sticky;
  top: 12px;
  max-height: calc(100vh - 140px);
  overflow: auto;
}

.resource-products__main {
  flex: 1;
  min-width: 0;
}

.inline-hint {
  margin-left: 6px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.table-card__hint {
  margin: 0 0 8px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.page-header__sub {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.aside-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  font-weight: 600;
  color: #334155;
  padding: 4px 6px 8px;
}

.aside-head__count {
  font-weight: 400;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.aside-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.aside-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 7px 8px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: #334155;
}

.aside-item:hover {
  background: var(--td-bg-color-container-hover, #f5f5f5);
}

.aside-item--active {
  background: rgba(0, 82, 217, 0.08);
  color: var(--td-brand-color, #0052d9);
  font-weight: 600;
}

.aside-item__name {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.aside-item__meta {
  font-size: 11px;
  color: var(--color-muted-foreground);
  font-weight: 400;
}

@media (max-width: 900px) {
  .resource-products {
    flex-direction: column;
  }

  .resource-products__aside {
    width: 100%;
    position: static;
    max-height: none;
  }
}

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

.group-cell {
  display: inline-flex;
  align-items: center;
  gap: 4px;
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

.price-sale--missing {
  color: var(--td-warning-color, #e37318);
  font-weight: 500;
}

.price-cost {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.price-margin {
  font-size: 13px;
  font-weight: 600;
  color: #2ba471;
  font-variant-numeric: tabular-nums;
}

.muted {
  color: var(--color-muted-foreground);
}

/* 调价弹窗：上游价摘要 + 售价预览 */
.markup-summary {
  display: flex;
  gap: 16px;
  padding: 8px 10px;
  margin-bottom: 12px;
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer, #f5f5f5);
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.markup-summary b {
  color: #334155;
}

.clone-tip {
  display: block;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.markup-preview {
  border: 1px solid var(--td-component-border, #e7e7e7);
  border-radius: 6px;
  padding: 10px 12px;
}

.markup-preview__title {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.markup-preview__grid {
  display: grid;
  grid-template-columns: 88px 1fr;
  gap: 4px 8px;
  margin-top: 6px;
  align-items: baseline;
  font-size: 13px;
}

.markup-preview__grid span {
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.markup-preview__grid b {
  color: #334155;
  font-variant-numeric: tabular-nums;
}

.profit-up {
  color: #2ba471 !important;
}

.profit-down {
  color: #d54941 !important;
}

.markup-preview__note,
.dialog-note {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
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
