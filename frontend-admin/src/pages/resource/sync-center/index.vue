<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <RefreshIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">同步与调度</h2>
          <p class="page-header__desc">
            渠道 × scope 独立节奏 · 增量游标与全量对账 · 上游改价待确认 · 字段级差异
          </p>
        </div>
      </div>
      <t-space>
        <t-button variant="outline" :loading="loading" @click="reloadActive">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreateDialog">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          手动触发同步
        </t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" theme="card" @change="handleTabChange">
        <!-- 1. 调度配置 -->
        <t-tab-panel value="schedules" label="调度配置">
          <div class="tab-toolbar">
            <div class="field">
              <span class="field__label">所属渠道</span>
              <t-select
                v-model="scheduleFilterProvider"
                clearable
                placeholder="全部渠道"
                :options="providerOptions"
                @change="loadSchedules"
              />
            </div>
            <span class="tab-toolbar__meta">共 {{ scheduleList.length }} 条调度</span>
          </div>
          <t-table
            row-key="id"
            :data="scheduleList"
            :columns="scheduleColumns"
            :loading="scheduleLoading"
            size="small"
            hover
            table-layout="fixed"
            cell-empty-content="—"
          >
            <template #provider_name="{ row }">
              <span class="cell-strong">{{ row.provider_name || `渠道 #${row.provider_id}` }}</span>
            </template>
            <template #scope="{ row }">
              <t-tag :theme="scopeTheme(row.scope)" variant="light" size="small" shape="round">
                {{ row.scope_name || row.scope }}
              </t-tag>
            </template>
            <template #interval="{ row }">
              <span class="cell-num">{{ humanInterval(row.interval_seconds) }}</span>
            </template>
            <template #full_interval="{ row }">
              <span class="cell-muted">{{ humanInterval(row.full_sync_interval_seconds) }}</span>
            </template>
            <template #window="{ row }">
              <span class="cell-muted">{{ windowLabel(row) }}</span>
            </template>
            <template #enabled="{ row }">
              <t-tag :theme="row.enabled ? 'success' : 'default'" variant="light" size="small" shape="round">
                {{ row.enabled ? '启用' : '停用' }}
              </t-tag>
            </template>
            <template #last_status="{ row }">
              <t-tag :theme="scheduleStatusTheme(row.last_status)" variant="light" size="small" shape="round">
                {{ scheduleStatusLabel(row.last_status) }}
              </t-tag>
            </template>
            <template #next_run_at="{ row }">
              <span class="cell-muted">{{ row.next_run_at ? formatTime(row.next_run_at) : '待触发' }}</span>
            </template>
            <template #action="{ row }">
              <div class="action-cell">
                <MobileAction
                  v-if="isMobile"
                  :options="buildMobileActionOptions([
                    { content: '配置', value: 'schedule' },
                    { content: '立即同步', value: 'sync', disabled: () => syncingScope === `${row.provider_id}-${row.scope}` },
                  ])"
                  @select="(value) => handleMobileAction(value, row)"
                />
                <template v-else>
                  <t-link theme="primary" hover="color" @click="openScheduleDialog(row)">配置</t-link>
                  <t-link
                    theme="primary"
                    hover="color"
                    :disabled="syncingScope === `${row.provider_id}-${row.scope}`"
                    @click="triggerScope(row)"
                  >
                    立即同步
                  </t-link>
                </template>
              </div>
            </template>
            <template #empty>
              <t-empty description="暂无调度配置，请先启用渠道同步" />
            </template>
          </t-table>
        </t-tab-panel>

        <!-- 2. 同步任务 -->
        <t-tab-panel value="tasks" label="同步任务">
          <div class="tab-toolbar">
            <div class="field">
              <span class="field__label">所属渠道</span>
              <t-select v-model="taskFilter.provider_id" clearable placeholder="全部渠道" :options="providerOptions" />
            </div>
            <div class="field">
              <span class="field__label">同步类型</span>
              <t-select v-model="taskFilter.task_type" clearable placeholder="全部类型" :options="scopeOptions" />
            </div>
            <div class="field">
              <span class="field__label">状态</span>
              <t-select v-model="taskFilter.status" clearable placeholder="全部状态" :options="taskStatusOptions" />
            </div>
          </div>
          <div class="tab-toolbar__actions">
            <t-space size="small">
              <t-button theme="primary" @click="searchTasks">查询</t-button>
              <t-button variant="outline" @click="resetTaskFilters">重置</t-button>
            </t-space>
          </div>
          <t-table
            row-key="id"
            :data="taskList"
            :columns="taskColumns"
            :loading="taskLoading"
            size="small"
            hover
            table-layout="fixed"
            cell-empty-content="—"
            :pagination="taskPagination"
            @page-change="handleTaskPageChange"
          >
            <template #task_type="{ row }">
              <t-tag :theme="scopeTheme(row.task_type)" variant="light" size="small" shape="round">
                {{ scopeLabel(row.task_type) }}
              </t-tag>
            </template>
            <template #result="{ row }">
              <span class="cell-num">{{ row.success_count }} / {{ row.total_count }}</span>
            </template>
            <template #status="{ row }">
              <t-tag :theme="scheduleStatusTheme(row.status)" variant="light" size="small" shape="round">
                {{ scheduleStatusLabel(row.status) }}
              </t-tag>
            </template>
            <template #error_message="{ row }">
              <span class="cell-error">{{ row.error_message || '—' }}</span>
            </template>
            <template #created_at="{ row }">
              <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无同步任务" />
            </template>
          </t-table>
        </t-tab-panel>

        <!-- 3. 同步日志 -->
        <t-tab-panel value="logs" label="同步日志">
          <t-table
            row-key="id"
            :data="logList"
            :columns="logColumns"
            :loading="logLoading"
            size="small"
            hover
            table-layout="fixed"
            cell-empty-content="—"
            :pagination="logPagination"
            @page-change="handleLogPageChange"
          >
            <template #sync_type="{ row }">
              <t-tag :theme="scopeTheme(row.sync_type)" variant="light" size="small" shape="round">
                {{ scopeLabel(row.sync_type) }}
              </t-tag>
            </template>
            <template #result="{ row }">
              <span class="cell-num">{{ row.success_count }} / {{ row.total_count }}</span>
            </template>
            <template #status="{ row }">
              <t-tag :theme="scheduleStatusTheme(row.status)" variant="light" size="small" shape="round">
                {{ scheduleStatusLabel(row.status) }}
              </t-tag>
            </template>
            <template #created_at="{ row }">
              <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无同步日志" />
            </template>
          </t-table>
        </t-tab-panel>

        <!-- 4. 差异记录 / 对账 -->
        <t-tab-panel value="diffs" label="差异与对账">
          <div class="diff-summary">
            <article
              v-for="card in diffStatCards"
              :key="card.key"
              class="diff-summary__item"
              :class="`diff-summary__item--${card.variant}`"
            >
              <span class="diff-summary__num">{{ card.value }}</span>
              <span class="diff-summary__label">{{ card.label }}</span>
            </article>
          </div>

          <div class="tab-toolbar">
            <div class="field">
              <span class="field__label">所属渠道</span>
              <t-select v-model="diffFilter.provider_id" clearable placeholder="全部渠道" :options="providerOptions" @change="loadDiffs" />
            </div>
            <div class="field">
              <span class="field__label">同步类型</span>
              <t-select v-model="diffFilter.scope" clearable placeholder="全部类型" :options="scopeOptions" @change="loadDiffs" />
            </div>
            <div class="field">
              <span class="field__label">差异动作</span>
              <t-select v-model="diffFilter.action" clearable placeholder="全部动作" :options="diffActionOptions" @change="loadDiffs" />
            </div>
          </div>

          <t-table
            row-key="id"
            :data="diffList"
            :columns="diffColumns"
            :loading="diffLoading"
            size="small"
            hover
            table-layout="fixed"
            cell-empty-content="—"
            :pagination="diffPagination"
            @page-change="handleDiffPageChange"
          >
            <template #provider_id="{ row }">
              <span class="cell-strong">{{ providerName(row.provider_id) }}</span>
            </template>
            <template #scope="{ row }">
              <t-tag :theme="scopeTheme(row.scope)" variant="light" size="small" shape="round">
                {{ scopeLabel(row.scope) }}
              </t-tag>
            </template>
            <template #action="{ row }">
              <t-tag :theme="diffActionTheme(row.action)" variant="light" size="small" shape="round">
                {{ diffActionLabel(row.action) }}
              </t-tag>
            </template>
            <template #changed="{ row }">
              <span class="cell-muted">
                <span class="cell-old">{{ row.old_value || '—' }}</span>
                <span class="cell-arrow">→</span>
                <span class="cell-new">{{ row.new_value || '—' }}</span>
              </span>
            </template>
            <template #disposition="{ row }">
              <t-tag :theme="row.disposition === 'pending' ? 'warning' : row.disposition === 'skipped' ? 'default' : 'success'" variant="light" size="small" shape="round">
                {{ dispositionLabel(row.disposition) }}
              </t-tag>
            </template>
            <template #created_at="{ row }">
              <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无差异记录，同步后自动生成" />
            </template>
          </t-table>
        </t-tab-panel>

        <!-- 5. 待确认调价 -->
        <t-tab-panel value="prices" label="待确认调价">
          <div class="tab-toolbar">
            <div class="field">
              <span class="field__label">所属渠道</span>
              <t-select v-model="priceFilter.provider_id" clearable placeholder="全部渠道" :options="providerOptions" @change="searchPrices" />
            </div>
            <div class="field">
              <span class="field__label">状态</span>
              <t-select v-model="priceFilter.status" clearable placeholder="全部状态" :options="priceStatusOptions" @change="searchPrices" />
            </div>
            <t-space size="small">
              <t-button theme="primary" :disabled="!selectedPriceIds.length" @click="openHandleDialog('confirm')">
                批量确认
              </t-button>
              <t-button theme="danger" variant="outline" :disabled="!selectedPriceIds.length" @click="openHandleDialog('reject')">
                批量驳回
              </t-button>
            </t-space>
          </div>

          <p class="notice-line">
            上游改价幅度超过渠道阈值时进入「待确认」，确认前<strong>不会改动售价</strong>；
            确认后按商品加价规则重算售价（未配置规则则仅更新成本价）并写入价格历史。
          </p>

          <t-table
            row-key="id"
            :data="priceList"
            :columns="priceColumns"
            :loading="priceLoading"
            size="small"
            hover
            table-layout="fixed"
            cell-empty-content="—"
            :pagination="pricePagination"
            :selected-row-keys="selectedPriceIds"
            @page-change="handlePricePageChange"
            @select-change="handlePriceSelect"
          >
            <template #provider_id="{ row }">
              <span class="cell-strong">{{ providerName(row.provider_id) }}</span>
            </template>
            <template #upstream_id="{ row }">
              <div class="product-cell">
                <span class="cell-strong">{{ row.upstream_id ? `#${row.upstream_id}` : `#${row.resource_product_id}` }}</span>
                <span class="product-sub">资源商品 #{{ row.resource_product_id }}</span>
              </div>
            </template>
            <template #field="{ row }">
              <t-tag :theme="row.field === 'cost_price' ? 'primary' : 'warning'" variant="light" size="small" shape="round">
                {{ row.field === 'cost_price' ? '成本价' : '建议售价' }}
              </t-tag>
            </template>
            <template #changed="{ row }">
              <span class="cell-num">
                {{ money(row.old_value) }}
                <span class="cell-arrow">→</span>
                {{ money(row.new_value) }}
              </span>
            </template>
            <template #change_ratio="{ row }">
              <span :class="ratioClass(row)">{{ ratioLabel(row) }}</span>
            </template>
            <template #status="{ row }">
              <t-tag :theme="priceStatusTheme(row.status)" variant="light" size="small" shape="round">
                {{ priceStatusLabel(row.status) }}
              </t-tag>
            </template>
            <template #created_at="{ row }">
              <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #handle="{ row }">
              <div v-if="row.status === 'pending'" class="action-cell">
                <t-link theme="primary" hover="color" @click="openHandleDialog('confirm', row)">确认</t-link>
                <t-link theme="danger" hover="color" @click="openHandleDialog('reject', row)">驳回</t-link>
              </div>
              <span v-else class="cell-muted">—</span>
            </template>
            <template #empty>
              <t-empty description="暂无调价事件" />
            </template>
          </t-table>
        </t-tab-panel>
      </t-tabs>
    </section>

    <!-- 调度配置弹窗 -->
    <t-dialog
      v-model:visible="scheduleDialogVisible"
      :header="editingSchedule ? `调度配置 · ${editingSchedule.provider_name || '渠道 #' + editingSchedule.provider_id} · ${editingSchedule.scope_name}` : '调度配置'"
      width="520px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: scheduleSaving }"
      @confirm="handleSaveSchedule"
      @close="scheduleDialogVisible = false"
    >
      <t-form label-align="top" :data="scheduleForm" @submit.prevent>
        <t-form-item label="同步间隔（秒）" help="目录/价格建议小时级（3600），实例建议分钟级（300）">
          <t-input-number v-model="scheduleForm.interval_seconds" :min="30" theme="column" />
        </t-form-item>
        <t-form-item label="全量对账周期（秒）" help="到期后执行软删除对账；商品目录建议 86400">
          <t-input-number v-model="scheduleForm.full_sync_interval_seconds" :min="300" theme="column" />
        </t-form-item>
        <t-form-item label="优先级" help="数值越大越先触发">
          <t-input-number v-model="scheduleForm.priority" :min="0" theme="column" />
        </t-form-item>
        <t-form-item label="启用">
          <t-switch v-model="scheduleForm.enabled" />
        </t-form-item>
        <t-form-item label="允许执行时段">
          <t-switch v-model="scheduleForm.window_enabled" @change="handleWindowToggle" />
        </t-form-item>
        <t-form-item v-if="scheduleForm.window_enabled" label="时段（小时 0-23）">
          <t-space align="center">
            <t-input-number v-model="scheduleForm.window_start" :min="0" :max="23" theme="column" />
            <span>点 至</span>
            <t-input-number v-model="scheduleForm.window_end" :min="0" :max="23" theme="column" />
            <span>点</span>
          </t-space>
        </t-form-item>
        <t-form-item label="立即生效">
          <t-switch v-model="scheduleForm.reset_next_run" />
          <span class="form-hint">开启后保存即把下次执行时间置空，调度器下轮立即触发</span>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 手动触发同步 -->
    <t-dialog
      v-model:visible="createVisible"
      header="手动触发同步"
      width="460px"
      :confirm-btn="{ content: '提交', theme: 'primary' }"
      @confirm="handleCreateTask"
      @close="createVisible = false"
    >
      <t-form label-align="top" :data="createForm" @submit.prevent>
        <t-form-item label="所属渠道">
          <t-select v-model="createForm.provider_id" :options="providerOptions" placeholder="请选择渠道" />
        </t-form-item>
        <t-form-item label="同步类型">
          <t-select v-model="createForm.task_type" :options="scopeOptions" placeholder="请选择同步类型" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 调价确认 / 驳回 -->
    <t-dialog
      v-model:visible="handleDialogVisible"
      :header="handleAction === 'confirm' ? '确认上游调价' : '驳回上游调价'"
      width="460px"
      :confirm-btn="{ content: handleAction === 'confirm' ? '确认应用' : '确认驳回', theme: handleAction === 'confirm' ? 'primary' : 'danger', loading: handleSaving }"
      @confirm="submitHandle"
      @close="handleDialogVisible = false"
    >
      <p class="notice-line">
        共 {{ handleIds.length }} 条事件。
        <template v-if="handleAction === 'confirm'">确认后将按加价规则更新关联商品售价并写入价格历史。</template>
        <template v-else>驳回仅标记事件，不改动任何商品价格。</template>
      </p>
      <t-textarea v-model="handleRemark" placeholder="备注（可选）" :maxlength="200" />
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import {
  MessagePlugin,
  type PageInfo,
  type PrimaryTableCol,
  type SelectOptions,
} from 'tdesign-vue-next'

