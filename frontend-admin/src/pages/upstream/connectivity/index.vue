<template>
  <div class="page-body">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <DataCheckedIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">连接测试</h2>
          <p class="page-header__desc">向上游提供商发起连通性探测，验证鉴权与接口可用性。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" @click="handleRefreshHistory">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新记录
        </t-button>
      </t-space>
    </header>

    <section class="test-grid">
      <article class="test-form-card surface-card">
        <h3 class="card-title">测试参数</h3>
        <div class="test-form">
          <div class="field">
            <span class="field__label">提供商</span>
            <t-select v-model="form.provider" clearable placeholder="请选择提供商" :options="providerOptions" />
          </div>
          <div class="field">
            <span class="field__label">检测项</span>
            <t-select v-model="form.check_item" placeholder="请选择检测项" :options="checkItemOptions" />
          </div>
          <t-button theme="primary" block :loading="testing" @click="handleRunTest">
            <template #icon>
              <PlayRectangleIcon aria-hidden="true" />
            </template>
            发起测试
          </t-button>
          <p class="hint-text">
            <InfoCircleIcon size="14" aria-hidden="true" />
            点击“发起测试”将实时调用所选提供商的连通性校验接口，并记录耗时。
          </p>
        </div>
      </article>

      <article class="result-card surface-card">
        <div class="card-head">
          <h3 class="card-title">最新检测结果</h3>
          <t-tag
            :theme="latestResult.status === 'success' ? 'success' : latestResult.status === 'error' ? 'danger' : 'warning'"
            variant="light"
            size="medium"
            shape="round"
          >
            {{ latestResult.status_label }}
          </t-tag>
        </div>
        <div class="result-grid">
          <div class="result-item">
            <span class="result-item__label">检测状态</span>
            <t-tag
              :theme="latestResult.status === 'success' ? 'success' : latestResult.status === 'error' ? 'danger' : 'warning'"
              variant="light"
              size="small"
              shape="round"
            >
              {{ latestResult.status_label }}
            </t-tag>
          </div>
          <div class="result-item">
            <span class="result-item__label">耗时</span>
            <span class="result-item__value">{{ latestResult.latency }}</span>
          </div>
          <div class="result-item result-item--full">
            <span class="result-item__label">请求路径</span>
            <code class="result-item__path">{{ latestResult.request_path }}</code>
          </div>
          <div class="result-item result-item--full">
            <span class="result-item__label">响应摘要</span>
            <span class="result-item__summary">{{ latestResult.summary }}</span>
          </div>
        </div>
      </article>
    </section>

    <section class="history-card surface-card">
      <div class="card-head">
        <h3 class="card-title">最近测试记录</h3>
        <span class="table-meta">共 {{ historyList.length }} 条（本地演示数据）</span>
      </div>
      <t-table
        row-key="id"
        :data="historyList"
        :columns="columns"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #check_item="{ row }">
          {{ checkItemLabelMap[row.check_item as CheckItem] || row.check_item }}
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 'success' ? 'success' : 'danger'" variant="light" size="small" shape="round">
            {{ row.status === 'success' ? '通过' : '失败' }}
          </t-tag>
        </template>

        <template #latency="{ row }">
          <span class="latency-cell">{{ row.latency }}</span>
        </template>

        <template #request_path="{ row }">
          <code class="mono-cell">{{ row.request_path }}</code>
        </template>

        <template #empty>
          <t-empty description="暂无测试记录" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { DataCheckedIcon, InfoCircleIcon, PlayRectangleIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProviderList, testConnection } from '@/api/admin'
import type { ProviderInfo } from '@/types/interface'

defineOptions({ name: 'UpstreamConnectivity' })

type CheckItem = 'products' | 'instances' | 'balance'
type TestStatus = 'success' | 'error'

const providers = ref<ProviderInfo[]>([])
const providerOptions = computed(() =>
  providers.value.map((item) => ({ label: item.name, value: item.id })),
)

const checkItemOptions = [
  { label: '连接测试', value: 'products' },
  { label: '列出实例', value: 'instances' },
  { label: '余额查询', value: 'balance' },
]

const checkItemLabelMap: Record<CheckItem, string> = {
  products: '连接测试',
  instances: '列出实例',
  balance: '余额查询',
}

const testing = ref(false)
const loadingProviders = ref(false)

const form = reactive({
  provider: undefined as number | undefined,
  check_item: 'products',
})

const latestResult = reactive({
  status: 'pending',
  status_label: '未执行',
  latency: '—',
  request_path: '—',
  summary: '请选择提供商并发起连接测试。',
})

