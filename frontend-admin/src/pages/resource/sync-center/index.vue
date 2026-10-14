<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <RefreshIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">同步运维控制台</h2>
          <p class="page-header__desc">
            两条链路分开看：<b>入站同步</b>（上游 → 我方：拉商品/价格/资源池/实例）与
            <b>出站动作</b>（我方 → 上游：开通·续费·暂停·删除是否到达上游）， 外加实例对账与调度节奏。
          </p>
        </div>
      </div>
      <t-space size="small">
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
      <!-- 链路切换：上游转售与自营平台是两条不同链路，数据必须能分开看。
           放页签之上：它是整个控制台的口径，不只作用于某一个页签。 -->
      <div class="chain-bar">
        <span class="chain-bar__label">链路</span>
        <t-radio-group v-model="chainKind" variant="default-filled" size="small" @change="handleChainChange">
          <t-radio-button v-for="opt in chainKindOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </t-radio-button>
        </t-radio-group>
        <span class="chain-bar__hint">
          {{ chainHint }}
        </span>
      </div>

      <t-tabs v-model="activeTab" theme="card" @change="handleTabChange">
        <!-- 1. 概览：待处理事项 + 渠道同步健康 -->
        <t-tab-panel value="overview" label="概览">
          <OverviewPanel
            v-if="tabLoaded.overview"
            :providers="providers"
            :loading="providerLoading"
            :chain-kind="chainKind"
            :failed-sync-count="failedSyncCount"
            :pending-price-count="pendingPriceCount"
            :open-finding-count="openFindingCount"
            @go-tab="goTab"
          />
        </t-tab-panel>

        <!-- 2. 入站同步（上游 → 我方）：同步记录 + 待确认调价 -->
        <t-tab-panel value="inbound" :label="inboundTabLabel">
          <InboundPanel
            v-if="tabLoaded.inbound"
            ref="inboundRef"
            :chain-kind="chainKind"
            :providers="providers"
            :scopes="scopeOptions"
            :show-catalog="showCatalog"
            @refresh-price-count="onPriceCount"
            @refresh-diffs="loadOverviewCounts"
          />
        </t-tab-panel>

        <!-- 3. 出站动作（我方 → 上游）：开通/续费/暂停/删除是否到达上游 -->
        <t-tab-panel value="outbound" label="出站动作">
          <OutboundPanel v-if="tabLoaded.outbound" ref="outboundRef" :chain-kind="chainKind" embedded />
        </t-tab-panel>

        <!-- 4. 对账与差异：差异台账 + 实例对账（本地 vs 上游逐台比对） -->
        <t-tab-panel value="reconcile" label="对账与差异">
          <ReconcilePanel v-if="tabLoaded.reconcile" ref="reconcileRef" :chain-kind="chainKind" embedded />
        </t-tab-panel>

        <!-- 5. 调度配置：渠道 × 同步类型的节奏与上次结果 -->
        <t-tab-panel value="schedules" label="调度配置">
          <SchedulesPanel
            v-if="tabLoaded.schedules"
            ref="schedulesRef"
            :chain-kind="chainKind"
            :providers="providers"
            @configure="openScheduleDialog"
            @trigger="triggerScope"
          />
        </t-tab-panel>
      </t-tabs>
    </section>

    <!-- 调度配置弹窗 -->
    <t-dialog
      v-model:visible="scheduleDialogVisible"
      :header="
        editingSchedule
          ? `调度配置 · ${editingSchedule.provider_name || '渠道 #' + editingSchedule.provider_id} · ${editingSchedule.scope_name}`
          : '调度配置'
      "
      width="520px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: scheduleSaving }"
      @confirm="handleSaveSchedule"
      @close="scheduleDialogVisible = false"
    >
      <t-form label-align="top" :data="scheduleForm" @submit.prevent>
        <t-form-item label="同步间隔" :help="intervalHelp">
          <IntervalField v-model:seconds="scheduleForm.interval_seconds" :min-seconds="30" />
        </t-form-item>
        <t-form-item label="全量对账周期" :help="fullIntervalHelp">
          <IntervalField v-model:seconds="scheduleForm.full_sync_interval_seconds" :min-seconds="300" />
        </t-form-item>
        <t-form-item label="优先级" help="数值越大越先触发；多条调度同时到期时按此排序">
          <t-input-number v-model="scheduleForm.priority" :min="0" theme="column" />
        </t-form-item>
        <t-form-item label="启用">
          <t-switch v-model="scheduleForm.enabled" />
        </t-form-item>
        <t-form-item label="允许执行时段" help="限定只在某几个小时之间执行，避开上游业务高峰；关闭表示全天可执行">
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
        <t-form-item
          label="立即生效"
          help="开启后保存即把下次执行时间置空，调度器下一轮扫描（每分钟一次）就会触发，不用等原定周期"
        >
          <t-switch v-model="scheduleForm.reset_next_run" />
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
        <t-form-item label="所属渠道" :help="showCatalog ? '' : '自营平台没有上游商品目录，通常只同步实例状态'">
          <t-select v-model="createForm.provider_id" :options="providerOptions" placeholder="请选择渠道" />
        </t-form-item>
        <t-form-item label="同步类型">
          <t-select v-model="createForm.task_type" :options="scopeOptions" placeholder="请选择同步类型" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { createSyncTask, getProviderList, getSyncScopeMeta, updateSyncSchedule } from '@/api/admin'
