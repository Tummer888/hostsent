<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CloudIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ detail?.name || '提供商详情' }}</h2>
          <p class="page-header__desc">
            <template v-if="detail">类型：{{ typeLabel(detail.provider_type) }} · {{ detail.provider_type }}</template>
            <template v-else>加载提供商信息…</template>
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" @click="router.push(listPath)">返回列表</t-button>
        <t-button v-if="detail?.ops_console_url" variant="outline" @click="openOpsConsole">运维平台</t-button>
        <t-button variant="outline" :loading="testing" @click="handleTestConnection">测试连接</t-button>
        <t-button v-if="detail?.sync_paused" theme="warning" :loading="resuming" @click="handleResumeSync"
          >恢复同步</t-button
        >
        <t-button theme="primary" :loading="saving" @click="toggleEdit">{{ editing ? '保存' : '编辑资料' }}</t-button>
      </t-space>
    </header>

    <section class="tabs-card surface-card">
      <t-tabs v-model="activeTab" theme="card" size="medium">
        <t-tab-panel value="basic" label="基本信息" />
        <t-tab-panel value="capability" label="能力矩阵" />
        <!-- 资源池页签只在渠道具备 pool 同步能力时出现（自营算力平台）。
             上游转售渠道没有本地容量概念，调度与适配器两层都不产生 pool scope，
             给一个永远空表的页签只会让人以为"数据没采到"。 -->
        <t-tab-panel v-if="hasPoolCapability" value="pools" label="资源池" />
        <t-tab-panel value="schedules" label="同步与调度" />
        <t-tab-panel value="logs" label="同步日志" />
      </t-tabs>
    </section>

    <section class="content-card surface-card">
      <!-- 基本信息 -->
      <template v-if="activeTab === 'basic'">
        <t-loading :loading="loading" class="panel-loading">
          <template v-if="detail">
            <t-alert
              v-if="detail.credential_error"
              theme="error"
              :message="`凭证不可用：${detail.credential_error}`"
              class="credential-alert"
            />
            <t-form ref="formRef" :data="formData" :rules="formRules" label-align="top">
              <div class="form-grid">
                <t-form-item label="提供商名称" name="name">
                  <t-input v-if="editing" v-model="formData.name" maxlength="50" />
                  <span v-else class="readonly-text">{{ detail.name }}</span>
                </t-form-item>
                <t-form-item label="提供商类型">
                  <span class="readonly-text">{{ typeLabel(detail.provider_type) }}</span>
                </t-form-item>
                <t-form-item label="API 地址" name="api_endpoint">
                  <t-input v-if="editing" v-model="formData.api_endpoint" />
                  <span v-else class="readonly-text mono-text">{{ detail.api_endpoint }}</span>
                </t-form-item>
                <t-form-item label="区域">
                  <t-input v-if="editing" v-model="formData.region" placeholder="例如：cn-east-1" />
                  <span v-else class="readonly-text">{{ detail.region || '—' }}</span>
                </t-form-item>
                <!-- 同步节奏不再放基本信息里：它是按同步类型分别配置的（渠道 × 类型），
                     单放一个"同步间隔（秒）"会让人以为一个数字管所有类型。
                     这里只保留只读的同步总开关与入口，配置在「同步与调度」页。 -->
                <t-form-item label="同步总开关">
                  <span class="readonly-text">
                    <t-tag
                      :theme="detail.sync_enabled ? 'success' : 'default'"
                      variant="light"
                      size="small"
                      shape="round"
                    >
                      {{ detail.sync_enabled ? '已开启' : '已关闭' }}
                    </t-tag>
                    <span v-if="!detail.sync_enabled" class="sync-off-hint">关闭期间所有同步类型都不执行</span>
                  </span>
                </t-form-item>
                <t-form-item label="同步节奏">
                  <span class="readonly-text">
                    <t-link theme="primary" hover="color" @click="activeTab = 'schedules'">
                      按同步类型分别配置
                    </t-link>
                    <span class="sync-off-hint">（拉取间隔 / 全量对账 / 执行时段）</span>
                  </span>
                </t-form-item>
                <t-form-item label="状态">
                  <span class="readonly-text">
                    <t-tag
                      :theme="detail.status === 1 ? 'success' : 'default'"
                      variant="light"
                      size="small"
                      shape="round"
                    >
                      {{ statusLabelMap[detail.status] || '未知' }}
                    </t-tag>
                  </span>
                </t-form-item>
                <t-form-item label="运维平台">
                  <t-input
                    v-if="editing"
                    v-model="formData.ops_console_url"
                    placeholder="上游/平台运维控制台地址，留空则不显示跳转入口"
                  />
                  <span v-else class="readonly-text">
                    <t-link v-if="detail.ops_console_url" theme="primary" hover="color" @click="openOpsConsole">
                      {{ detail.ops_console_url }}
                    </t-link>
                    <span v-else>—</span>
                  </span>
                </t-form-item>
                <t-form-item label="同步健康">
                  <span class="readonly-text">
                    <t-tag v-if="detail.sync_paused" theme="warning" variant="light" size="small" shape="round">
                      {{ detail.last_sync_error === '适配器未实现' ? '未接入' : '已暂停' }}
                    </t-tag>
                    <t-tag
                      v-else-if="detail.consecutive_failures > 0"
                      theme="danger"
                      variant="light"
                      size="small"
                      shape="round"
                    >
                      失败 {{ detail.consecutive_failures }} 次
                    </t-tag>
                    <t-tag v-else theme="success" variant="light" size="small" shape="round">正常</t-tag>
                    <span v-if="detail.last_sync_error" class="readonly-text sync-error">{{
                      detail.last_sync_error
                    }}</span>
                  </span>
                </t-form-item>
                <t-form-item label="创建时间">
                  <span class="readonly-text">{{ formatTime(detail.created_at) }}</span>
                </t-form-item>
                <t-form-item label="更新时间">
                  <span class="readonly-text">{{ formatTime(detail.updated_at) }}</span>
                </t-form-item>
              </div>

              <!-- 传输参数（契约③）：超时/重试/限流，0 表示用默认 -->
              <div v-if="editing" class="advanced-box">
                <t-divider align="left">传输设置</t-divider>
                <div class="form-grid">
                  <t-form-item label="超时（秒）" name="timeout_seconds">
                    <t-input-number v-model="formData.timeout_seconds" :min="0" placeholder="0 表示默认" />
                  </t-form-item>
                  <t-form-item label="最大重试次数" name="retry_max">
                    <t-input-number v-model="formData.retry_max" :min="0" placeholder="0 表示默认" />
                  </t-form-item>
                  <t-form-item label="限流 QPS" name="rate_limit_qps">
                    <t-input-number v-model="formData.rate_limit_qps" :min="0" placeholder="0 表示不限" />
                  </t-form-item>
                </div>
              </div>
            </t-form>

            <!-- 凭证：只读态展示脱敏值；编辑态按描述符渲染动态字段 -->
            <div v-if="!editing" class="credential-view">
              <div class="form-section__title">凭证（脱敏）</div>
              <t-descriptions :column="2" bordered size="small">
                <t-descriptions-item v-for="field in credentialFields" :key="field.key" :label="field.label">
                  <span class="mono-text">{{ detail.credentials?.[field.key] || '—' }}</span>
                </t-descriptions-item>
                <t-descriptions-item v-if="!credentialFields.length" label="凭证">
                  <span class="readonly-text">该渠道类型未声明凭证字段</span>
                </t-descriptions-item>
              </t-descriptions>
            </div>
            <CredentialFormFields
              v-else-if="credentialFields.length"
              v-model="formData.credentials"
              :fields="credentialFields"
            />
            <div v-else-if="editing" class="form-grid credential-legacy">
              <t-form-item label="API 密钥">
                <t-input v-model="formData.api_key" type="password" placeholder="留空表示不修改" />
              </t-form-item>
              <t-form-item label="API 密码">
                <t-input v-model="formData.api_secret" type="password" placeholder="留空表示不修改" />
              </t-form-item>
            </div>

            <!-- 数量必须带单位：裸 "1 / 15" 读者无从判断是 15GB 还是 15 核，
                 渠道聚合值来自资源池（GB 口径，见 formatPoolMemory 注释）。 -->
            <t-descriptions v-if="!editing && isCompute" :column="3" bordered size="small" class="resource-summary">
              <t-descriptions-item label="CPU 配额">{{ detail.used_cpu }} / {{ detail.total_cpu }} 核</t-descriptions-item>
              <t-descriptions-item label="内存配额"
                >{{ formatPoolMemory(detail.used_memory) }} /
                {{ formatPoolMemory(detail.total_memory) }}</t-descriptions-item
              >
              <t-descriptions-item label="磁盘配额"
                >{{ formatDisk(detail.used_disk) }} /
                {{ formatDisk(detail.total_disk) }}</t-descriptions-item
              >
            </t-descriptions>
          </template>
          <t-empty v-else-if="!loading" description="未获取到提供商信息" />
        </t-loading>

        <t-alert
          v-if="editing"
          theme="info"
          :message="
            credentialFields.length
              ? '编辑时凭证字段留空或保持脱敏回显表示不修改；修改后将重新加密落库。'
              : '编辑时 API 密钥/密码留空表示保持原值不变。'
          "
          class="edit-alert"
        />
      </template>

      <!-- 能力矩阵 -->
      <template v-else-if="activeTab === 'capability'">
        <CapabilityMatrix v-if="detail" :descriptor="detail.capabilities" :title="`${detail.name} 能力矩阵`" />
        <t-empty v-else description="加载能力矩阵…" />
      </template>

      <!-- 资源池（仅具备 pool 能力的渠道；见 hasPoolCapability 注释） -->
      <template v-else-if="activeTab === 'pools' && hasPoolCapability">
        <PoolPanel :provider-id="providerId" />
      </template>

      <!-- 同步与调度：按同步类型逐条配置，与同步运维控制台同一套口径与接口 -->
      <template v-else-if="activeTab === 'schedules'">
        <ProviderSchedules
          v-if="detail"
          :provider-id="providerId"
          :sync-enabled="detail.sync_enabled"
          :unit="isCompute ? '平台' : '渠道'"
          @update:sync-enabled="handleSyncEnabledChange"
        />
      </template>

      <!-- 同步日志：按当前渠道过滤 /resource/sync/logs，一次同步运行写一行（引擎 writeLog），
           是排障的第一手材料。空态要说清"为什么没有"并给出触发入口 ——
           只写"暂无日志"会让运营以为是坏了，实际上新渠道本来就是跑过才有记录。 -->
      <template v-else-if="activeTab === 'logs'">
        <div class="logs-head">
          <div class="logs-head__meta">
            <span>共 {{ logTotal }} 条同步日志</span>
            <span v-if="logLastAt" class="logs-head__time">最近一次：{{ formatTime(logLastAt) }}</span>
          </div>
          <t-space size="small">
            <t-select
              v-model="logStatus"
              size="small"
              clearable
              placeholder="全部状态"
              :options="logStatusOptions"
              style="width: 130px"
              @change="reloadLogs"
            />
            <t-button size="small" variant="outline" :loading="logLoading" @click="reloadLogs">
              <template #icon>
                <RefreshIcon />
              </template>
              刷新
            </t-button>
            <t-button
              size="small"
              theme="primary"
              :loading="triggering"
              :disabled="!triggerScope || !detail?.sync_enabled"
              @click="handleTriggerSync"
            >
              立即同步一次
            </t-button>
          </t-space>
        </div>

        <!-- 渠道侧前置条件没满足时先说原因：否则"点了没日志"看起来像接口坏了。 -->
        <t-alert v-if="logBlockReason" theme="warning" class="logs-block-alert" :message="logBlockReason" />

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
            <!-- 0 / 0 不是失败：该类型本次没有可处理的条目（如渠道下暂无实例），
                 但任务本身跑通了。不加区分地显示 0/0 会被读成"一条都没成功"。 -->
            <t-tooltip v-if="row.total_count === 0 && row.status === 'success'" content="本次没有需要同步的条目（任务本身已执行完成）" placement="top">
              <span class="cell-num cell-muted">{{ row.success_count }} / {{ row.total_count }}</span>
            </t-tooltip>
            <span v-else class="cell-num">{{ row.success_count }} / {{ row.total_count }}</span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="logStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ logStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #error_message="{ row }">
            <t-tooltip v-if="row.error_message" :content="row.error_message" placement="top-left">
              <span class="cell-error">{{ row.error_message }}</span>
            </t-tooltip>
            <span v-else class="cell-muted">—</span>
          </template>
          <template #created_at="{ row }">
            <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
          </template>
          <template #empty>
            <t-empty :description="logEmptyText" />
          </template>
        </t-table>
      </template>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { CloudIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import {
  MessagePlugin,
  type FormInstanceFunctions,
  type FormRule,
  type PageInfo,
  type PrimaryTableCol,
} from 'tdesign-vue-next'

import {
  createSyncTask,
  getProviderDetail,
  getProviderTypes,
  getSyncLogList,
  getSyncScheduleList,
  resumeProviderSync,
  testConnection,
  updateProvider,
} from '@/api/admin'
import type { ProviderField, ProviderInfo, ProviderTypeItem, SyncLogInfo } from '@/types/interface'
import { formatDisk, formatPoolMemory } from '@/utils/spec-format'
import PoolPanel from '@/pages/resource/pools/index.vue'
import CapabilityMatrix from './components/CapabilityMatrix.vue'
import CredentialFormFields from './components/CredentialFormFields.vue'
import ProviderSchedules from './components/ProviderSchedules.vue'

defineOptions({ name: 'ResourceProvidersDetail' })

const route = useRoute()
const router = useRouter()

const providerId = computed(() => Number(route.params.id))
// 详情页归属哪个渠道列表：按渠道链路回跳（自营平台 / 上游转售），避免跨页跳错。
const listPath = computed(() => (detail.value?.kind === 'compute' ? '/resource/platforms' : '/resource/providers'))
// 自营平台（compute）才有本地容量概念，配额概览同理只在自营链路展示；
// 上游转售渠道的容量权威在上游，本地字段恒为 0。
const isCompute = computed(() => detail.value?.kind === 'compute')
// 是否具备资源池能力：以渠道能力描述符的 sync_scopes 为准（后端唯一的权威判定，
// 与同步引擎、调度生成、PoolReader 类型断言同源）。上游转售渠道（魔方财务等）的
// 描述符只声明 catalog/price/instance，不含 pool —— 这类渠道不展示资源池页签。
const hasPoolCapability = computed(() => (detail.value?.capabilities?.sync_scopes || []).includes('pool'))
const detail = ref<ProviderInfo | null>(null)
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const resuming = ref(false)
const editing = ref(false)
const activeTab = ref('basic')
const typeList = ref<ProviderTypeItem[]>([])
const formRef = ref<FormInstanceFunctions | null>(null)

// —— 同步日志（按当前渠道过滤）——
// 同步范围（scope）的展示名与配色跟同步中心保持一致，避免同一概念两处叫法不同。
// 七个 scope 全部映射（后端 upstream.Scope* 全集）：只覆盖前五个的话，
// image/stock 会以英文原样渲染出来，而"英文键名"正是运营最难判断的形态。
const SCOPE_LABELS: Record<string, string> = {
  catalog: '商品目录',
  price: '价格',
  pool: '资源池',
  region: '区域',
  image: '镜像',
  instance: '实例',
  stock: '库存',
}
const LOG_STATUS_LABELS: Record<string, string> = {
  pending: '待执行',
  running: '执行中',
  success: '成功',
  failed: '失败',
  skipped: '已跳过',
}
const logStatusOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '已跳过', value: 'skipped' },
]

