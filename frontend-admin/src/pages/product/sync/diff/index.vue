<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">差异对比</h2>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="runCompare">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        重新对比
      </t-button>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">对比范围</h3>
        <t-button theme="primary" @click="runCompare">
          <template #icon>
            <SearchIcon aria-hidden="true" />
          </template>
          开始对比
        </t-button>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">上游供应商</span>
          <t-select v-model="providerId" clearable placeholder="全部供应商" :options="providerOptions" filterable />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">对比结果</h3>
        <span class="table-card__meta">上游 {{ upstreamTotal }} · 平台 {{ platformTotal }} · 未匹配 {{ unmatchedCount }}</span>
      </div>

      <div class="diff-summary">
        <div class="diff-summary__item diff-summary__item--ok">
          <span class="diff-summary__num">{{ matchedCount }}</span>
          <span class="diff-summary__label">已匹配</span>
        </div>
        <div class="diff-summary__item diff-summary__item--warn">
          <span class="diff-summary__num">{{ unmatchedCount }}</span>
          <span class="diff-summary__label">上游未匹配</span>
        </div>
        <div class="diff-summary__item diff-summary__item--muted">
          <span class="diff-summary__num">{{ platformOnlyCount }}</span>
          <span class="diff-summary__label">仅平台存在</span>
        </div>
      </div>

      <t-table
        row-key="key"
        :data="diffRows"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #match="{ row }">
          <t-tag :theme="row.match === 'ok' ? 'success' : row.match === 'unmatched' ? 'danger' : 'warning'" variant="light" size="small" shape="round">
            {{ row.matchLabel }}
          </t-tag>
        </template>

        <template #upstream="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.upstreamName }}</span>
            <span class="product-sub">上游 #{{ row.upstreamId }} · {{ row.upstreamSpec }}</span>
          </div>
        </template>

        <template #platform="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.platformName || '—' }}</span>
            <span class="product-sub">{{ row.platformNote }}</span>
          </div>
        </template>

        <template #empty>
          <t-empty description="请选择供应商后点击「开始对比」" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import { getProviderList as getUpstreamProviderList, getProductList as getUpstreamProductList } from '@/api/admin'
import { getProductList as getPlatformProductList } from '@/api/product'
import type { ProductInfo, ProviderInfo, SaleProductInfo } from '@/types/interface'

defineOptions({ name: 'ProductSyncDiff' })

const loading = ref(false)
const providerOptions = ref<{ label: string; value: number }[]>([])
const providerId = ref<number | undefined>(undefined)

const upstreamList = ref<ProductInfo[]>([])
const platformList = ref<SaleProductInfo[]>([])
const upstreamTotal = ref(0)
const platformTotal = ref(0)

interface DiffRow {
  key: string
  match: 'ok' | 'unmatched' | 'name'
  matchLabel: string
  upstreamId: number
  upstreamName: string
  upstreamSpec: string
  platformName: string
  platformNote: string
}

const columns: PrimaryTableCol<DiffRow>[] = [
  { colKey: 'match', title: '匹配状态', width: 110 },
  { colKey: 'upstream', title: '上游商品', minWidth: 200 },
  { colKey: 'platform', title: '平台商品', minWidth: 200 },
]

const diffRows = computed<DiffRow[]>(() => {
  const rows: DiffRow[] = []
  for (const up of upstreamList.value) {
    const linked = platformList.value.find((item) => item.source_product_id === up.id)
    const nameMatch = linked
      ? undefined
      : platformList.value.find((item) => item.name === up.name && !rows.some((r) => r.upstreamId === up.id))
    const target = linked || nameMatch
    if (target) {
      rows.push({
        key: `ok-${up.id}`,
        match: linked ? 'ok' : 'name',
        matchLabel: linked ? '已映射' : '名称匹配',
        upstreamId: up.id,
        upstreamName: up.name,
        upstreamSpec: up.specs || `${up.cpu}C/${up.memory}G/${up.disk}G`,
        platformName: target.name,
        platformNote: linked ? `通过 source_product_id 关联` : `仅名称相同，未建立映射`,
      })
    } else {
      rows.push({
        key: `up-${up.id}`,
        match: 'unmatched',
        matchLabel: '上游未匹配',
        upstreamId: up.id,
        upstreamName: up.name,
        upstreamSpec: up.specs || `${up.cpu}C/${up.memory}G/${up.disk}G`,
        platformName: '',
        platformNote: '平台暂无对应商品，需要新建或映射',
      })
    }
  }
  return rows
})

const matchedCount = computed(() => diffRows.value.filter((row) => row.match === 'ok' || row.match === 'name').length)
const unmatchedCount = computed(() => diffRows.value.filter((row) => row.match === 'unmatched').length)
const platformOnlyCount = computed(() => {
  if (!upstreamList.value.length) return 0
  const upstreamIds = new Set(upstreamList.value.map((item) => item.id))
  return platformList.value.filter((item) => !item.source_product_id || !upstreamIds.has(item.source_product_id)).length
})

async function loadProviders() {
  try {
    const data = await getUpstreamProviderList({ page: 1, page_size: 200 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 供应商加载失败不阻塞 */
  }
}

async function runCompare() {
  loading.value = true
  try {
    const [upData, platData] = await Promise.all([
      getUpstreamProductList({ provider_id: providerId.value, page: 1, page_size: 500 }),
      getPlatformProductList({ page: 1, page_size: 500, status: 1 }),
    ])
    upstreamList.value = upData.items
    platformList.value = platData.items
    upstreamTotal.value = upData.meta.total
    platformTotal.value = platData.meta.total
  } catch (error) {
    upstreamList.value = []
    platformList.value = []
    /* 对比失败时清空结果 */
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadProviders()
})
</script>

<style lang="css">
@import '../../shared.css';

.product-module .diff-summary {
  display: flex;
  gap: var(--space-md);
  margin-bottom: var(--space-lg);
  flex-wrap: wrap;
}

.product-module .diff-summary__item {
  display: flex;
  align-items: baseline;
  gap: var(--space-sm);
  padding: var(--space-md) var(--space-lg);
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-2);
  border: 1px solid var(--color-border);
  min-width: 120px;
}

.product-module .diff-summary__num {
  font-size: 24px;
  font-weight: 700;
}

.product-module .diff-summary__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.product-module .diff-summary__item--ok .diff-summary__num {
  color: #10b981;
}

.product-module .diff-summary__item--warn .diff-summary__num {
  color: #ef4444;
}

.product-module .diff-summary__item--muted .diff-summary__num {
  color: #6b7280;
}
</style>
