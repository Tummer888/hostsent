<template>
  <div class="risk-page">
    <!-- 汇总卡片：运营打开页面先要知道「有没有需要处理的」。
         与列表共用同一套筛选条件，卡片数与筛出来的列表永远对得上。 -->
    <div class="risk-summary">
      <button
        v-for="card in summaryCards"
        :key="card.status || 'all'"
        type="button"
        class="risk-summary__card"
        :class="{ 'is-active': filters.status === card.status }"
        @click="toggleStatusFilter(card.status)"
      >
        <span class="risk-summary__value" :class="`is-${card.tone}`">{{ card.count }}</span>
        <span class="risk-summary__label">{{ card.label }}</span>
      </button>
    </div>

    <SecurityListPage
      title="异常行为监控"
      table-title="风险事件"
      :total="pagination.total"
      :data="tableData"
      :columns="columns"
      :loading="loading"
      :error-message="errorMessage"
      empty-text="暂无风险事件"
      :pagination="pagination"
      @search="handleSearch"
      @reset="handleReset"
      @reload="loadData"
      @page-change="handlePageChange"
      :icon="ChartBarIcon"
    >
      <template #filters>
        <div class="field">
          <span class="field__label">风险类型</span>
          <t-select v-model="filters.risk_type" clearable :options="RISK_TYPE_OPTIONS" placeholder="风险类型" />
        </div>
        <div class="field">
          <span class="field__label">风险等级</span>
          <t-select v-model="filters.risk_level" clearable :options="RISK_LEVEL_OPTIONS" placeholder="风险等级" />
        </div>
        <div class="field">
          <span class="field__label">处置状态</span>
          <t-select v-model="filters.status" clearable :options="RISK_STATUS_OPTIONS" placeholder="处置状态" />
        </div>
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" clearable placeholder="关键词 / 用户名 / IP" />
        </div>
        <div class="field field--wide">
          <span class="field__label">发生时间</span>
          <t-date-range-picker v-model="dateRange" clearable allow-input @change="handleDateChange" />
        </div>
      </template>

      <template #risk_type="{ row }">
        {{ RISK_TYPE_LABEL[row.risk_type] || row.risk_type || '—' }}
      </template>

      <template #risk_level="{ row }">
        <t-tag :theme="securityRiskTagTheme[row.risk_level] || 'default'" variant="light-outline">
          {{ RISK_LEVEL_LABEL[row.risk_level] || row.risk_level || '—' }}
        </t-tag>
      </template>

      <template #username="{ row }">
        <span>{{ row.username || '—' }}</span>
        <!-- 主体域徽标：user_id 承载两个 ID 空间且会撞号，不标出来无法区分
             同 ID 的客户与员工。 -->
        <t-tag
          v-if="row.subject_type === 'admin'"
          theme="primary"
          variant="light"
          size="small"
          class="subject-badge"
        >
          员工
        </t-tag>
      </template>

      <template #rule_code="{ row }">
        <span :title="row.rule_code">{{ RISK_RULE_LABEL[row.rule_code] || row.rule_code || '—' }}</span>
      </template>

      <template #status="{ row }">
        <t-tag :theme="securityStatusTagTheme[row.status] || 'default'" variant="light-outline">
          {{ RISK_STATUS_LABEL[row.status] || row.status || '—' }}
        </t-tag>
      </template>

      <template #occur_count="{ row }">
        {{ formatSecurityCount(row.occur_count) }}
      </template>

      <template #handled_by="{ row }">
        {{ row.handled_by_name || (row.handled_by ? `#${row.handled_by}` : '—') }}
      </template>

      <template #first_occurred_at="{ row }">
        {{ formatSecurityTime(row.first_occurred_at) }}
      </template>

      <template #last_occurred_at="{ row }">
        {{ formatSecurityTime(row.last_occurred_at) }}
      </template>

      <template #operation="{ row }">
        <MobileAction
          v-if="isMobile"
          :options="buildMobileActionOptions([
            { content: '详情', value: 'detail' },
            { content: '调整等级', value: 'level' },
            { content: '忽略', value: 'ignore' },
            { content: '处置', value: 'resolve' },
            { content: '拉黑', value: 'blacklist', theme: 'error' },
            { content: '失效会话', value: 'revoke' },
          ])"
          @select="(value) => handleMobileAction(value, row)"
        />
        <t-space v-else size="small">
          <t-link theme="primary" @click="openDetail(row)">详情</t-link>
          <t-link v-permission="'security:risk:list'" theme="primary" @click="openLevel(row)">调整等级</t-link>
          <t-link v-permission="'security:risk:list'" theme="primary" @click="handleIgnore(row)">忽略</t-link>
          <t-link v-permission="'security:risk:list'" theme="primary" @click="handleResolve(row)">处置</t-link>
          <t-link v-permission="'security:blacklist:manage'" theme="danger" @click="handleBlacklist(row)">拉黑</t-link>
          <t-link v-permission="'security:session:manage'" theme="primary" @click="handleRevoke(row)">失效会话</t-link>
        </t-space>
      </template>
    </SecurityListPage>
  </div>

  <!-- 事件详情：命中上下文（规则/主体域/设备指纹/明细 payload）都在这里。
       列表列宽有限放不下，而这些正是运营判断「要不要处置」的依据。 -->
  <t-drawer
    v-model:visible="detailVisible"
    header="风险事件详情"
    size="min(720px, 92vw)"
    :footer="false"
    destroy-on-close
  >
    <t-descriptions v-if="detailRow" :column="1" bordered size="small">
      <t-descriptions-item label="事件 ID">#{{ detailRow.id }}</t-descriptions-item>
      <t-descriptions-item label="风险类型">
        {{ RISK_TYPE_LABEL[detailRow.risk_type] || detailRow.risk_type }}
      </t-descriptions-item>
      <t-descriptions-item label="风险等级">
        <t-tag :theme="securityRiskTagTheme[detailRow.risk_level] || 'default'" variant="light-outline">
          {{ RISK_LEVEL_LABEL[detailRow.risk_level] || detailRow.risk_level }}
        </t-tag>
      </t-descriptions-item>
      <t-descriptions-item label="关联账号">
        {{ detailRow.username || '—' }}
        <t-tag v-if="detailRow.subject_type === 'admin'" theme="primary" variant="light" size="small">员工后台</t-tag>
        <span v-else class="detail-muted">（客户 ID #{{ detailRow.user_id }}）</span>
      </t-descriptions-item>
      <t-descriptions-item label="命中规则">
        {{ RISK_RULE_LABEL[detailRow.rule_code] || detailRow.rule_code }}
        <span class="detail-muted">{{ detailRow.rule_code }}</span>
      </t-descriptions-item>
      <t-descriptions-item label="关联 IP">{{ detailRow.ip || '—' }}</t-descriptions-item>
      <t-descriptions-item label="设备指纹">{{ detailRow.device_fingerprint || '—' }}</t-descriptions-item>
      <t-descriptions-item label="发生次数">{{ detailRow.occur_count }}</t-descriptions-item>
      <t-descriptions-item label="首次发生">{{ formatSecurityTime(detailRow.first_occurred_at) }}</t-descriptions-item>
      <t-descriptions-item label="最近发生">{{ formatSecurityTime(detailRow.last_occurred_at) }}</t-descriptions-item>
      <t-descriptions-item label="处置状态">
        <t-tag :theme="securityStatusTagTheme[detailRow.status] || 'default'" variant="light-outline">
          {{ RISK_STATUS_LABEL[detailRow.status] || detailRow.status }}
        </t-tag>
      </t-descriptions-item>
      <t-descriptions-item label="处置人">
        {{ detailRow.handled_by_name || (detailRow.handled_by ? `#${detailRow.handled_by}` : '—') }}
        <span v-if="detailRow.handled_at" class="detail-muted">{{ formatSecurityTime(detailRow.handled_at) }}</span>
      </t-descriptions-item>
      <t-descriptions-item label="处置说明">{{ detailRow.handle_note || '—' }}</t-descriptions-item>
      <t-descriptions-item label="事件摘要">{{ detailRow.summary || '—' }}</t-descriptions-item>
    </t-descriptions>

    <!-- 命中明细：后端存的是 JSON 字符串，原样展示会是一整行难读的文本。 -->
    <section v-if="detailRow" class="detail-payload">
      <h4 class="detail-payload__title">命中明细</h4>
      <pre class="detail-payload__body">{{ prettyPayload }}</pre>
    </section>

    <template v-if="detailRow">
      <t-space class="detail-actions">
        <t-button theme="primary" variant="outline" @click="openLevel(detailRow)">调整等级</t-button>
        <t-button theme="danger" variant="outline" @click="handleBlacklistFromDetail">加入黑名单</t-button>
        <t-button theme="danger" variant="outline" @click="handleRevokeFromDetail">失效会话</t-button>
      </t-space>
      <p class="detail-hint">
        处置动作都会记录操作人与时间；「加入黑名单」按事件里最高优先的维度（IP → 设备 → 账号）拉黑，
        生效后该来源的登录会被直接拒绝。
      </p>
    </template>
  </t-drawer>

  <!-- 手动调整风险等级：规则只能按固定阈值定级，真实场景需要人工改级并留痕。 -->
  <t-dialog
    v-model:visible="levelDialog.visible"
    header="调整风险等级"
    width="460px"
    :confirm-btn="{ content: '保存', loading: levelDialog.saving }"
    @confirm="submitLevel"
  >
    <t-form label-align="top">
      <t-form-item label="事件">
        <t-input :value="levelDialog.summary" readonly disabled />
      </t-form-item>
      <t-form-item label="风险等级">
        <t-radio-group v-model="levelDialog.level" variant="default-filled">
          <t-radio-button v-for="item in RISK_LEVEL_OPTIONS" :key="item.value" :value="item.value">
            {{ item.label }}
          </t-radio-button>
        </t-radio-group>
      </t-form-item>
      <t-form-item label="调整原因">
        <t-textarea v-model="levelDialog.note" :maxlength="200" placeholder="如：促销日大量用户换设备登录，降级为低" />
      </t-form-item>
      <p class="level-hint">
        只改等级、不改处置状态 —— 提级后事件仍留在「待处理」里，不会被自动当成已处理。
      </p>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { ChartBarIcon } from 'tdesign-icons-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'