import {
  createSyncTask,
  getPriceChangeList,
  getProviderList,
  getSyncDiffList,
  getSyncDiffSummary,
  getSyncLogList,
  getSyncScheduleList,
  getSyncScopeMeta,
  getSyncTaskList,
  handlePriceChanges,
  updateSyncSchedule,
} from '@/api/admin'
import type {
  PriceChangeInfo,
  ProviderInfo,
  SyncDiffInfo,
  SyncLogInfo,
  SyncScheduleInfo,
  SyncTaskInfo,
} from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ResourceSyncCenter' })

type TabValue = 'schedules' | 'tasks' | 'logs' | 'diffs' | 'prices'

const activeTab = ref<TabValue>('schedules')
const loading = ref(false)
const { isMobile } = useIsMobile()
const providerOptions = ref<{ label: string; value: number }[]>([])
const providerNameMap = ref<Record<number, string>>({})
const scopeOptions = ref<{ label: string; value: string }[]>([])

// ---------- 通用工具 ----------

function formatTime(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function humanInterval(seconds: number): string {
  if (!seconds || seconds <= 0) return '—'
  if (seconds % 86400 === 0) return `${seconds / 86400} 天`
  if (seconds % 3600 === 0) return `${seconds / 3600} 小时`
  if (seconds % 60 === 0) return `${seconds / 60} 分钟`
  return `${seconds} 秒`
}

function money(value: number | null): string {
  if (value === null || value === undefined) return '—'
  return Number(value).toFixed(2)
}

function providerName(id: number): string {
  return providerNameMap.value[id] || `渠道 #${id}`
}

function scopeLabel(scope: string): string {
  const found = scopeOptions.value.find((item) => item.value === scope)
  return found ? found.label : scope
}

function scopeTheme(scope: string): string {
  if (scope === 'catalog') return 'primary'
  if (scope === 'price') return 'warning'
  if (scope === 'pool') return 'success'
  if (scope === 'region') return 'default'
  if (scope === 'instance') return 'danger'
  return 'default'
}

const statusLabels: Record<string, string> = {
  pending: '待执行',
  running: '执行中',
  success: '成功',
  failed: '失败',
  skipped: '已跳过',
}

function scheduleStatusLabel(status: string): string {
  return statusLabels[status] || status || '—'
}

function scheduleStatusTheme(status: string): string {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  if (status === 'skipped') return 'default'
  return 'default'
}

function windowLabel(row: SyncScheduleInfo): string {
  if (row.window_start === null || row.window_end === null) return '不限'
  return `${row.window_start} 点 - ${row.window_end} 点`
}

// ---------- 渠道 / scope 元数据 ----------

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 200 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
    const map: Record<number, string> = {}
    for (const item of data.items) map[item.id] = item.name
    providerNameMap.value = map
  } catch {
    /* 渠道下拉失败不阻塞 */
  }
}