const logList = ref<SyncLogInfo[]>([])
const logLoading = ref(false)
const logStatus = ref<string>('')
const logTotal = ref(0)
const triggering = ref(false)
// 手动触发用当前渠道已配置的第一个同步类型：绝大多数排障场景就是"想立刻看一次结果"，
// 不必强迫用户先去别处选类型。调度行为空时（渠道刚建/总开关关闭）不显示该按钮。
const triggerScope = computed(() => scheduleScopes.value[0] || '')
const scheduleScopes = ref<string[]>([])
const logPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const logColumns: PrimaryTableCol<SyncLogInfo>[] = [
  { colKey: 'sync_type', title: '同步范围', width: 110 },
  { colKey: 'result', title: '成功/总数', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'error_message', title: '错误信息', minWidth: 200, ellipsis: true },
  { colKey: 'task_id', title: '任务 ID', width: 90 },
  { colKey: 'created_at', title: '时间', minWidth: 160 },
]
const logLastAt = computed(() => (logList.value.length ? logList.value[0].created_at : ''))

/**
 * 没有日志时先解释原因，再让人去动作。
 *
 * 实测反馈"同步日志里根本没有显示日志"：新渠道在第一次同步之前本来就是空的，
 * 但空表的"暂无同步日志"读起来像功能坏了。这里按渠道当前状态给出具体原因
 * （总开关关闭 / 适配器未接入 / 熔断暂停 / 单纯还没跑过），并在能触发时提供入口。
 */