import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  blacklistRiskEvent,
  getRiskEventList,
  getRiskEventStats,
  handleRiskEvent,
  ignoreRiskEvent,
  revokeRiskEventSessions,
  updateRiskEventLevel,
  type RiskEventInfo,
  type RiskEventListQuery,
} from '@/api/security'

import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import SecurityListPage from '../SecurityListPage.vue'
import {
  RISK_LEVEL_LABEL,
  RISK_LEVEL_OPTIONS,
  RISK_RULE_LABEL,
  RISK_STATUS_LABEL,
  RISK_STATUS_OPTIONS,
  RISK_TYPE_LABEL,
  RISK_TYPE_OPTIONS,
  applyDateRange,
  formatSecurityCount,
  formatSecurityTime,
  securityRiskTagTheme,
  securityStatusTagTheme,
} from '../shared'

defineOptions({ name: 'UserSecurityRisk' })

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<RiskEventInfo[]>([])
const dateRange = ref<string[]>([])

const filters = reactive<RiskEventListQuery>({
  page: 1,
  page_size: 10,
  risk_type: '',
  risk_level: '',
  status: '',
  keyword: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

// 汇总卡片：点一下即按该状态筛选（再点一次取消）。比让运营去下拉里选更直接。
const stats = ref<Record<string, number>>({})
const summaryCards = computed(() => [
  { status: 'pending', label: '待处理', count: stats.value.pending ?? 0, tone: 'pending' },
  { status: 'handled', label: '已处置', count: stats.value.handled ?? 0, tone: 'handled' },
  { status: 'ignored', label: '已忽略', count: stats.value.ignored ?? 0, tone: 'ignored' },
  { status: '', label: '全部', count: stats.value.total ?? 0, tone: 'total' },
])

const { isMobile } = useIsMobile()

const columns = computed<PrimaryTableCol<RiskEventInfo>[]>(() => [
  { colKey: 'risk_type', title: '风险类型', width: 120 },
  { colKey: 'risk_level', title: '等级', width: 90 },
  { colKey: 'username', title: '关联账号', width: 140 },
  { colKey: 'ip', title: 'IP 地址', width: 130 },
  { colKey: 'rule_code', title: '命中规则', width: 170 },
  { colKey: 'summary', title: '摘要', minWidth: 200, ellipsis: true },
  { colKey: 'occur_count', title: '发生次数', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'handled_by', title: '处置人', width: 110 },
  { colKey: 'last_occurred_at', title: '最近发生', width: 170 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 300, fixed: 'right' },
])

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getRiskEventList({
      ...filters,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    tableData.value = response.items
    pagination.total = response.meta.total
    await loadStats()
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载风险事件失败'
  } finally {
    loading.value = false
  }
}

// loadStats 用同一套筛选（去掉分页与状态）取各状态计数。
//
// 状态必须剥掉：汇总问的是「各状态各多少条」，带着某个状态去问是无意义的组合。
async function loadStats() {
  try {
    stats.value = await getRiskEventStats({
      risk_type: filters.risk_type,
      risk_level: filters.risk_level,
      keyword: filters.keyword,
      start_time: filters.start_time,
      end_time: filters.end_time,
    })
  } catch {
    // 汇总失败不影响列表：卡片是辅助信息，不该让整页报错
    stats.value = {}
  }
}

function toggleStatusFilter(status: string) {
  filters.status = filters.status === status ? '' : status
  pagination.current = 1
  void loadData()
}

function handleDateChange(value: unknown) {
  applyDateRange(filters, value)
}

function handleSearch() {
  pagination.current = 1
  void loadData()
}

function handleReset() {
  filters.risk_type = ''
  filters.risk_level = ''
  filters.status = ''
  filters.keyword = ''
  filters.start_time = undefined
  filters.end_time = undefined
  dateRange.value = []
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  void loadData()
}

// —— 详情抽屉 ——
const detailVisible = ref(false)
const detailRow = ref<RiskEventInfo | null>(null)

// prettyPayload 把后端存的 JSON 明细格式化后再展示。
//
// 解析失败就原样返回：明细是排查线索，宁可能看到原始文本，
// 也不该因为一次解析异常让整个抽屉内容消失。
const prettyPayload = computed(() => {
  const raw = detailRow.value?.detail_payload
  if (!raw) return '（无）'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
})

function openDetail(row: RiskEventInfo) {
  detailRow.value = row
  detailVisible.value = true
}

// —— 等级调整 ——
const levelDialog = reactive({
  visible: false,
  saving: false,
  id: 0,
  summary: '',
  level: 'low',
  note: '',
})

function openLevel(row: RiskEventInfo) {
  levelDialog.id = row.id
  levelDialog.summary = row.summary || `#${row.id}`
  levelDialog.level = row.risk_level || 'low'
  levelDialog.note = ''
  levelDialog.visible = true
}

async function submitLevel() {
  levelDialog.saving = true
  try {
    const updated = await updateRiskEventLevel(levelDialog.id, {
      risk_level: levelDialog.level,
      note: levelDialog.note,
    })
    MessagePlugin.success(`风险等级已调整为「${RISK_LEVEL_LABEL[updated.risk_level] || updated.risk_level}」`)
    levelDialog.visible = false
    if (detailRow.value && detailRow.value.id === updated.id) {
      detailRow.value = updated
    }
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '调整等级失败')
  } finally {
    levelDialog.saving = false
  }
}

// —— 处置动作 ——
async function handleIgnore(row: RiskEventInfo) {
  await ignoreRiskEvent(row.id)
  await loadData()
}

async function handleResolve(row: RiskEventInfo) {
  await handleRiskEvent(row.id)
  await loadData()
}

async function handleBlacklist(row: RiskEventInfo) {
  await blacklistRiskEvent(row.id)
  await loadData()
}

async function handleRevoke(row: RiskEventInfo) {
  await revokeRiskEventSessions(row.id)
  await loadData()
}

// 抽屉里的动作：执行完刷新列表，并收起抽屉（那条事件的状态可能已变）。
async function handleBlacklistFromDetail() {
  const row = detailRow.value
  if (!row) return
  await handleBlacklist(row)
  detailVisible.value = false
}

async function handleRevokeFromDetail() {
  const row = detailRow.value
  if (!row) return
  await handleRevoke(row)
  detailVisible.value = false
}

onMounted(() => {
  void loadData()
})

function handleMobileAction(value: string | number | Record<string, any>, row: RiskEventInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'level':
      openLevel(row)
      break
    case 'ignore':
      handleIgnore(row)
      break
    case 'resolve':
      handleResolve(row)
      break
    case 'blacklist':
      handleBlacklist(row)
      break
    case 'revoke':
      handleRevoke(row)
      break
  }
}
</script>

<style scoped lang="css">
.risk-page {
  display: flex;
  flex-direction: column;
}

.risk-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.risk-summary__card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px 16px;
  border: 1px solid var(--td-brand-color-2);
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.risk-summary__card:hover {
  border-color: var(--color-primary);
}

/* 选中态：与筛选下拉的 status 双向同步（点卡片即筛选，再点取消）。 */
.risk-summary__card.is-active {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(var(--color-primary-rgb), 0.1);
}

.risk-summary__value {
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
  color: var(--color-foreground);
}

.risk-summary__value.is-pending {
  color: var(--td-warning-color-6, #e37318);
}

.risk-summary__value.is-handled {
  color: var(--td-success-color-6, #2ba471);
}

.risk-summary__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.subject-badge {
  margin-left: 6px;
}

.detail-muted {
  margin-left: 6px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.detail-payload {
  margin-top: 16px;
}

.detail-payload__title {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.detail-payload__body {
  margin: 0;
  padding: 12px;
  max-height: 260px;
  overflow: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}

.detail-actions {
  margin-top: 16px;
}

.detail-hint,
.level-hint {
  margin: 10px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

@media (max-width: 768px) {
  .risk-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