async function loadScopes() {
  try {
    const data = await getSyncScopeMeta()
    scopeOptions.value = data.map((item) => ({ label: item.name, value: item.scope }))
  } catch {
    scopeOptions.value = [
      { label: '商品目录', value: 'catalog' },
      { label: '价格', value: 'price' },
      { label: '资源池', value: 'pool' },
      { label: '区域', value: 'region' },
      { label: '实例', value: 'instance' },
    ]
  }
}

// ---------- 1. 调度配置 ----------

const scheduleList = ref<SyncScheduleInfo[]>([])
const scheduleLoading = ref(false)
const scheduleFilterProvider = ref<number | undefined>(undefined)

const scheduleColumns: PrimaryTableCol<SyncScheduleInfo>[] = [
  { colKey: 'provider_name', title: '渠道', minWidth: 160 },
  { colKey: 'scope', title: '同步类型', width: 110 },
  { colKey: 'interval', title: '同步间隔', width: 100 },
  { colKey: 'full_interval', title: '全量对账', width: 100 },
  { colKey: 'window', title: '执行时段', width: 110 },
  { colKey: 'priority', title: '优先级', width: 80 },
  { colKey: 'enabled', title: '状态', width: 80 },
  { colKey: 'last_status', title: '上次结果', width: 100 },
  { colKey: 'next_run_at', title: '下次执行', minWidth: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 140, fixed: 'right' as const, align: 'center' as const },
]

