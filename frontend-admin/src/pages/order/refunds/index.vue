<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">退款管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadRefunds">
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
        <t-input v-model="filters.keyword" placeholder="退款单号 / 原因" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">关联订单 ID</span>
        <t-input v-model="filters.order_id" placeholder="精确匹配订单 ID" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">状态</span>
        <t-select v-model="filters.status" clearable placeholder="全部状态" :options="refundStatusOptions" />
      </div>
      <div class="field">
        <span class="field__label">退款去向</span>
        <t-select v-model="filters.refund_mode" clearable placeholder="全部去向" :options="refundModeOptions" />
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
        <h3 class="card-title">退款单列表</h3>
        <span class="table-card__meta">共 {{ total }} 个退款单</span>
      </div>

      <!-- 批量审核条：勾选后出现（移动端不提供多选，与实例列表同口径） -->
      <div class="batch-bar" :class="{ 'batch-bar--active': selectedIds.length > 0 }">
        <span class="batch-bar__count">
          <template v-if="selectedIds.length">已选 {{ selectedIds.length }} 个退款单</template>
          <template v-else>勾选退款单后可批量通过 / 驳回（仅待审核生效）</template>
        </span>
        <t-space size="small">
          <t-button
            v-if="canAudit"
            size="small"
            theme="primary"
            variant="outline"
            :disabled="!selectedIds.length || batchLoading"
            @click="openBatchAuditDialog(true)"
          >
            批量通过
          </t-button>
          <t-button
            v-if="canAudit"
            size="small"
            theme="danger"
            variant="outline"
            :disabled="!selectedIds.length || batchLoading"
            @click="openBatchAuditDialog(false)"
          >
            批量驳回
          </t-button>
          <t-button v-if="selectedIds.length" size="small" variant="text" @click="clearSelection">清空选择</t-button>
        </t-space>
      </div>

      <t-table
        row-key="id"
        :data="refundList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :selected-row-keys="selectedIds"
        :pagination="isMobile ? undefined : pagination"
        @select-change="handleSelectChange"
        @page-change="handlePageChange"
      >
        <template #refund_no="{ row }">
          <t-link theme="primary" hover="color" @click="openDetail(row)">{{ row.refund_no }}</t-link>
        </template>

        <template #amount="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
            <span v-if="row.refund_mode === 'channel' && row.fee_amount > 0" class="price-sub">
              扣点 ¥{{ formatPrice(row.fee_amount) }}
            </span>
          </div>
        </template>

        <template #refund_mode="{ row }">
          <div class="price-cell">
            <t-tag :theme="refundModeTheme(row.refund_mode)" variant="light" size="small" shape="round">
              {{ refundModeLabel(row.refund_mode) }}
            </t-tag>
            <span v-if="row.refund_mode === 'channel'" class="price-sub">
              {{ channelRefundStatusLabel(row.channel_refund_status) }}
            </span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="refundStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ refundStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="
                buildMobileActionOptions([
                  { content: '详情', value: 'detail', theme: 'default' },
                  { content: '审核', value: 'audit', hidden: () => !(row.status === 'pending'), theme: 'warning' },
                ])
              "
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link v-if="row.status === 'pending'" theme="warning" hover="color" @click="openDetail(row)"
                >审核</t-link
              >
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无退款单" />
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

    <!-- 批量审核：统一填写审核备注（写入每张退款单，后端字段已支持） -->
    <t-dialog
      v-model:visible="batchAudit.visible"
      :header="batchAudit.approve ? '批量通过退款' : '批量驳回退款'"
      width="480px"
      :confirm-btn="{
        content: `${batchAudit.approve ? '通过' : '驳回'} ${selectedIds.length} 张`,
        theme: batchAudit.approve ? 'primary' : 'danger',
        loading: batchLoading,
      }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleBatchAudit"
      @close="batchAudit.visible = false"
    >
      <t-form label-align="top" :data="batchAudit" @submit.prevent>
        <t-form-item label="审核备注（选填，应用于本次全部退款单）" name="remark">
          <t-textarea
            v-model="batchAudit.remark"
            :autosize="{ minRows: 2, maxRows: 4 }"
            :placeholder="batchAudit.approve ? '如：已与客户确认，同意退款' : '如：不符合退款条件，活动商品不退'"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 批量审核结果：逐条回显成功/跳过/失败与原因 -->
    <t-dialog v-model:visible="batchResult.visible" :header="`${batchResult.label}结果`" width="640px" :footer="false">
      <p class="batch-result__summary">
        成功 {{ batchResult.data?.succeeded ?? 0 }} 张 · 跳过 {{ batchResult.data?.skipped ?? 0 }} 张 · 失败
        {{ batchResult.data?.failed ?? 0 }} 张
      </p>
      <t-table
        :data="batchResult.data?.items ?? []"
        :columns="batchResultColumns"
        size="small"
        row-key="id"
        table-layout="fixed"
        max-height="360"
      >
        <template #status="{ row }">
          <t-tag :theme="batchStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ batchStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #message="{ row }">
          <span :class="{ 'batch-result__err': row.status === 'failed' }">{{ row.message || '—' }}</span>
        </template>
      </t-table>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { MoneyIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { batchApproveRefunds, batchRejectRefunds, getRefundList } from '@/api/order'
import {
  channelRefundStatusLabel,
  formatPrice,
  refundModeLabel,
  refundModeOptions,
  refundModeTheme,
  refundStatusLabel,
  refundStatusOptions,
  refundStatusTheme,
} from '@/pages/order/constants'
import type { BatchActionResponse, RefundInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { usePermission } from '@/composables/usePermission'

defineOptions({ name: 'OrderRefunds' })

const router = useRouter()
const { has } = usePermission()

const refundList = ref<RefundInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

// 批量审核：仅待审核的退款单会生效，其余由后端逐条跳过并回显原因。
const selectedIds = ref<Array<string | number>>([])
const batchLoading = ref(false)
const canAudit = computed(() => has('order:refund:audit'))

const filters = reactive<{
  keyword: string | undefined
  order_id: string | undefined
  status: string | undefined
  refund_mode: string | undefined
}>({
  keyword: undefined,
  order_id: undefined,
  status: undefined,
  refund_mode: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护（见 loadUsers/loadData）
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns = computed<PrimaryTableCol<RefundInfo>[]>(() => {
  const base: PrimaryTableCol<RefundInfo>[] = [
    { colKey: 'refund_no', title: '退款单号', minWidth: 180 },
    { colKey: 'order_no', title: '关联订单', minWidth: 170 },
    { colKey: 'amount', title: '金额', width: 120 },
    { colKey: 'refund_mode', title: '退款去向', width: 130 },
    { colKey: 'reason', title: '原因', minWidth: 150 },
    { colKey: 'status', title: '状态', width: 100 },
    { colKey: 'audit_by_name', title: '审核人', width: 100 },
    { colKey: 'audited_at', title: '审核时间', width: 170 },
    { colKey: 'created_at', title: '创建时间', width: 170 },
    {
      colKey: 'action',
      title: '操作',
      width: isMobile.value ? 70 : 130,
      fixed: 'right' as const,
      align: 'center' as const,
    },
  ]
  // 多选列只在桌面端与有审核权限时出现（移动端多选框会挤压操作列）。
  if (!isMobile.value && has('order:refund:audit')) {
    base.unshift({ colKey: 'row-select', type: 'multiple', width: 46, fixed: 'left' as const })
  }
  return base
})

function handleSelectChange(keys: Array<string | number>) {
  selectedIds.value = keys.map((k) => Number(k)).filter((id) => Number.isFinite(id) && id > 0)
}

function clearSelection() {
  selectedIds.value = []
}

function selectedRefundIds(): number[] {
  return selectedIds.value.map((k) => Number(k))
}

// —— 批量审核 ——

const batchAudit = reactive<{ visible: boolean; approve: boolean; remark: string }>({
  visible: false,
  approve: true,
  remark: '',
})
const batchResult = reactive<{ visible: boolean; label: string; data: BatchActionResponse | null }>({
  visible: false,
  label: '',
  data: null,
})

const batchResultColumns: PrimaryTableCol<{ id: number; status: string; message?: string }>[] = [
  { colKey: 'id', title: 'ID', width: 90 },
  { colKey: 'status', title: '结果', width: 100 },
  { colKey: 'message', title: '说明', minWidth: 220 },
]

function batchStatusLabel(status: string): string {
  const map: Record<string, string> = { success: '成功', skipped: '跳过', failed: '失败' }
  return map[status] || status
}

function batchStatusTheme(status: string): string {
  switch (status) {
    case 'success':
      return 'success'
    case 'skipped':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'default'
  }
}

function openBatchAuditDialog(approve: boolean) {
  if (!selectedIds.value.length) return
  batchAudit.approve = approve
  batchAudit.remark = ''
  batchAudit.visible = true
}

async function handleBatchAudit() {
  batchAudit.visible = false
  const label = batchAudit.approve ? '批量通过' : '批量驳回'
  batchLoading.value = true
  try {
    const payload = { ids: selectedRefundIds(), remark: batchAudit.remark }
    const resp = batchAudit.approve ? await batchApproveRefunds(payload) : await batchRejectRefunds(payload)
    batchResult.label = label
    batchResult.data = resp
    batchResult.visible = true
    clearSelection()
    loadRefunds()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `${label}失败`)
  } finally {
    batchLoading.value = false
  }
}

async function loadRefunds() {
  loading.value = true
  try {
    const data = await getRefundList({
      keyword: filters.keyword,
      order_id: filters.order_id ? Number(filters.order_id) : undefined,
      status: filters.status,
      refund_mode: filters.refund_mode,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    refundList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载退款单列表失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadRefunds()
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
  loadRefunds()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.order_id = undefined
  filters.status = undefined
  filters.refund_mode = undefined
  pagination.current = 1
  loadRefunds()
}

function openDetail(row: RefundInfo) {
  router.push(`/orders/refunds/${row.id}`)
}

onMounted(loadRefunds)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, unknown>, row: RefundInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'audit':
      openDetail(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
/* 批量操作条：常态灰色提示，勾选后高亮（与订单列表同款结构）。 */
.batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
  margin-bottom: var(--space-md);
  padding: 10px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  transition:
    border-color var(--hs-duration-fast),
    background-color var(--hs-duration-fast);
}

.batch-bar--active {
  border-color: var(--td-brand-color-3);
  background: var(--td-brand-color-1);
}

.batch-bar__count {
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.batch-bar--active .batch-bar__count {
  color: #3f5848;
  font-weight: 600;
}

.batch-result__summary {
  margin: 0 0 var(--space-md);
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.batch-result__err {
  color: var(--td-error-color-6);
}
</style>
