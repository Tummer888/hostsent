<template>
  <div class="page-body notify-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <RootListIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">发送日志</h2>
          <p class="page-header__desc">
            一行 = 一个「渠道 × 目标」。<b>已跳过</b>是配置类问题（无渠道/未接入），不重试；<b>失败</b>是服务商/网络问题，按指数退避重试，耗尽转死信。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button
          v-if="canRetry && selectedIds.length"
          theme="warning"
          :loading="batchRetrying"
          @click="handleBatchRetry"
        >
          批量重投（{{ selectedIds.length }}）
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadData">
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
          <span class="field__label">事件</span>
          <t-input v-model="filters.event" placeholder="如：order.paid" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">通道</span>
          <t-select v-model="filters.channel" clearable placeholder="全部通道" :options="deliveryChannelOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.send_status" clearable placeholder="全部状态" :options="deliveryStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">批次号</span>
          <t-input v-model="filters.batch_id" placeholder="群发批次，如 BC..." clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="收件地址 / 标题" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">时间区间</span>
          <t-date-range-picker v-model="dateRange" clearable allow-input @change="handleSearch" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">投递记录</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        :selected-row-keys="selectedIds"
        @select-change="handleSelectChange"
        @page-change="handlePageChange"
      >
        <template #id="{ row }">
          <span class="cell-muted">{{ row.id }}</span>
        </template>
        <template #batch_id="{ row }">
          <span class="cell-muted">{{ row.batch_id || '—' }}</span>
        </template>
        <template #event="{ row }">
          <span class="cell-strong">{{ row.event || '—' }}</span>
        </template>
        <template #channel="{ row }">
          <t-tag :theme="deliveryChannelTheme(row.channel)" variant="light" size="small" shape="round">
            {{ deliveryChannelLabel(row.channel) }}
          </t-tag>
        </template>
        <template #recipient="{ row }">
          <span class="cell-muted">{{ row.recipient || row.target_name || '—' }}</span>
        </template>
        <template #send_status="{ row }">
          <t-tag :theme="deliveryStatusTheme(row.send_status)" variant="light" size="small" shape="round">
            {{ deliveryStatusLabel(row.send_status) }}
          </t-tag>
        </template>
        <template #attempts="{ row }">
          <span class="cell-muted">{{ row.attempts }} / {{ row.max_attempts }}</span>
        </template>
        <template #provider_code="{ row }">
          <span class="cell-muted">{{ row.provider_code || '—' }}</span>
        </template>
        <template #fail_reason="{ row }">
          <span
            class="cell-muted fail-ellipsis"
            :title="row.fail_reason"
          >{{ row.fail_reason || '—' }}</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #sent_at="{ row }">
          <span class="time-text">{{ formatTime(row.sent_at) }}</span>
        </template>
        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail' },
                { content: '重投', value: 'retry', theme: 'warning', hidden: () => !(canRetry && deliveryRetryable(row.send_status)) },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link
                v-if="canRetry && deliveryRetryable(row.send_status)"
                theme="warning"
                hover="color"
                @click="handleRetry(row)"
              >
                重投
              </t-link>
            </template>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无投递记录：业务通知外发后会在此出现" />
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

    <!-- 详情抽屉 -->
    <t-drawer
      v-model:visible="detailVisible"
      header="投递详情"
      size="620px"
      :footer="false"
      :loading="detailLoading"
    >
      <template v-if="detail">
        <div class="detail-block">
          <div class="recon-row"><span class="recon-row__label">记录 ID</span><span class="recon-row__value">{{ detail.id }}</span></div>
          <div class="recon-row"><span class="recon-row__label">批次号</span><span class="recon-row__value">{{ detail.batch_id || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">事件</span><span class="recon-row__value">{{ detail.event || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">通道</span>
            <span class="recon-row__value">
              <t-tag :theme="deliveryChannelTheme(detail.channel)" variant="light" size="small">{{ deliveryChannelLabel(detail.channel) }}</t-tag>
            </span>
          </div>
          <div class="recon-row"><span class="recon-row__label">收件人</span><span class="recon-row__value">{{ detail.recipient || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">状态</span>
            <span class="recon-row__value">
              <t-tag :theme="deliveryStatusTheme(detail.send_status)" variant="light" size="small">{{ deliveryStatusLabel(detail.send_status) }}</t-tag>
            </span>
          </div>
          <div class="recon-row"><span class="recon-row__label">尝试次数</span><span class="recon-row__value">{{ detail.attempts }} / {{ detail.max_attempts }}</span></div>
          <div v-if="detail.next_retry_at" class="recon-row"><span class="recon-row__label">下次重试</span><span class="recon-row__value">{{ formatTime(detail.next_retry_at) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">服务商消息号</span><span class="recon-row__value">{{ detail.provider_msg_id || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">服务商返回码</span><span class="recon-row__value">{{ detail.provider_code || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">费用</span><span class="recon-row__value">{{ detail.cost_fen > 0 ? `${fenToYuan(detail.cost_fen)} 元` : '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">来源模块</span><span class="recon-row__value">{{ detail.source_module || '—' }} {{ detail.source_id ? `#${detail.source_id}` : '' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">创建时间</span><span class="recon-row__value">{{ formatTime(detail.created_at) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">发送时间</span><span class="recon-row__value">{{ formatTime(detail.sent_at) }}</span></div>
        </div>

        <div v-if="detail.fail_reason" class="detail-block">
          <h4 class="detail-block__title">失败原因</h4>
          <p class="fail-text">{{ detail.fail_reason }}</p>
        </div>

        <div class="detail-block">
          <h4 class="detail-block__title">标题</h4>
          <p class="detail-text">{{ detail.title || '—' }}</p>
        </div>

        <div class="detail-block">
          <h4 class="detail-block__title">渲染后内容</h4>
          <pre class="detail-content">{{ detail.content || '—' }}</pre>
        </div>

        <div class="detail-block">
          <h4 class="detail-block__title">模板变量（JSON）</h4>
          <pre class="detail-content">{{ varsJson }}</pre>
        </div>

        <div class="detail-actions">
          <t-button
            v-if="canRetry && deliveryRetryable(detail.send_status)"
            theme="warning"
            @click="handleRetry(detail)"
          >
            重投该记录
          </t-button>
        </div>
      </template>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { RefreshIcon, RootListIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  batchRetryDeliveries,
  getDeliveries,
  getDelivery,
  retryDelivery,
  type DeliveryItem,
} from '@/api/notification'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import {
  deliveryChannelLabel,
  deliveryChannelOptions,
  deliveryChannelTheme,
  deliveryRetryable,
  deliveryStatusLabel,
  deliveryStatusOptions,
  deliveryStatusTheme,
  fenToYuan,
  formatTime,
} from '@/pages/notification/constants'
import { useUserStore } from '@/store'

defineOptions({ name: 'NotifyDeliveries' })

const userStore = useUserStore()
const canRetry = computed(() => userStore.permissions?.includes('notify:delivery') || userStore.isSuperAdmin)
const { isMobile } = useIsMobile()

const loading = ref(false)
const list = ref<DeliveryItem[]>([])
const total = ref(0)
const dateRange = ref<string[] | null>(null)

const filters = reactive({
  event: '',
  channel: undefined as string | undefined,
  send_status: undefined as string | undefined,
  batch_id: '',
  keyword: '',
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns = computed<PrimaryTableCol<DeliveryItem>[]>(() => {
  const base: PrimaryTableCol<DeliveryItem>[] = []
  if (canRetry.value) {
    base.push({ colKey: 'row-select', type: 'multiple', width: 46, fixed: 'left' as const })
  }
  base.push(
    { colKey: 'id', title: 'ID', width: 80 },
    { colKey: 'batch_id', title: '批次', width: 130 },
    { colKey: 'event', title: '事件', width: 150 },
    { colKey: 'channel', title: '通道', width: 90 },
    { colKey: 'recipient', title: '目标', minWidth: 160 },
    { colKey: 'send_status', title: '状态', width: 100 },
    { colKey: 'attempts', title: '尝试', width: 80, align: 'center' as const },
    { colKey: 'provider_code', title: '服务商码', width: 110 },
    { colKey: 'fail_reason', title: '失败原因', minWidth: 160 },
    { colKey: 'created_at', title: '创建时间', width: 160 },
    { colKey: 'sent_at', title: '发送时间', width: 160 },
    { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 110, fixed: 'right' as const, align: 'center' as const },
  )
  return base
})

async function loadData() {
  loading.value = true
  try {
    const data = await getDeliveries({
      event: filters.event || undefined,
      channel: filters.channel,
      send_status: filters.send_status,
      batch_id: filters.batch_id || undefined,
      keyword: filters.keyword || undefined,
      start_time: dateRange.value?.[0] || undefined,
      end_time: dateRange.value?.[1] || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items || []
    total.value = data.meta?.total ?? 0
    pagination.total = total.value
    mobilePage.total = total.value
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载发送日志失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  selectedIds.value = []
  loadData()
}

function handleReset() {
  filters.event = ''
  filters.channel = undefined
  filters.send_status = undefined
  filters.batch_id = ''
  filters.keyword = ''
  dateRange.value = null
  pagination.current = 1
  selectedIds.value = []
  loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadData()
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

// ===== 选择 / 批量重投 =====
const selectedIds = ref<number[]>([])
const batchRetrying = ref(false)

function handleSelectChange(keys: Array<string | number>) {
  // 只允许勾选可重投状态，避免把 sent 一起提交（后端也会逐条拒绝）。
  selectedIds.value = keys.map((k) => Number(k)).filter((id) => {
    const row = list.value.find((item) => item.id === id)
    return row ? deliveryRetryable(row.send_status) : false
  })
}

function handleBatchRetry() {
  if (!selectedIds.value.length) return
  const dialog = DialogPlugin.confirm({
    header: '批量重投',
    body: `确认重投选中的 ${selectedIds.value.length} 条投递记录吗？不可重投的记录会被跳过。`,
    confirmBtn: { content: '确认重投', theme: 'warning' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      batchRetrying.value = true
      try {
        const resp = await batchRetryDeliveries(selectedIds.value)
        const skipped = resp.skipped?.length ? `，跳过 ${resp.skipped.length} 条` : ''
        MessagePlugin.success(`已重投 ${resp.retried} 条${skipped}`)
        dialog.destroy()
        selectedIds.value = []
        loadData()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '批量重投失败')
      } finally {
        batchRetrying.value = false
      }
    },
    onClose: () => dialog.destroy(),
  })
}

// ===== 单条重投 =====
function handleRetry(row: DeliveryItem) {
  const dialog = DialogPlugin.confirm({
    header: '重投',
    body: `确认重投记录 #${row.id}（${deliveryChannelLabel(row.channel)} → ${row.recipient || row.target_name}）吗？`,
    confirmBtn: { content: '确认重投', theme: 'warning' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await retryDelivery(row.id)
        MessagePlugin.success('已重新入队')
        dialog.destroy()
        loadData()
        if (detailVisible.value) openDetail(row)
      } catch (error) {
        MessagePlugin.error((error as Error).message || '重投失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

// ===== 详情 =====
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<DeliveryItem | null>(null)

const varsJson = computed(() => {
  const vars = detail.value?.vars
  if (!vars || !Object.keys(vars).length) return '{}'
  try {
    return JSON.stringify(vars, null, 2)
  } catch {
    return '{}'
  }
})

async function openDetail(row: DeliveryItem) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = row
  try {
    // 详情接口返回完整渲染内容与变量；列表项可能被后端截断。
    detail.value = await getDelivery(row.id)
  } catch {
    // 详情拉取失败时保留列表行数据，不阻塞查看。
  } finally {
    detailLoading.value = false
  }
}

function handleMobileAction(value: string | number | Record<string, any>, row: DeliveryItem) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'retry':
      handleRetry(row)
      break
  }
}

onMounted(loadData)
</script>

<style lang="css" scoped>
.fail-ellipsis {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-block {
  margin-bottom: var(--space-lg);
}

.detail-block__title {
  margin: 0 0 var(--space-sm);
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.detail-text {
  margin: 0;
  font-size: 13px;
  color: #334155;
}

.detail-content {
  margin: 0;
  padding: 10px 12px;
  background: var(--hs-surface-2, #f4f6fa);
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.6;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 260px;
  overflow-y: auto;
}

.fail-text {
  margin: 0;
  color: var(--td-error-color, #d54941);
  font-size: 13px;
  word-break: break-word;
}

.detail-actions {
  padding-top: var(--space-sm);
}
</style>

<style lang="css">
@import '../shared.css';
</style>