async function loadSchedules() {
  scheduleLoading.value = true
  try {
    const data = await getSyncScheduleList({ provider_id: scheduleFilterProvider.value, page: 1, page_size: 200 })
    scheduleList.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载调度配置失败')
  } finally {
    scheduleLoading.value = false
  }
}

const scheduleDialogVisible = ref(false)
const scheduleSaving = ref(false)
const editingSchedule = ref<SyncScheduleInfo | null>(null)
const scheduleForm = reactive({
  interval_seconds: 3600,
  full_sync_interval_seconds: 86400,
  priority: 0,
  enabled: true,
  window_enabled: false,
  window_start: 0,
  window_end: 6,
  reset_next_run: false,
})

function openScheduleDialog(row: SyncScheduleInfo) {
  editingSchedule.value = row
  scheduleForm.interval_seconds = row.interval_seconds
  scheduleForm.full_sync_interval_seconds = row.full_sync_interval_seconds
  scheduleForm.priority = row.priority
  scheduleForm.enabled = row.enabled
  scheduleForm.window_enabled = row.window_start !== null && row.window_end !== null
  scheduleForm.window_start = row.window_start ?? 0
  scheduleForm.window_end = row.window_end ?? 6
  scheduleForm.reset_next_run = false
  scheduleDialogVisible.value = true
}