import type { ProviderInfo, SyncScheduleInfo } from '@/types/interface'
import { chainKindOptions, hasUpstreamCatalog, normalizeChainKind, type ChainKind } from './chain-kind'
import InboundPanel from './components/InboundPanel.vue'
import IntervalField from './components/IntervalField.vue'
import OutboundPanel from '@/pages/resource/task-queue/index.vue'
import OverviewPanel from './components/OverviewPanel.vue'
import ReconcilePanel from '@/pages/resource/reconcile/index.vue'
import SchedulesPanel from './components/SchedulesPanel.vue'

defineOptions({ name: 'ResourceSyncCenter' })

// 页签顺序 = 阅读顺序：先看总览有没有事要做 → 再看两个方向的数据流 → 最后是对账与节奏配置。
type TabValue = 'overview' | 'inbound' | 'outbound' | 'reconcile' | 'schedules'

// 五个页签里四个是独立子组件（各自数据加载与分页），用 v-if 懒挂载：
// 一次性挂载会让首屏并发发出七八个请求。
const tabLoaded = reactive<Record<string, boolean>>({})
function markLoaded(tab: string) {
  tabLoaded[tab] = true
}

const activeTab = ref<TabValue>('overview')
const loading = ref(false)

// ---------- 链路筛选 ----------

const chainKind = ref<ChainKind>('')

/** 链路说明：切换后用户要知道当前视图包含了什么、排除了什么。 */
const chainHint = computed(() => {
  if (chainKind.value === 'upstream') return '只看上游转售渠道：目录、价格与生命周期以对上为准'
  if (chainKind.value === 'compute') return '只看自营平台：本地定价与售卖，平台仅作开通执行器'
  return '两条链路的数据都显示；用渠道列上的标签区分'
})

const showCatalog = computed(() => hasUpstreamCatalog(chainKind.value))

// ---------- 渠道 / scope 元数据 ----------

const providers = ref<ProviderInfo[]>([])
const providerLoading = ref(false)
const scopeOptions = ref<{ label: string; value: string }[]>([])

/** 渠道下拉：按当前链路收敛，避免在"只看自营"时还能选到上游渠道。 */
const providerOptions = computed(() =>
  (chainKind.value ? providers.value.filter((p) => p.kind === chainKind.value) : providers.value).map((item) => ({
    label: item.name,
    value: item.id,
  })),
)