const logBlockReason = computed(() => {
  if (!detail.value) return ''
  if (!detail.value.sync_enabled) {
    return '该渠道的同步总开关已关闭，调度器不会触发任何同步，因此不会有新日志。请在「同步与调度」页开启。'
  }
  if (detail.value.sync_paused) {
    const why = detail.value.last_sync_error === '适配器未实现' ? '适配器未接入' : '连续失败已熔断'
    return `该渠道同步已暂停（${why}）：${detail.value.last_sync_error || '请先解除暂停'}。可在页头点「恢复同步」后重试。`
  }
  return ''
})

const logEmptyText = computed(() => {
  if (logBlockReason.value) return '按上方提示处理后，同步日志会在这里出现'
  if (logStatus.value) return '当前状态筛选下没有记录，试试清空「状态」筛选'
  return '该渠道还没有执行过同步：可点右上角「立即同步一次」，或等待调度器按节奏触发'
})

function scopeLabel(scope: string): string {
  return SCOPE_LABELS[scope] || scope || '—'
}

function scopeTheme(scope: string): string {
  if (scope === 'catalog') return 'primary'
  if (scope === 'price') return 'warning'
  if (scope === 'pool') return 'success'
  if (scope === 'region') return 'default'
  if (scope === 'instance') return 'danger'
  return 'default'
}