async function loadProviders() {
  loadingProviders.value = true
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providers.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商失败')
  } finally {
    loadingProviders.value = false
  }
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

async function handleRunTest() {
  if (!form.provider) {
    MessagePlugin.warning('请先选择提供商')
    return
  }
  const provider = providers.value.find((item) => item.id === form.provider)
  if (!provider) return
  testing.value = true
  const startedAt = Date.now()
  try {
    const result = await testConnection(provider.id)
    const latency = Date.now() - startedAt
    const ok = result.success
    latestResult.status = ok ? 'success' : 'error'
    latestResult.status_label = ok ? '连接正常' : '连接失败'
    latestResult.latency = `${latency} ms`
    latestResult.request_path = provider.api_endpoint
    latestResult.summary = result.message || (ok ? `「${provider.name}」鉴权校验通过。` : `「${provider.name}」连接失败。`)
    if (ok) MessagePlugin.success(`「${provider.name}」连接成功`)
    else MessagePlugin.error(result.message || `「${provider.name}」连接失败`)
    historyList.value.unshift({
      id: Date.now(),
      time: formatTime(new Date().toISOString()),
      provider_name: provider.name,
      check_item: form.check_item as CheckItem,
      status: ok ? 'success' : 'error',
      latency: `${latency} ms`,
      request_path: provider.api_endpoint,
    })
  } catch (error) {
    latestResult.status = 'error'
    latestResult.status_label = '连接失败'
    latestResult.summary = (error as Error).message || '连接测试异常'
    MessagePlugin.error((error as Error).message || `「${provider.name}」连接测试失败`)
  } finally {
    testing.value = false
  }
}

function handleRefreshHistory() {
  loadProviders()
}

type HistoryRow = {
  id: number
  time: string
  provider_name: string
  check_item: CheckItem
  status: TestStatus
  latency: string
  request_path: string
}

const historyRows: HistoryRow[] = []

const historyList = ref<HistoryRow[]>([...historyRows])

onMounted(loadProviders)

const columns: PrimaryTableCol<HistoryRow>[] = [
  { colKey: 'time', title: '时间', width: 180 },
  { colKey: 'provider_name', title: '提供商', minWidth: 200, ellipsis: true },
  { colKey: 'check_item', title: '检测项', width: 120 },
  { colKey: 'request_path', title: '请求路径', minWidth: 220 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'latency', title: '耗时', width: 110, align: 'right' as const },
]
</script>

<style scoped lang="css">
.page-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

.surface-card {
  position: relative;
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  border: 1px solid var(--color-border);
  box-shadow: var(--hs-shadow-xs);
  transition:
    border-color var(--hs-duration-fast),
    box-shadow var(--hs-duration-fast);
}

.surface-card:hover {
  box-shadow: var(--hs-shadow-sm);
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-lg);
  padding: var(--space-lg) var(--space-xl);
  flex-wrap: wrap;
}

.page-header__main {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  min-width: 0;
}

.page-header__chip {
  width: 44px;
  height: 44px;
  border-radius: var(--hs-radius-xl);
  background: linear-gradient(135deg, #16a34a, #15803d);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 10px rgba(22, 163, 74, 0.25);
}

.page-header__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--color-foreground);
}

.page-header__desc {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.test-grid {
  display: grid;
  grid-template-columns: minmax(300px, 380px) minmax(0, 1fr);
  gap: var(--space-lg);
}

.test-form-card,
.result-card,
.history-card {
  padding: var(--space-lg) 20px;
}

.card-title {
  margin: 0 0 var(--space-md);
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.card-head .card-title {
  margin-bottom: 0;
}

.table-meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.test-form-card .card-title {
  margin-bottom: var(--space-lg);
}

.test-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.field__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.hint-text {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.result-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md) var(--space-xl);
  margin-top: var(--space-md);
}

.result-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.result-item--full {
  grid-column: 1 / -1;
}

.result-item__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.result-item__value {
  font-size: 20px;
  font-weight: 700;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.result-item__path {
  padding: 8px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  font-family: var(--hs-font-mono);
  font-size: 12px;
  color: #334155;
  word-break: break-all;
}

.result-item__summary {
  font-size: 13px;
  line-height: 1.7;
  color: #334155;
}

.history-card .card-head {
  margin-bottom: var(--space-md);
}

.mono-cell {
  font-family: var(--hs-font-mono);
  font-size: 12px;
  color: #334155;
}

.latency-cell {
  color: #334155;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 1100px) {
  .test-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .result-grid {
    grid-template-columns: 1fr;
  }
}
</style>