function handleWindowToggle(value: boolean) {
  scheduleForm.window_enabled = value
}

async function handleSaveSchedule() {
  if (!editingSchedule.value) return
  scheduleSaving.value = true
  try {
    await updateSyncSchedule(editingSchedule.value.id, {
      interval_seconds: scheduleForm.interval_seconds,
      full_sync_interval_seconds: scheduleForm.full_sync_interval_seconds,
      priority: scheduleForm.priority,
      enabled: scheduleForm.enabled,
      window_clear: !scheduleForm.window_enabled,
      window_start: scheduleForm.window_enabled ? scheduleForm.window_start : null,
      window_end: scheduleForm.window_enabled ? scheduleForm.window_end : null,
      reset_next_run: scheduleForm.reset_next_run,
    })
    MessagePlugin.success('调度配置已保存')
    scheduleDialogVisible.value = false
    loadSchedules()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存调度配置失败')
  } finally {
    scheduleSaving.value = false
  }
}

// ---------- 手动触发 ----------

const createVisible = ref(false)
const createForm = reactive<{ provider_id: number | undefined; task_type: string | undefined }>({
  provider_id: undefined,
  task_type: undefined,
})
const syncingScope = ref('')

function openCreateDialog() {
  createForm.provider_id = undefined
  createForm.task_type = undefined
  createVisible.value = true
}

async function handleCreateTask() {
  if (!createForm.provider_id || !createForm.task_type) {
    MessagePlugin.warning('请选择渠道与同步类型')
    return
  }
  try {
    await createSyncTask({ provider_id: createForm.provider_id, task_type: createForm.task_type })
    MessagePlugin.success('同步任务已创建')
    createVisible.value = false
    reloadActive()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建同步任务失败')
  }
}