function logStatusLabel(status: string): string {
  return LOG_STATUS_LABELS[status] || status || '—'
}

function logStatusTheme(status: string): string {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'default'
}

async function loadLogs() {
  if (!providerId.value) return
  logLoading.value = true
  try {
    const data = await getSyncLogList({
      provider_id: providerId.value,
      status: logStatus.value || undefined,
      page: logPagination.current,
      page_size: logPagination.pageSize,
    })
    logList.value = data.items
    logTotal.value = data.meta.total
    logPagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载同步日志失败')
  } finally {
    logLoading.value = false
  }
}

/**
 * 取该渠道实际可同步的类型，用于「立即同步一次」的目标。
 *
 * 走调度接口而不是写死 catalog：自营平台（compute）没有商品目录，
 * 写死 catalog 会触发一个上游不支持的类型，任务直接 skipped。
 * 该接口同时会把缺失的调度行补齐（后端懒创建），所以这一步也顺带修好"新建渠道没有调度行"。
 */
async function loadScheduleScopes() {
  if (!providerId.value) return
  try {
    const data = await getSyncScheduleList({ provider_id: providerId.value, page: 1, page_size: 100 })
    scheduleScopes.value = data.items.filter((item) => item.enabled).map((item) => item.scope)
  } catch {
    /* 触发目标取不到时不阻塞日志展示（按钮会因无目标而禁用） */
  }
}

