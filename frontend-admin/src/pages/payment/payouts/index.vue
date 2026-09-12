<template>
  <div class="page-body payment-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <UploadIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">打款管理</h2>
          <p class="page-header__desc">提现审批通过后生成打款单：接口模式自动出款，人工模式由财务登记转账凭证后置为已打款。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadPayouts">
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
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">打款模式</span>
          <t-select v-model="filters.mode" clearable placeholder="全部模式" :options="modeOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="payoutStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">创建时间</span>
          <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
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
        <h3 class="card-title">打款单列表</h3>
        <span class="table-card__meta">共 {{ total }} 笔</span>
      </div>
      <t-table
        row-key="id"
        :data="payoutList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #payout_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.payout_no }}</span>
            <span class="price-sub">提现单 {{ row.withdraw_no || '—' }}</span>
          </div>
        </template>

        <template #user_id="{ row }">
          <span class="cell-muted">#{{ row.user_id }}</span>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #mode="{ row }">
          <t-tag :theme="row.mode === 'api' ? 'primary' : 'warning'" variant="light" size="small" shape="round">
            {{ payoutModeLabel(row.mode) }}
          </t-tag>
        </template>

        <template #status="{ row }">
          <t-tag :theme="payoutStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ payoutStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #attempts="{ row }">
          <span class="cell-muted">{{ row.attempts }}</span>
        </template>

        <template #fail_reason="{ row }">
          <span class="cell-muted" :title="row.fail_reason">{{ row.fail_reason || '—' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '登记打款', value: 'mark-paid', hidden: () => !isPayable(row) || !canOperate, theme: 'success' },
                { content: '重试打款', value: 'retry', hidden: () => !isRetryable(row) || !canOperate },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link v-if="isPayable(row) && canOperate" theme="success" hover="color" @click="openMarkPaid(row)">登记打款</t-link>
              <t-link v-if="isRetryable(row) && canOperate" theme="primary" hover="color" @click="handleRetry(row)">重试打款</t-link>
              <span v-if="!isPayable(row) && !isRetryable(row)" class="price-sub">—</span>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无打款单" />
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

    <!-- 人工打款登记 -->
    <t-dialog
      v-model:visible="markVisible"
      header="登记打款完成"
      width="480px"
      :confirm-btn="{ content: '确认已打款', theme: 'primary', loading: submitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleMarkPaid"
      @close="markVisible = false"
    >
      <t-alert
        v-if="markTarget"
        theme="warning"
        :message="`确认后打款单 ${markTarget.payout_no} 置为已打款，并解冻提现金额（¥${formatPrice(markTarget.amount)}），完成后不可撤销。`"
        style="margin-bottom: 12px"
      />
      <t-form label-align="top" :data="markForm" @submit.prevent>
        <t-form-item label="渠道交易号 / 转账流水号" name="channel_tx">
          <t-input v-model="markForm.channel_tx" placeholder="选填" />
        </t-form-item>
        <t-form-item label="打款凭证 URL" name="receipt_url">
          <t-input v-model="markForm.receipt_url" placeholder="选填，转账凭证图片地址" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-input v-model="markForm.remark" placeholder="选填" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { RefreshIcon, SearchIcon, UploadIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getPayouts, markPayoutPaid, retryPayout } from '@/api/payment'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import {
  formatPrice,
  formatTime,
  modeOptions,
  payoutModeLabel,
  payoutStatusLabel,
  payoutStatusOptions,
  payoutStatusTheme,
  toDateString,
} from '@/pages/payment/constants'
import type { PayoutInfo } from '@/types/interface'
import { useUserStore } from '@/store'

defineOptions({ name: 'PaymentPayouts' })

const userStore = useUserStore()
const canOperate = computed(() => userStore.permissions?.includes('payment:payout:operate') || userStore.isSuperAdmin)

const payoutList = ref<PayoutInfo[]>([])
const loading = ref(false)
const total = ref(0)
const { isMobile } = useIsMobile()

const filters = reactive<{
  user_id?: string
  mode?: string
  status?: string
  dateRange: (string | Date | undefined)[] | undefined
}>({
  user_id: undefined,
  mode: undefined,
  status: undefined,
  dateRange: [],
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<PayoutInfo>[] = [
  { colKey: 'payout_no', title: '打款单', minWidth: 200 },
  { colKey: 'user_id', title: '用户', width: 90, align: 'center' as const },
  { colKey: 'amount', title: '金额', width: 120 },
  { colKey: 'mode', title: '模式', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'attempts', title: '尝试', width: 70, align: 'center' as const },
  { colKey: 'fail_reason', title: '失败原因', minWidth: 160 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 160, fixed: 'right' as const, align: 'center' as const },
]

function isPayable(row: PayoutInfo): boolean {
  return row.mode === 'manual' && (row.status === 'pending' || row.status === 'paying')
}

function isRetryable(row: PayoutInfo): boolean {
  return row.status === 'failed'
}

async function loadPayouts() {
  loading.value = true
  try {
    const [startDate, endDate] = filters.dateRange ?? []
    const data = await getPayouts({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      mode: filters.mode,
      status: filters.status,
      start_time: toDateString(startDate) ? `${toDateString(startDate)} 00:00:00` : undefined,
      end_time: toDateString(endDate) ? `${toDateString(endDate)} 23:59:59` : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    payoutList.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载打款单失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  loadPayouts()
}

function handleResetFilters() {
  filters.user_id = undefined
  filters.mode = undefined
  filters.status = undefined
  filters.dateRange = []
  pagination.current = 1
  loadPayouts()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadPayouts()
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

const markVisible = ref(false)
const markTarget = ref<PayoutInfo | null>(null)
const submitting = ref(false)
const markForm = reactive<{ channel_tx?: string; receipt_url?: string; remark?: string }>({
  channel_tx: undefined,
  receipt_url: undefined,
  remark: undefined,
})

function openMarkPaid(row: PayoutInfo) {
  markTarget.value = row
  markForm.channel_tx = row.channel_tx || undefined
  markForm.receipt_url = row.receipt_url || undefined
  markForm.remark = undefined
  markVisible.value = true
}

async function handleMarkPaid() {
  if (!markTarget.value) return
  submitting.value = true
  try {
    await markPayoutPaid(markTarget.value.id, {
      channel_tx: markForm.channel_tx,
      receipt_url: markForm.receipt_url,
      remark: markForm.remark,
    })
    MessagePlugin.success('已登记打款完成')
    markVisible.value = false
    loadPayouts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '登记打款失败')
  } finally {
    submitting.value = false
  }
}

async function handleRetry(row: PayoutInfo) {
  try {
    await retryPayout(row.id)
    MessagePlugin.success('已提交重试打款')
    loadPayouts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重试打款失败')
  }
}

function handleMobileAction(value: string | number | Record<string, any>, row: PayoutInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'mark-paid':
      openMarkPaid(row)
      break
    case 'retry':
      handleRetry(row)
      break
  }
}

onMounted(loadPayouts)
</script>

<style lang="css">
@import '../shared.css';
</style>