async function loadProviders() {
  providerLoading.value = true
  try {
    const data = await getProviderList({ page: 1, page_size: 200 })
    providers.value = data.items
  } catch {
    /* 渠道下拉失败不阻塞控制台 */
  } finally {
    providerLoading.value = false
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

// ---------- 概览计数 ----------

// 失败同步数：概览卡片的输入，取「失败状态的同步运行条数」。
// 卡片点进去就是「入站同步 + 仅看失败」，两边必须是同一个口径；
// 若这里改数「熔断渠道数」，点进去看到的条数会对不上，用户会以为统计坏了。
const failedSyncCount = ref(0)
const pendingPriceCount = ref(0)
const openFindingCount = ref(0)

function onPriceCount(count: number) {
  pendingPriceCount.value = count
}

/**
 * 概览计数：失败同步条数 + 未闭环差异条数。
 *
 * 单独查而不是复用各面板状态：概览页签可能在那些面板懒挂载之前就被打开，
 * 计数必须独立可得。两个查询都只要 total（page_size=1），代价可忽略。
 */
async function loadOverviewCounts() {
  const { getReconcileFindings, getSyncTaskList } = await import('@/api/admin')
  const kind = chainKind.value || undefined
  const [failed, findings] = await Promise.allSettled([
    getSyncTaskList({ status: 'failed', provider_kind: kind, page: 1, page_size: 1 }),
    getReconcileFindings({ provider_kind: kind, page: 1, page_size: 1 }),
  ])
  failedSyncCount.value = failed.status === 'fulfilled' ? failed.value.meta.total : 0
  openFindingCount.value = findings.status === 'fulfilled' ? findings.value.total : 0
}

const inboundTabLabel = computed(() =>
  pendingPriceCount.value > 0 ? `入站同步（待办 ${pendingPriceCount.value}）` : '入站同步',
)

// ---------- 子组件引用 ----------

const inboundRef = ref<InstanceType<typeof InboundPanel> | null>(null)
const outboundRef = ref<InstanceType<typeof OutboundPanel> | null>(null)
const reconcileRef = ref<InstanceType<typeof ReconcilePanel> | null>(null)
const schedulesRef = ref<InstanceType<typeof SchedulesPanel> | null>(null)

// ---------- 页签切换与刷新 ----------

function goTab(tab: string) {
  if (!['overview', 'inbound', 'outbound', 'reconcile', 'schedules'].includes(tab)) return
  activeTab.value = tab as TabValue
  markLoaded(tab)
  loadActive()
}

function loadActive() {
  const tab = activeTab.value
  markLoaded(tab)
  if (tab === 'inbound') inboundRef.value?.reload()
  else if (tab === 'outbound') outboundRef.value?.reload()
  else if (tab === 'reconcile') reconcileRef.value?.reload()
  else if (tab === 'schedules') schedulesRef.value?.reload()
  else {
    loadProviders()
    loadOverviewCounts()
  }
}

function handleTabChange(value: string | number) {
  activeTab.value = value as TabValue
  loadActive()
}

async function reloadActive() {
  loading.value = true
  try {
    if (activeTab.value === 'overview') {
      await Promise.all([loadProviders(), loadOverviewCounts()])
    } else {
      loadActive()
    }
  } finally {
    loading.value = false
  }
}

/** 链路切换：所有页签的数据口径都变了，已挂载的面板逐个按新链路重查。 */
async function handleChainChange() {
  syncChainToUrl()
  await Promise.all([loadOverviewCounts(), inboundRef.value?.reloadForChain()])
  if (activeTab.value === 'outbound') outboundRef.value?.reload()
  else if (activeTab.value === 'reconcile') reconcileRef.value?.reload()
  else if (activeTab.value === 'schedules') schedulesRef.value?.reload()
}

// ---------- 调度配置弹窗 ----------

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

const syncingScope = ref('')

/**
 * 两个周期的解释按同步类型分别写。
 *
 * 原来只有一句「目录/价格建议小时级，实例建议分钟级」，用户看不出这两个字段
 * 到底在同步什么、为什么默认值差这么多。按类型给具体建议更省事：
 * 实例状态变化快（开关机、到期），目录与价格变化慢且上游接口更重。
 */
const intervalHelp = computed(() => {
  const scope = editingSchedule.value?.scope
  const base: Record<string, string> = {
    catalog: '多久拉一次上游商品目录（新增/改配/下架）。目录变动慢，建议 6 小时~1 天。',
    price: '多久核对一次上游价格。上游调价会进「待确认调价」，建议 6 小时~1 天。',
    pool: '多久刷新一次资源池容量（CPU/内存/磁盘用量）。建议 1~6 小时。',
    region: '多久刷新一次区域与节点清单。区域变动很少，建议 1 天。',
    instance: '多久拉一次实例状态、IP 与到期时间。状态变化快，建议 5~15 分钟。',
  }
  const hint = (scope && base[scope]) || '每隔多久向上游拉取一次本类型的数据。'
  return `${hint} 间隔越短越及时，但上游接口调用越频繁；小于 30 秒会被拒绝。`
})

const fullIntervalHelp = computed(() => {
  const scope = editingSchedule.value?.scope
  const base: Record<string, string> = {
    catalog: '多久做一次全量对账：找出上游已删除、但我们这边还在的商品并标记下架。建议 1 天。',
    price: '多久做一次全量价格核对（不只看有变化的）。建议 1 天。',
    pool: '多久做一次全量池对账：上游已不存在的池会被标记离线。建议 1 天。',
    region: '多久做一次全量区域对账。建议 1 天。',
    instance: '多久做一次全量实例对账：上游已不存在的实例会被标记离线。建议 1 天。',
  }
  const hint = (scope && base[scope]) || '全量对账的周期。'
  return `${hint} 它比「同步间隔」慢得多，用来兜住增量同步发现不了的删除类变化；不得小于 5 分钟。`
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
    schedulesRef.value?.reload()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存调度配置失败')
  } finally {
    scheduleSaving.value = false
  }
}

// ---------- 手动触发同步 ----------

const createVisible = ref(false)
const createForm = reactive<{ provider_id: number | undefined; task_type: string | undefined }>({
  provider_id: undefined,
  task_type: undefined,
})

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
    MessagePlugin.success('同步任务已创建，稍后到「入站同步」查看结果')
    createVisible.value = false
    activeTab.value = 'inbound'
    markLoaded('inbound')
    // 任务异步执行，稍等一拍再拉，避免刚创建就查还是 pending。
    setTimeout(() => inboundRef.value?.reload(), 1200)
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
    setTimeout(() => schedulesRef.value?.reload(), 1500)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '触发同步失败')
  } finally {
    syncingScope.value = ''
  }
}