function reloadLogs() {
  logPagination.current = 1
  loadLogs()
}

function handleLogPageChange(pageInfo: PageInfo) {
  logPagination.current = pageInfo.current
  logPagination.pageSize = pageInfo.pageSize
  loadLogs()
}

// 手动触发一次同步：任务异步执行，稍等一拍再拉日志，否则查到的还是 pending 没有记录。
async function handleTriggerSync() {
  if (!triggerScope.value) return
  triggering.value = true
  try {
    await createSyncTask({ provider_id: providerId.value, task_type: triggerScope.value })
    MessagePlugin.success(`已触发「${scopeLabel(triggerScope.value)}」同步，正在执行…`)
    setTimeout(loadLogs, 2500)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '触发同步失败')
  } finally {
    triggering.value = false
  }
}

// 凭证字段由渠道能力描述符驱动（落库的脱敏值据此回显）。
const credentialFields = computed<ProviderField[]>(() => detail.value?.capabilities?.credential_schema || [])

const statusLabelMap: Record<number, string> = {
  1: '启用',
  0: '禁用',
}

const formData = reactive({
  name: '',
  api_endpoint: '',
  region: '',
  api_key: '',
  api_secret: '',
  credentials: {} as Record<string, string>,
  // sync_enabled 不在这里：它是同步总开关，由「同步与调度」面板经调度接口/渠道接口单独提交，
  // 本表单只管渠道接入资料，避免同一个字段两个入口互相覆盖。
  status: 1,
  timeout_seconds: 0,
  retry_max: 0,
  rate_limit_qps: 0,
  ops_console_url: '',
})