async function triggerScope(row: SyncScheduleInfo) {
  const key = `${row.provider_id}-${row.scope}`
  if (syncingScope.value === key) return
  syncingScope.value = key
  try {
    await createSyncTask({ provider_id: row.provider_id, task_type: row.scope })
    MessagePlugin.success(`已触发「${row.provider_name || '渠道'}」${row.scope_name}同步`)
    setTimeout(loadSchedules, 1500)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '触发同步失败')
  } finally {
    syncingScope.value = ''
  }
}

// ---------- 2. 同步任务 ----------

const taskList = ref<SyncTaskInfo[]>([])
const taskLoading = ref(false)
const taskStatusOptions = [
  { label: '待执行', value: 'pending' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '已跳过', value: 'skipped' },
]
const taskFilter = reactive<{ provider_id: number | undefined; task_type: string | undefined; status: string | undefined }>({
  provider_id: undefined,
  task_type: undefined,
  status: undefined,
})
const taskPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const taskColumns: PrimaryTableCol<SyncTaskInfo>[] = [
  { colKey: 'id', title: '任务 ID', width: 90 },
  { colKey: 'provider_id', title: '渠道', width: 140 },
  { colKey: 'task_type', title: '类型', width: 110 },
  { colKey: 'result', title: '成功/总数', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'error_message', title: '错误信息', minWidth: 220 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
]

async function loadTasks() {
  taskLoading.value = true
  try {
    const data = await getSyncTaskList({
      provider_id: taskFilter.provider_id,
      task_type: taskFilter.task_type,
      status: taskFilter.status,
      page: taskPagination.current,
      page_size: taskPagination.pageSize,
    })
    taskList.value = data.items
    taskPagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载同步任务失败')
  } finally {
    taskLoading.value = false
  }
}

function searchTasks() {
  taskPagination.current = 1
  loadTasks()
}

function resetTaskFilters() {
  taskFilter.provider_id = undefined
  taskFilter.task_type = undefined
  taskFilter.status = undefined
  searchTasks()
}

function handleTaskPageChange(pageInfo: PageInfo) {
  taskPagination.current = pageInfo.current
  taskPagination.pageSize = pageInfo.pageSize
  loadTasks()
}

// ---------- 3. 同步日志 ----------

const logList = ref<SyncLogInfo[]>([])
const logLoading = ref(false)
const logPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const logColumns: PrimaryTableCol<SyncLogInfo>[] = [
  { colKey: 'id', title: '日志 ID', width: 90 },
  { colKey: 'task_id', title: '任务 ID', width: 90 },
  { colKey: 'provider_id', title: '渠道', width: 140 },
  { colKey: 'sync_type', title: '类型', width: 110 },
  { colKey: 'result', title: '成功/总数', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
]

async function loadLogs() {
  logLoading.value = true
  try {
    const data = await getSyncLogList({ page: logPagination.current, page_size: logPagination.pageSize })
    logList.value = data.items
    logPagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载同步日志失败')
  } finally {
    logLoading.value = false
  }
}

function handleLogPageChange(pageInfo: PageInfo) {
  logPagination.current = pageInfo.current
  logPagination.pageSize = pageInfo.pageSize
  loadLogs()
}

// ---------- 4. 差异与对账 ----------

const diffList = ref<SyncDiffInfo[]>([])
const diffLoading = ref(false)
const diffSummary = ref({ total: 0, created: 0, updated: 0, offline: 0, price_changed: 0 })
const diffFilter = reactive<{ provider_id: number | undefined; scope: string | undefined; action: string | undefined }>({
  provider_id: undefined,
  scope: undefined,
  action: undefined,
})
const diffPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const diffActionOptions = [
  { label: '新增', value: 'created' },
  { label: '字段更新', value: 'updated' },
  { label: '上游下线', value: 'offline' },
  { label: '本地缺失', value: 'missing' },
  { label: '价格变化', value: 'price_changed' },
]

const diffActionLabels: Record<string, string> = {
  created: '新增',
  updated: '字段更新',
  offline: '上游下线',
  missing: '本地缺失',
  price_changed: '价格变化',
}

function diffActionLabel(action: string): string {
  return diffActionLabels[action] || action
}

function diffActionTheme(action: string): string {
  if (action === 'created') return 'success'
  if (action === 'updated') return 'primary'
  if (action === 'offline') return 'danger'
  if (action === 'missing') return 'warning'
  if (action === 'price_changed') return 'warning'
  return 'default'
}

function dispositionLabel(disposition: string): string {
  if (disposition === 'pending') return '待确认'
  if (disposition === 'skipped') return '已跳过'
  return '已应用'
}

const diffStatCards = computed(() => [
  { key: 'total', label: '差异总数', value: diffSummary.value.total, variant: 'total' },
  { key: 'created', label: '新增', value: diffSummary.value.created, variant: 'ok' },
  { key: 'updated', label: '字段更新', value: diffSummary.value.updated, variant: 'muted' },
  { key: 'offline', label: '上游下线', value: diffSummary.value.offline, variant: 'warn' },
  { key: 'price_changed', label: '价格变化', value: diffSummary.value.price_changed, variant: 'warn' },
])

const diffColumns: PrimaryTableCol<SyncDiffInfo>[] = [
  { colKey: 'id', title: '差异 ID', width: 90 },
  { colKey: 'provider_id', title: '渠道', width: 140 },
  { colKey: 'scope', title: '类型', width: 100 },
  { colKey: 'action', title: '动作', width: 100 },
  { colKey: 'external_id', title: '上游 ID', width: 100 },
  { colKey: 'field', title: '字段', width: 100 },
  { colKey: 'changed', title: '变更', minWidth: 200 },
  { colKey: 'disposition', title: '处置', width: 90 },
  { colKey: 'created_at', title: '时间', minWidth: 160 },
]

async function loadDiffs() {
  diffLoading.value = true
  try {
    const [listData, summaryData] = await Promise.all([
      getSyncDiffList({
        provider_id: diffFilter.provider_id,
        scope: diffFilter.scope,
        action: diffFilter.action,
        page: diffPagination.current,
        page_size: diffPagination.pageSize,
      }),
      getSyncDiffSummary({ provider_id: diffFilter.provider_id, days: 7 }),
    ])
    diffList.value = listData.items
    diffPagination.total = listData.meta.total
    const agg = { total: 0, created: 0, updated: 0, offline: 0, price_changed: 0 }
    for (const item of summaryData.items) {
      agg.total += item.total
      for (const [action, count] of Object.entries(item.by_action || {})) {
        if (action in agg) agg[action as keyof typeof agg] += count
      }
    }
    diffSummary.value = agg
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载差异记录失败')
  } finally {
    diffLoading.value = false
  }
}

function handleDiffPageChange(pageInfo: PageInfo) {
  diffPagination.current = pageInfo.current
  diffPagination.pageSize = pageInfo.pageSize
  loadDiffs()
}

// ---------- 5. 待确认调价 ----------

const priceList = ref<PriceChangeInfo[]>([])
const priceLoading = ref(false)
const selectedPriceIds = ref<number[]>([])
const priceFilter = reactive<{ provider_id: number | undefined; status: string | undefined }>({
  provider_id: undefined,
  status: 'pending',
})
const pricePagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const priceStatusOptions = [
  { label: '待确认', value: 'pending' },
  { label: '已确认', value: 'confirmed' },
  { label: '已驳回', value: 'rejected' },
  { label: '自动应用', value: 'auto_applied' },
]

const priceStatusLabels: Record<string, string> = {
  pending: '待确认',
  confirmed: '已确认',
  rejected: '已驳回',
  auto_applied: '自动应用',
}

function priceStatusLabel(status: string): string {
  return priceStatusLabels[status] || status
}

function priceStatusTheme(status: string): string {
  if (status === 'pending') return 'warning'
  if (status === 'confirmed') return 'success'
  if (status === 'rejected') return 'danger'
  return 'default'
}

function ratioLabel(row: PriceChangeInfo): string {
  if (row.change_ratio === null || row.change_ratio === undefined) return '—'
  const pct = Number(row.change_ratio) * 100
  const threshold = row.threshold ? Number(row.threshold) * 100 : 0
  return `${pct.toFixed(2)}%${threshold ? `（阈值 ${threshold.toFixed(2)}%）` : ''}`
}

function ratioClass(row: PriceChangeInfo): string {
  if (row.status === 'pending') return 'cell-ratio cell-ratio--high'
  return 'cell-ratio'
}

const priceColumns: PrimaryTableCol<PriceChangeInfo>[] = [
  { colKey: 'row-select', type: 'multiple', width: 50 },
  { colKey: 'id', title: '事件 ID', width: 90 },
  { colKey: 'provider_id', title: '渠道', width: 140 },
  { colKey: 'upstream_id', title: '上游商品', minWidth: 140 },
  { colKey: 'field', title: '字段', width: 100 },
  { colKey: 'changed', title: '变动', minWidth: 160 },
  { colKey: 'change_ratio', title: '幅度', minWidth: 170 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '时间', minWidth: 160 },
  { colKey: 'handle', title: '操作', width: 110, fixed: 'right' as const, align: 'center' as const },
]

async function loadPrices() {
  priceLoading.value = true
  try {
    const data = await getPriceChangeList({
      provider_id: priceFilter.provider_id,
      status: priceFilter.status,
      page: pricePagination.current,
      page_size: pricePagination.pageSize,
    })
    priceList.value = data.items
    pricePagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载调价事件失败')
  } finally {
    priceLoading.value = false
  }
}

function searchPrices() {
  pricePagination.current = 1
  selectedPriceIds.value = []
  loadPrices()
}

function handlePricePageChange(pageInfo: PageInfo) {
  pricePagination.current = pageInfo.current
  pricePagination.pageSize = pageInfo.pageSize
  loadPrices()
}

function handlePriceSelect(keys: Array<string | number>, options: SelectOptions<PriceChangeInfo>) {
  void options
  selectedPriceIds.value = keys.map((key) => Number(key))
}

const handleDialogVisible = ref(false)
const handleSaving = ref(false)
const handleAction = ref<'confirm' | 'reject'>('confirm')
const handleIds = ref<number[]>([])
const handleRemark = ref('')

function openHandleDialog(action: 'confirm' | 'reject', row?: PriceChangeInfo) {
  handleAction.value = action
  handleIds.value = row ? [row.id] : [...selectedPriceIds.value]
  handleRemark.value = ''
  if (!handleIds.value.length) {
    MessagePlugin.warning('请先选择待处理事件')
    return
  }
  handleDialogVisible.value = true
}

async function submitHandle() {
  if (!handleIds.value.length) return
  handleSaving.value = true
  try {
    const res = await handlePriceChanges({
      ids: handleIds.value,
      action: handleAction.value,
      remark: handleRemark.value,
    })
    MessagePlugin.success(`已处理 ${res.handled} 条调价事件`)
    handleDialogVisible.value = false
    selectedPriceIds.value = []
    loadPrices()
    loadDiffs()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '处理调价事件失败')
  } finally {
    handleSaving.value = false
  }
}

// ---------- Tab 懒加载 ----------

function loadActive() {
  if (activeTab.value === 'schedules') loadSchedules()
  else if (activeTab.value === 'tasks') loadTasks()
  else if (activeTab.value === 'logs') loadLogs()
  else if (activeTab.value === 'diffs') loadDiffs()
  else if (activeTab.value === 'prices') loadPrices()
}

function handleTabChange(value: string | number) {
  activeTab.value = value as TabValue
  loadActive()
}

async function reloadActive() {
  loading.value = true
  try {
    loadActive()
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadProviders(), loadScopes()])
  loadActive()
})