// ---------- 初始化 ----------

/** 链路写进 URL：刷新与分享保持当前视图（否则一切换就"跳回全部"）。 */
function syncChainToUrl() {
  const url = new URL(window.location.href)
  if (chainKind.value) url.searchParams.set('kind', chainKind.value)
  else url.searchParams.delete('kind')
  window.history.replaceState({}, '', url.toString())
}

// 旧书签兼容：?tab=<旧值> 映射到新的五页签口径。
const legacyTabMap: Record<string, TabValue> = {
  schedules: 'schedules',
  tasks: 'inbound',
  logs: 'inbound',
  prices: 'inbound',
  actions: 'outbound',
  reconcile: 'reconcile',
  anomalies: 'inbound',
  overview: 'overview',
}

onMounted(async () => {
  const params = new URLSearchParams(window.location.search)
  chainKind.value = normalizeChainKind(params.get('kind'))
  const requested = params.get('tab')
  if (requested && legacyTabMap[requested]) {
    activeTab.value = legacyTabMap[requested]
  }
  await Promise.all([loadProviders(), loadScopes(), loadOverviewCounts()])
  markLoaded(activeTab.value)
  loadActive()
  // 旧「异常与失败」入口落进来时直接切到只看失败，保持那个书签的实际意图。
  if (requested === 'anomalies' || requested === 'logs') {
    setTimeout(() => inboundRef.value?.showFailedOnly?.(), 300)
  }
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.page-header__desc {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

/* 链路切换条：页签之上，属于整个控制台的口径。 */
.chain-bar {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
  padding: 0 0 var(--space-md);
  margin-bottom: var(--space-md);
  border-bottom: 1px solid var(--color-border);
}

.chain-bar__label {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.chain-bar__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.form-hint {
  margin-left: 8px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
