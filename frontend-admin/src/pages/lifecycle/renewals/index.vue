<template>
  <div class="page-body lifecycle-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <OrderIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">续费记录</h2>
          <p class="page-header__desc">查看所有续费单：来源（用户/自动/管理员代续）、金额与到期时间变化。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="续费单号 / 订单号 / 实例标识" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
        <div class="field">
          <span class="field__label">来源</span>
          <t-select v-model="filters.source" clearable placeholder="全部来源" :options="sourceOptions" />
        </div>
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input-number v-model="filters.user_id" :min="1" theme="column" placeholder="按用户过滤" style="width: 100%" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">续费记录</h3>
        <span class="table-card__meta">共 {{ total }} 条记录</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        bordered
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #renewal_no="{ row }">
          <t-link theme="primary" hover="color" @click="openDetail(row)">{{ row.renewal_no }}</t-link>
        </template>
        <template #user="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="product-sub">ID {{ row.user_id }}</span>
          </div>
        </template>
        <template #instance="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.instance_mark }}</span>
            <span class="product-sub">{{ row.product_name || '—' }}</span>
          </div>
        </template>
        <template #amount="{ row }">
          <span class="cell-strong">¥{{ formatAmount(row.amount) }}</span>
          <span class="product-sub">× {{ row.period_count }} 周期</span>
        </template>
        <template #source="{ row }">
          <t-tag :theme="sourceTheme(row.source)" variant="light" size="small" shape="round">
            {{ sourceLabel(row.source) }}
          </t-tag>
        </template>
        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
            {{ statusLabel(row.status) }}
          </t-tag>
        </template>
        <template #expire_change="{ row }">
          <div class="product-cell">
            <span class="time-text">{{ formatTime(row.expire_before) }} → {{ formatTime(row.expire_after) }}</span>
          </div>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无续费记录" />
        </template>
      </t-table>
    </section>

    <!-- 续费详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="续费详情" size="420px" :footer="false">
      <t-loading :loading="detailLoading" show-overlay>
        <div v-if="detail" class="detail-list">
          <div class="detail-row"><span>续费单号</span><b>{{ detail.renewal_no }}</b></div>
          <div class="detail-row"><span>状态</span>
            <t-tag :theme="statusTheme(detail.status)" variant="light" size="small">{{ statusLabel(detail.status) }}</t-tag>
          </div>
          <div class="detail-row"><span>来源</span>{{ sourceLabel(detail.source) }}</div>
          <div class="detail-row"><span>实例</span>{{ detail.instance_mark }}（{{ detail.product_name }}）</div>
          <div class="detail-row"><span>用户</span>{{ detail.username }}（ID {{ detail.user_id }}）</div>
          <div class="detail-row"><span>金额</span>¥{{ formatAmount(detail.amount) }} × {{ detail.period_count }} 周期</div>
          <div class="detail-row"><span>关联订单</span>{{ detail.order_no || '—' }}</div>
          <div class="detail-row"><span>支付时间</span>{{ formatTime(detail.pay_time) }}</div>
          <div class="detail-row"><span>续费前到期</span>{{ formatTime(detail.expire_before) }}</div>
          <div class="detail-row"><span>续费后到期</span>{{ formatTime(detail.expire_after) }}</div>
          <div class="detail-row"><span>创建时间</span>{{ formatTime(detail.created_at) }}</div>
          <div v-if="detail.fail_reason" class="detail-row"><span>失败原因</span><span class="fail-text">{{ detail.fail_reason }}</span></div>
        </div>
      </t-loading>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { OrderIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getRenewalDetail, getRenewalList, type RenewalInfo } from '@/api/lifecycle'

defineOptions({ name: 'LifecycleRenewals' })

const loading = ref(false)
const list = ref<RenewalInfo[]>([])
const total = ref(0)
const filters = reactive<{ keyword: string; status: string; source: string; user_id?: number }>({
  keyword: '',
  status: '',
  source: '',
})
const page = reactive({ current: 1, size: 10 })

const statusOptions = [
  { label: '待支付', value: 'pending' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '已取消', value: 'cancelled' },
]

const sourceOptions = [
  { label: '用户手动', value: 'manual' },
  { label: '自动续费', value: 'auto' },
  { label: '管理员代续', value: 'admin' },
]

const statusMap: Record<string, { label: string; theme: string }> = {
  pending: { label: '待支付', theme: 'warning' },
  success: { label: '成功', theme: 'success' },
  failed: { label: '失败', theme: 'danger' },
  cancelled: { label: '已取消', theme: 'default' },
}

const sourceMap: Record<string, { label: string; theme: string }> = {
  manual: { label: '用户手动', theme: 'primary' },
  auto: { label: '自动续费', theme: 'success' },
  admin: { label: '管理员代续', theme: 'warning' },
}

function statusLabel(s: string): string {
  return statusMap[s]?.label || s
}

function statusTheme(s: string): string {
  return statusMap[s]?.theme || 'default'
}

function sourceLabel(s: string): string {
  return sourceMap[s]?.label || s
}

function sourceTheme(s: string): string {
  return sourceMap[s]?.theme || 'default'
}

function formatTime(value?: string): string {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

function formatAmount(value?: number): string {
  return (value ?? 0).toFixed(2)
}

const columns: PrimaryTableCol[] = [
  { colKey: 'renewal_no', title: '续费单号', width: 170 },
  { colKey: 'user', title: '用户', width: 140 },
  { colKey: 'instance', title: '实例 / 产品', width: 180 },
  { colKey: 'amount', title: '金额', width: 130 },
  { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'expire_change', title: '到期时间变化', width: 280 },
  { colKey: 'created_at', title: '创建时间', width: 160 },
]

const pagination = computed(() => ({
  current: page.current,
  pageSize: page.size,
  total: total.value,
  showJumper: true,
}))

async function loadData() {
  loading.value = true
  try {
    const resp = await getRenewalList({
      keyword: filters.keyword || undefined,
      status: filters.status || undefined,
      source: filters.source || undefined,
      user_id: filters.user_id || undefined,
      page: page.current,
      page_size: page.size,
    })
    list.value = resp.items || []
    total.value = resp.meta?.total ?? 0
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载续费记录失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  page.current = 1
  loadData()
}

function handleReset() {
  filters.keyword = ''
  filters.status = ''
  filters.source = ''
  filters.user_id = undefined
  page.current = 1
  loadData()
}

function handlePageChange(info: PageInfo) {
  page.current = info.current
  page.size = info.pageSize
  loadData()
}

// —— 详情抽屉 ——
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<RenewalInfo | null>(null)

async function openDetail(row: RenewalInfo) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    detail.value = await getRenewalDetail(row.id)
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载续费详情失败')
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

onMounted(loadData)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.detail-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.detail-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  font-size: 13px;
  color: #334155;
}
.detail-row > span:first-child {
  flex: 0 0 88px;
  color: var(--color-muted-foreground, #94a3b8);
}
.fail-text {
  color: var(--td-error-color, #d54941);
}
</style>