const formRules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入提供商名称', type: 'error', trigger: 'blur' }],
  api_endpoint: [
    { required: true, message: '请输入 API 地址', type: 'error', trigger: 'blur' },
    { pattern: /^https?:\/\//, message: 'API 地址需以 http(s):// 开头', type: 'error', trigger: 'blur' },
  ],
}

function typeLabel(type: string): string {
  return typeList.value.find((item) => item.type === type)?.name || type
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

async function loadDetail() {
  loading.value = true
  try {
    detail.value = await getProviderDetail(providerId.value)
    // 能力到手后回落页签：详情是异步加载的，若首屏 activeTab 停在当前渠道不支持的
    // 页签上（如上游渠道的 pools），页签栏里根本没有它 —— 内容区会空白且无法察觉。
    if (activeTab.value === 'pools' && !hasPoolCapability.value) activeTab.value = 'basic'
    Object.assign(formData, {
      name: detail.value.name,
      api_endpoint: detail.value.api_endpoint,
      region: detail.value.region,
      api_key: '',
      api_secret: '',
      credentials: { ...(detail.value.credentials || {}) },
      status: detail.value.status,
      timeout_seconds: detail.value.timeout_seconds,
      retry_max: detail.value.retry_max,
      rate_limit_qps: detail.value.rate_limit_qps,
      ops_console_url: detail.value.ops_console_url || '',
    })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商详情失败')
    router.replace('/resource/providers')
  } finally {
    loading.value = false
  }
}

async function loadTypes() {
  try {
    typeList.value = await getProviderTypes()
  } catch {
    /* 类型名映射失败不阻塞详情展示 */
  }
}

function toggleEdit() {
  if (!editing.value) {
    editing.value = true
    return
  }
  handleSave()
}

// 运维平台为外部系统，新窗口打开（不接管站内路由）。
function openOpsConsole() {
  const url = detail.value?.ops_console_url
  if (url) window.open(url, '_blank', 'noopener')
}

async function handleSave() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  saving.value = true
  try {
    const payload = {
      name: formData.name,
      api_endpoint: formData.api_endpoint,
      region: formData.region,
      status: formData.status,
      timeout_seconds: formData.timeout_seconds,
      retry_max: formData.retry_max,
      rate_limit_qps: formData.rate_limit_qps,
      ops_console_url: formData.ops_console_url || '',
    } as Parameters<typeof updateProvider>[1]
    if (credentialFields.value.length) {
      payload.credentials = formData.credentials
    } else {
      payload.api_key = formData.api_key || undefined
      payload.api_secret = formData.api_secret || undefined
    }
    const updated = await updateProvider(providerId.value, payload)
    detail.value = updated
    formData.credentials = { ...(updated.credentials || {}) }
    formData.api_key = ''
    formData.api_secret = ''
    editing.value = false
    MessagePlugin.success('提供商资料已保存')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存提供商资料失败')
  } finally {
    saving.value = false
  }
}

