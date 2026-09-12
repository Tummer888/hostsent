<template>
  <div class="page-body payment-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MailIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">回调日志</h2>
          <p class="page-header__desc">渠道异步通知全量留痕：原始报文、验签结果与处理结论，用于排障与重放核验。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadCallbacks">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">渠道编码</span>
          <t-input v-model="filters.channel_code" placeholder="如 manual_main" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">支付单号</span>
          <t-input v-model="filters.payment_no" placeholder="如 P20260912…" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">验签结果</span>
          <t-select v-model="filters.verify_ok" clearable placeholder="全部" :options="verifyOptions" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">回调记录</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="callbackList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #payment_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.payment_no || '—' }}</span>
            <span class="price-sub">{{ row.channel_code || '—' }}</span>
          </div>
        </template>

        <template #notify_id="{ row }">
          <span class="cell-muted">{{ row.notify_id || '—' }}</span>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #verify_ok="{ row }">
          <t-tag :theme="row.verify_ok ? 'success' : 'danger'" variant="light" size="small" shape="round">
            {{ row.verify_ok ? '通过' : '失败' }}
          </t-tag>
        </template>

        <template #handle_status="{ row }">
          <t-tag :theme="callbackHandleTheme(row.handle_status)" variant="light" size="small" shape="round">
            {{ callbackHandleLabel(row.handle_status) }}
          </t-tag>
        </template>

        <template #source_ip="{ row }">
          <span class="cell-muted">{{ row.source_ip || '—' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="openDetail(row)">报文</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无回调记录" />
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

    <t-drawer v-model:visible="detailVisible" header="回调详情" size="600px" :footer="false">
      <template v-if="detail">
        <div class="detail-block">
          <div class="recon-row"><span class="recon-row__label">支付单号</span><span class="recon-row__value">{{ detail.payment_no || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">渠道</span><span class="recon-row__value">{{ detail.channel_code || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">通知 ID</span><span class="recon-row__value">{{ detail.notify_id || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">金额</span><span class="recon-row__value">¥{{ formatPrice(detail.amount) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">验签</span><span class="recon-row__value">{{ detail.verify_ok ? '通过' : '失败' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">处理结果</span><span class="recon-row__value">{{ callbackHandleLabel(detail.handle_status) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">来源 IP</span><span class="recon-row__value">{{ detail.source_ip || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">接收时间</span><span class="recon-row__value">{{ formatTime(detail.created_at) }}</span></div>
        </div>
        <div v-if="detail.handle_msg" class="detail-block">
          <h4 class="detail-block__title">处理信息</h4>
          <pre class="detail-pre">{{ detail.handle_msg }}</pre>
        </div>
        <div class="detail-block">
          <h4 class="detail-block__title">原始报文</h4>
          <pre class="detail-pre">{{ prettyJSON(detail.raw_body) }}</pre>
        </div>
      </template>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { MailIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getCallbacks } from '@/api/payment'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import { callbackHandleLabel, callbackHandleTheme, formatPrice, formatTime } from '@/pages/payment/constants'
import type { CallbackInfo } from '@/types/interface'

defineOptions({ name: 'PaymentCallbacks' })

const callbackList = ref<CallbackInfo[]>([])
const loading = ref(false)
const total = ref(0)
const { isMobile } = useIsMobile()

const verifyOptions = [
  { label: '验签通过', value: true },
  { label: '验签失败', value: false },
]

const filters = reactive<{ channel_code?: string; payment_no?: string; verify_ok?: boolean }>({
  channel_code: undefined,
  payment_no: undefined,
  verify_ok: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<CallbackInfo>[] = [
  { colKey: 'payment_no', title: '支付单 / 渠道', minWidth: 190 },
  { colKey: 'notify_id', title: '通知 ID', minWidth: 150 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'verify_ok', title: '验签', width: 80, align: 'center' as const },
  { colKey: 'handle_status', title: '处理', width: 100 },
  { colKey: 'source_ip', title: '来源 IP', width: 130 },
  { colKey: 'created_at', title: '接收时间', width: 170 },
  { colKey: 'action', title: '操作', width: 90, fixed: 'right' as const, align: 'center' as const },
]

async function loadCallbacks() {
  loading.value = true
  try {
    const data = await getCallbacks({
      channel_code: filters.channel_code,
      payment_no: filters.payment_no,
      verify_ok: filters.verify_ok,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    callbackList.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载回调日志失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  loadCallbacks()
}

function handleResetFilters() {
  filters.channel_code = undefined
  filters.payment_no = undefined
  filters.verify_ok = undefined
  pagination.current = 1
  loadCallbacks()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadCallbacks()
}

function goMobilePage(target: number) {
  const maxPage = Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize))
  const clamped = Math.min(Math.max(target, 1), maxPage)
  if (clamped === mobilePage.current) return
  handlePageChange({ current: clamped, pageSize: mobilePage.pageSize } as PageInfo)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  handlePageChange({ current: 1, pageSize } as PageInfo)
}

const detailVisible = ref(false)
const detail = ref<CallbackInfo | null>(null)

function openDetail(row: CallbackInfo) {
  detail.value = row
  detailVisible.value = true
}

function prettyJSON(raw: string): string {
  if (!raw) return '—'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

onMounted(loadCallbacks)
</script>

<style lang="css" scoped>
.detail-block {
  margin-bottom: var(--space-lg);
}

.detail-block__title {
  margin: 0 0 var(--space-sm);
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.detail-pre {
  margin: 0;
  padding: var(--space-sm) var(--space-md);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  font-size: 12px;
  font-family: var(--hs-font-mono);
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 420px;
  overflow: auto;
}
</style>

<style lang="css">
@import '../shared.css';
</style>