// 移动端操作下拉分发（同步范围配置表）
function handleMobileAction(value: string | number | Record<string, any>, row: SyncScheduleInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'schedule':
      openScheduleDialog(row)
      break
    case 'sync':
      void triggerScope(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, #0284c7, #0369a1);
  --chip-shadow: 0 4px 10px rgba(2, 132, 199, 0.25);
}

.page-header__desc {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.tab-toolbar {
  display: flex;
  align-items: flex-end;
  gap: var(--space-md);
  flex-wrap: wrap;
  padding: var(--space-md) 0 var(--space-lg);
}

.tab-toolbar__actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--td-brand-color-1);
  margin-bottom: 14px;
}

.tab-toolbar .field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 180px;
}

.tab-toolbar__meta {
  margin-left: auto;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.cell-strong {
  font-weight: 600;
  color: var(--color-foreground);
}

.cell-num {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.cell-muted {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.cell-error {
  font-size: 12px;
  color: #dc2626;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.cell-old {
  color: var(--color-muted-foreground);
  text-decoration: line-through;
}

.cell-new {
  color: var(--color-foreground);
}

.cell-arrow {
  margin: 0 6px;
  color: var(--color-muted-foreground);
}

.cell-ratio {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.cell-ratio--high {
  color: #dc2626;
  font-weight: 600;
}

.action-cell {
  display: inline-flex;
  gap: var(--space-md);
  align-items: center;
}

.product-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.product-sub {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.notice-line {
  margin: 0 0 var(--space-md);
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
  background: var(--hs-surface-2);
  border-radius: var(--hs-radius-md);
}

.form-hint {
  margin-left: 8px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.diff-summary {
  display: flex;
  gap: var(--space-md);
  flex-wrap: wrap;
  padding: var(--space-md) 0 0;
}

.diff-summary__item {
  display: flex;
  align-items: baseline;
  gap: var(--space-sm);
  padding: var(--space-sm) var(--space-lg);
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-2);
  border: 1px solid var(--color-border);
  min-width: 110px;
}

.diff-summary__num {
  font-size: 20px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.diff-summary__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.diff-summary__item--ok .diff-summary__num {
  color: #10b981;
}

.diff-summary__item--warn .diff-summary__num {
  color: #ef4444;
}

.diff-summary__item--muted .diff-summary__num {
  color: #6b7280;
}
</style>