async function handleTestConnection() {
  testing.value = true
  try {
    const result = await testConnection(providerId.value)
    if (result.success) {
      MessagePlugin.success(`连接测试通过：${result.message}`)
    } else {
      MessagePlugin.error(`连接测试失败：${result.message}`)
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '连接测试失败')
  } finally {
    testing.value = false
  }
}

async function handleResumeSync() {
  resuming.value = true
  try {
    await resumeProviderSync(providerId.value)
    MessagePlugin.success('已恢复同步，下次调度将重新尝试')
    await loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复同步失败')
  } finally {
    resuming.value = false
  }
}

// 同步总开关由子面板提交（只带 sync_enabled 一个字段），这里把详情快照同步过来，
// 免得「同步健康」「同步总开关」两处标签各显示一份互相矛盾的旧值。
function handleSyncEnabledChange(value: boolean) {
  if (detail.value) detail.value.sync_enabled = value
}

onMounted(() => {
  loadTypes()
  loadDetail()
})

// 切到「同步日志」时才拉取：日志表按 provider 过滤，详情页首屏没必要预载。
// 从日志页切走再回来会重新拉一次，保证看到的是最新一条同步结果。
watch(activeTab, (tab) => {
  if (tab === 'logs') {
    loadLogs()
    void loadScheduleScopes()
  }
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.tabs-card,
.content-card {
  padding: var(--space-lg) 24px;
}

.panel-loading {
  min-height: 160px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.sync-error {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  word-break: break-all;
  color: var(--color-muted-foreground);
}

/* 只读态跟在标签后面的补充说明（如"关闭期间所有同步类型都不执行"）。 */
.sync-off-hint {
  margin-left: 8px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.readonly-text {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  color: var(--color-foreground);
  line-height: 1.6;
}

.mono-text {
  font-family: var(--hs-font-mono);
  font-size: 13px;
  word-break: break-all;
}

.resource-summary {
  margin-top: var(--space-lg);
}

.edit-alert {
  margin-top: var(--space-lg);
}

.credential-alert {
  margin-bottom: var(--space-lg);
}

.credential-view {
  margin-top: var(--space-lg);
}

.credential-view .form-section__title,
.form-section__title {
  margin: 0 0 var(--space-sm);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.credential-legacy {
  margin-top: var(--space-md);
}

.advanced-box {
  margin-top: var(--space-md);
  border: 1px dashed var(--color-border);
  border-radius: var(--hs-radius-lg);
  padding: var(--space-md) var(--space-lg) 0;
}

.logs-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
}

.logs-head__meta {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  font-size: 13px;
  color: var(--color-muted-foreground);
}

/* 「最近一次」时间戳比条数更次要，字号再收一档；等宽数字避免刷新时左右跳动。 */
.logs-head__time {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

/* 渠道侧前置条件未满足时的说明条（总开关关闭/熔断），压在日志表上方。 */
.logs-block-alert {
  margin-bottom: var(--space-md);
}

/* 错误信息单行截断，完整内容走 tooltip（同步失败原因常含上游返回的长 JSON）。 */

@media (max-width: 1200px) {
  .form-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
