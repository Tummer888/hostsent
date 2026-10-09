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
          <span class="field__label">已做动作</span>
          <t-select v-model="filters.action" clearable :options="RISK_ACTION_OPTIONS" placeholder="不限" />
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

      <!-- 处置状态：只回答「这条待办关掉没有」。 -->
      <template #status="{ row }">
        <t-tag :theme="securityStatusTagTheme[row.status] || 'default'" variant="light-outline">
          {{ RISK_STATUS_LABEL[row.status] || row.status || '—' }}
        </t-tag>
      </template>

      <!-- 已做动作：回答「对这条做过哪些管控」。
           与状态分开显示是关键 —— 拉黑、失效会话不改变待办状态，
           改造前它们没有任何回显，运营点完看到状态没变会以为没生效。 -->
      <template #actions="{ row }">
        <t-space v-if="row.actions && row.actions.length" size="4" break-line>
          <t-tag
            v-for="code in row.actions"
            :key="code"
            :theme="RISK_ACTION_THEME[code] || 'default'"
            variant="light-outline"
            size="small"
          >
            {{ RISK_ACTION_LABEL[code] || code }}
          </t-tag>
        </t-space>
        <span v-else class="cell-muted">未做管控</span>
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
          :options="buildMobileActionOptions(mobileActions(row))"
          @select="(value) => handleMobileAction(value, row)"
        />
        <t-space v-else size="small" break-line>
          <t-link theme="primary" @click="openDetail(row)">详情</t-link>
          <t-link v-permission="'security:risk:list'" theme="primary" @click="openLevel(row)">调整等级</t-link>
          <t-link
            v-permission="'security:risk:list'"
            theme="primary"
            :disabled="row.status === 'ignored'"
            @click="openClose(row, 'ignored')"
          >
            忽略
          </t-link>
          <t-link
            v-permission="'security:risk:list'"
            theme="primary"
            :disabled="row.status === 'handled'"
            @click="openClose(row, 'handled')"
          >
            处置
          </t-link>
          <t-link v-permission="'security:blacklist:manage'" theme="danger" @click="openBlacklist(row)">拉黑</t-link>
          <t-link v-permission="'security:session:manage'" theme="primary" @click="openRevoke(row)">失效会话</t-link>
        </t-space>
      </template>
    </SecurityListPage>
  </div>

  <!-- 事件详情：命中上下文（规则/主体域/设备指纹/明细 payload）+ 处置时间线。
       列表列宽有限放不下，而这些正是运营判断「下一步该做什么」的依据。 -->
  <t-drawer
    v-model:visible="detailVisible"
    header="风险事件详情"
    size="min(760px, 94vw)"
    :footer="false"
    destroy-on-close
  >
    <template v-if="detailRow">
      <t-descriptions :column="1" bordered size="small">
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
          <span class="detail-muted">只表示待办是否关闭，不代表做过管控</span>
        </t-descriptions-item>
        <t-descriptions-item label="已做管控">
          <t-space v-if="detailRow.actions && detailRow.actions.length" size="4" break-line>
            <t-tag
              v-for="code in detailRow.actions"
              :key="code"
              :theme="RISK_ACTION_THEME[code] || 'default'"
              variant="light-outline"
              size="small"
            >
              {{ RISK_ACTION_LABEL[code] || code }}
            </t-tag>
          </t-space>
          <span v-else class="detail-muted">未做任何管控</span>
        </t-descriptions-item>
        <t-descriptions-item label="处置说明">{{ detailRow.handle_note || '—' }}</t-descriptions-item>
        <t-descriptions-item label="事件摘要">{{ detailRow.summary || '—' }}</t-descriptions-item>
      </t-descriptions>

      <!-- 命中明细：后端存的是 JSON 字符串，原样展示会是一整行难读的文本。 -->
      <section class="detail-payload">
        <h4 class="detail-payload__title">命中明细</h4>
        <pre class="detail-payload__body">{{ prettyPayload }}</pre>
      </section>

      <!-- 处置时间线：每次动作一行，回答「谁在什么时候做了什么」。
           事件的 handle_note / handled_by 只有一份，会被后来的操作覆盖，
           这里才是完整留痕。 -->
      <section class="detail-timeline">
        <h4 class="detail-payload__title">处置记录</h4>
        <t-loading :loading="timelineLoading" size="small">
          <p v-if="!timeline.length" class="detail-hint">这条事件还没有任何处置记录。</p>
          <ul v-else class="timeline">
            <li v-for="item in timeline" :key="item.id" class="timeline__item">
              <t-tag :theme="RISK_ACTION_THEME[item.action] || 'default'" variant="light" size="small">
                {{ RISK_ACTION_LABEL[item.action] || item.action }}
              </t-tag>
              <div class="timeline__body">
                <div class="timeline__meta">
                  {{ item.operator_name || (item.operator_id ? `#${item.operator_id}` : '系统') }}
                  · {{ formatSecurityTime(item.created_at) }}
                </div>
                <div v-if="item.note" class="timeline__note">{{ item.note }}</div>
                <div v-if="prettyActionDetail(item.detail)" class="timeline__detail">
                  {{ prettyActionDetail(item.detail) }}
                </div>
              </div>
            </li>
          </ul>
        </t-loading>
      </section>

      <t-space class="detail-actions" break-line>
        <t-button theme="primary" variant="outline" @click="openLevel(detailRow)">调整等级</t-button>
        <t-button
          theme="primary"
          variant="outline"
          :disabled="detailRow.status === 'handled'"
          @click="openClose(detailRow, 'handled')"
        >
          标记已处置
        </t-button>
        <t-button theme="danger" variant="outline" @click="openBlacklist(detailRow)">加入黑名单</t-button>
        <t-button theme="warning" variant="outline" @click="openRevoke(detailRow)">失效会话</t-button>
      </t-space>
      <p class="detail-hint">
        「调整等级 / 加入黑名单 / 失效会话」都是**独立动作**，不会自动关闭待办 ——
        它们做完后列表的「已做动作」列会亮起来，但「处置状态」仍是待处理，
        表示这条还要继续观察。要结案请点「处置」或「忽略」。
      </p>
    </template>
  </t-drawer>

  <!-- 处置 / 忽略：关闭待办，可选顺带做管控。 -->
  <t-dialog
    v-model:visible="closeDialog.visible"
    :header="closeDialog.status === 'ignored' ? '忽略风险事件' : '处置风险事件'"
    width="520px"
    :confirm-btn="{ content: closeDialog.status === 'ignored' ? '确认忽略' : '确认处置', loading: closeDialog.saving }"
    @confirm="submitClose"
  >
    <t-form label-align="top">
      <t-form-item label="事件">
        <t-input :value="closeDialog.summary" readonly disabled />
      </t-form-item>
      <t-form-item label="处置说明">
        <t-textarea
          v-model="closeDialog.note"
          :maxlength="200"
          :placeholder="closeDialog.status === 'ignored' ? '如：用户自助换机，属正常行为' : '如：确认撞库，已封禁来源'"
        />
      </t-form-item>
      <t-form-item label="同时执行管控（可选）">
        <t-space direction="vertical" size="4">
          <t-checkbox v-model="closeDialog.blacklist">加入黑名单</t-checkbox>
          <t-checkbox v-model="closeDialog.revoke_sessions" :disabled="!canRevoke">强制下线该账号</t-checkbox>
        </t-space>
        <p v-if="!canRevoke" class="level-hint">
          员工后台账号暂不支持强制下线（会话表只承载客户会话），可改用拉黑或到员工管理里禁用。
        </p>
      </t-form-item>
      <p class="level-hint">
        「忽略」与「处置」都只表示<strong>待办已关闭</strong>；勾选的管控动作会真正生效
        （拉黑后该来源下次登录被拒）。若管控失败，事件不会被关闭，以免留下「已处置但其实没封上」的假象。
      </p>
    </t-form>
  </t-dialog>

  <!-- 拉黑：默认维度由后端按事件推断，运营可显式改选。 -->
  <t-dialog
    v-model:visible="blacklistDialog.visible"
    header="加入黑名单"
    width="520px"
    :confirm-btn="{ content: '确认拉黑', theme: 'danger', loading: blacklistDialog.saving }"
    @confirm="submitBlacklist"
  >
    <t-form label-align="top">
      <t-form-item label="事件">
        <t-input :value="blacklistDialog.summary" readonly disabled />
      </t-form-item>
      <t-form-item label="拉黑维度">
        <t-select v-model="blacklistDialog.type" :options="blacklistTypeOptions" placeholder="按事件可用字段自动选择" />
      </t-form-item>
      <t-form-item label="原因">
        <t-textarea v-model="blacklistDialog.note" :maxlength="200" placeholder="如：确认撞库来源，永久封禁" />
      </t-form-item>
      <t-form-item label="同时关闭待办">
        <t-switch v-model="blacklistDialog.close_event" />
      </t-form-item>
      <p class="level-hint">
        拉黑本身不会关闭待办：确认没问题再关，还没查清就先留着继续观察。
        同一个维度+命中值重复添加会被拒绝，提示你直接编辑已有记录。
      </p>
    </t-form>
  </t-dialog>

  <!-- 失效会话：按事件主体域精确匹配，员工域直接拒绝。 -->
  <t-dialog
    v-model:visible="revokeDialog.visible"
    header="失效会话"
    width="520px"
    :confirm-btn="{ content: '确认失效', theme: 'warning', loading: revokeDialog.saving }"
    @confirm="submitRevoke"
  >
    <t-form label-align="top">
      <t-form-item label="事件">
        <t-input :value="revokeDialog.summary" readonly disabled />
      </t-form-item>
      <t-form-item label="目标">
        <t-input :value="revokeTarget" readonly disabled />
      </t-form-item>
      <t-form-item label="原因">
        <t-textarea v-model="revokeDialog.note" :maxlength="200" placeholder="如：账号疑似被盗，先踢下线" />
      </t-form-item>
      <t-form-item label="同时关闭待办">
        <t-switch v-model="revokeDialog.close_event" />
      </t-form-item>
      <p class="level-hint">
        会撤销该账号<strong>当前所有有效会话</strong>，立即生效（被踢的令牌马上不可用）。
        只影响与事件同一主体域的会话，不会误伤同 ID 的其它域账号。
      </p>
    </t-form>
  </t-dialog>

  <!-- 手动调整风险等级：规则只能按固定阈值定级，真实场景需要人工改级并留痕。 -->
  <t-dialog
    v-model:visible="levelDialog.visible"
    header="调整风险等级"
    width="480px"
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
      <t-form-item label="同时关闭待办">
        <t-space direction="vertical" size="4">
          <t-switch v-model="levelDialog.closeEvent" />
          <t-radio-group v-if="levelDialog.closeEvent" v-model="levelDialog.closeAs" variant="default-filled" size="small">
            <t-radio-button value="handled">标记已处置</t-radio-button>
            <t-radio-button value="ignored">标记已忽略</t-radio-button>
          </t-radio-group>
        </t-space>
      </t-form-item>
      <p class="level-hint">
        默认只改等级、不改处置状态 —— 提级意味着「这条要重点看」，顺手关掉会让你丢掉待办。
        确认是误报时，再打开「同时关闭待办」一次结案。
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
  getRiskEventActions,
  getRiskEventList,
  getRiskEventStats,
  handleRiskEvent,
  ignoreRiskEvent,
  revokeRiskEventSessions,
  updateRiskEventLevel,
  type RiskEventActionInfo,
  type RiskEventInfo,
  type RiskEventListQuery,
} from '@/api/security'

import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions, type MobileActionItem } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import SecurityListPage from '../SecurityListPage.vue'
import {
  BLACKLIST_TYPE_OPTIONS,
  RISK_ACTION_LABEL,
  RISK_ACTION_OPTIONS,
  RISK_ACTION_THEME,
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

// filters 直接用 API 的查询类型：action 已在 RiskEventListQuery 里声明，
// 另建一个同名扩展接口只会在后端加字段时多一处要同步的地方。
const filters = reactive<RiskEventListQuery>({
  page: 1,
  page_size: 10,
  risk_type: '',
  risk_level: '',
  status: '',
  action: '',
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
  { colKey: 'risk_type', title: '风险类型', width: 110 },
  { colKey: 'risk_level', title: '等级', width: 80 },
  { colKey: 'username', title: '关联账号', width: 140 },
  { colKey: 'ip', title: 'IP 地址', width: 130 },
  { colKey: 'rule_code', title: '命中规则', width: 160 },
  { colKey: 'summary', title: '摘要', minWidth: 180, ellipsis: true },
  { colKey: 'occur_count', title: '次数', width: 70 },
  { colKey: 'status', title: '处置状态', width: 90 },
  { colKey: 'actions', title: '已做动作', width: 170 },
  { colKey: 'handled_by', title: '处置人', width: 100 },
  { colKey: 'last_occurred_at', title: '最近发生', width: 160 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 320, fixed: 'right' },
])

const blacklistTypeOptions = BLACKLIST_TYPE_OPTIONS

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
  filters.action = ''
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
const timeline = ref<RiskEventActionInfo[]>([])
const timelineLoading = ref(false)

// prettyPayload 把后端存的 JSON 明细格式化后再展示。
//
// 解析失败就原样返回：明细是排查线索，宁可能看到原始文本，
// 也不该因为一次解析异常让整个抽屉内容消失。
const prettyPayload = computed(() => {
  const raw = detailRow.value?.detail_payload
  if (!raw) return '（无）'
  return prettyJSON(raw)
})

function prettyJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

// prettyActionDetail 把处置流水里的结果快照转成人话。
//
// 存的是 JSON（{"type":"ip","target_value":"1.2.3.4"}），直接铺在时间线上
// 会是一串带引号的机器文本；这里翻成「IP 1.2.3.4」这类可读描述。
function prettyActionDetail(raw?: string): string {
  if (!raw) return ''
  let payload: Record<string, unknown>
  try {
    payload = JSON.parse(raw) as Record<string, unknown>
  } catch {
    return ''
  }
  const parts: string[] = []
  if (typeof payload.type === 'string' && payload.target_value) {
    const label = BLACKLIST_TYPE_OPTIONS.find((item) => item.value === payload.type)?.label || payload.type
    parts.push(`黑名单：${label} ${String(payload.target_value)}`)
  }
  if (typeof payload.revoked === 'number') {
    parts.push(`失效会话 ${payload.revoked} 个`)
  }
  if (typeof payload.from === 'string' && typeof payload.to === 'string') {
    parts.push(
      `等级 ${RISK_LEVEL_LABEL[payload.from] || payload.from} → ${RISK_LEVEL_LABEL[payload.to] || payload.to}`,
    )
  }
  if (typeof payload.status === 'string') {
    parts.push(`待办 ${RISK_STATUS_LABEL[payload.status] || payload.status}`)
  }
  if (Array.isArray(payload.outcomes) && payload.outcomes.length) {
    parts.push((payload.outcomes as string[]).join('；'))
  }
  return parts.join(' · ')
}

async function openDetail(row: RiskEventInfo) {
  detailRow.value = row
  detailVisible.value = true
  timeline.value = []
  await loadTimeline(row.id)
}

// loadTimeline 拉取该事件的处置时间线。
//
// 失败只清空时间线，不打断抽屉：事件本体已经拿到了，时间线是补充信息。
async function loadTimeline(id: number) {
  timelineLoading.value = true
  try {
    const response = await getRiskEventActions(id)
    timeline.value = response.items
  } catch {
    timeline.value = []
  } finally {
    timelineLoading.value = false
  }
}

// —— 处置 / 忽略 ——
const closeDialog = reactive({
  visible: false,
  saving: false,
  id: 0,
  status: 'handled' as 'handled' | 'ignored',
  summary: '',
  subjectType: 'user',
  note: '',
  blacklist: false,
  revoke_sessions: false,
})

// 员工域不支持强制下线（会话表只承载客户会话），置灰并给出说明，
// 而不是让运营点了才知道做不到。
const canRevoke = computed(() => closeDialog.subjectType !== 'admin')

function openClose(row: RiskEventInfo, status: 'handled' | 'ignored') {
  closeDialog.id = row.id
  closeDialog.status = status
  closeDialog.summary = row.summary || `#${row.id}`
  closeDialog.subjectType = row.subject_type
  closeDialog.note = ''
  closeDialog.blacklist = false
  closeDialog.revoke_sessions = false
  closeDialog.visible = true
}

async function submitClose() {
  closeDialog.saving = true
  try {
    const payload = {
      note: closeDialog.note,
      blacklist: closeDialog.blacklist,
      revoke_sessions: closeDialog.revoke_sessions && canRevoke.value,
    }
    const updated =
      closeDialog.status === 'ignored'
        ? await ignoreRiskEvent(closeDialog.id, payload)
        : await handleRiskEvent(closeDialog.id, payload)
    MessagePlugin.success(
      closeDialog.status === 'ignored' ? '已忽略该风险事件' : '已处置该风险事件',
    )
    closeDialog.visible = false
    // 详情抽屉里操作的：同步刷新抽屉本体与时间线，否则看到的是操作前的快照。
    if (detailRow.value && detailRow.value.id === updated.id) {
      detailRow.value = updated
      void loadTimeline(updated.id)
    }
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '操作失败')
  } finally {
    closeDialog.saving = false
  }
}

// —— 拉黑 ——
const blacklistDialog = reactive({
  visible: false,
  saving: false,
  id: 0,
  summary: '',
  type: '',
  note: '',
  close_event: false,
})

function openBlacklist(row: RiskEventInfo) {
  blacklistDialog.id = row.id
  blacklistDialog.summary = row.summary || `#${row.id}`
  // 默认留空 = 由后端按事件可用字段推断（IP → 设备 → 账号）。
  // 前端不猜：只有后端知道该事件到底有哪些字段有值。
  blacklistDialog.type = ''
  blacklistDialog.note = ''
  blacklistDialog.close_event = false
  blacklistDialog.visible = true
}

async function submitBlacklist() {
  blacklistDialog.saving = true
  try {
    const created = await blacklistRiskEvent(blacklistDialog.id, {
      type: blacklistDialog.type || undefined,
      note: blacklistDialog.note,
      close_event: blacklistDialog.close_event,
    })
    const label = BLACKLIST_TYPE_OPTIONS.find((item) => item.value === created.type)?.label || created.type
    MessagePlugin.success(`已拉黑${label}「${created.target_value}」，该来源下次登录将被拒绝`)
    blacklistDialog.visible = false
    await refreshAfterAction(blacklistDialog.id)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '拉黑失败')
  } finally {
    blacklistDialog.saving = false
  }
}

// —— 失效会话 ——
const revokeDialog = reactive({
  visible: false,
  saving: false,
  id: 0,
  summary: '',
  subjectType: 'user',
  userId: 0,
  username: '',
  note: '',
  close_event: false,
})

const revokeTarget = computed(() =>
  revokeDialog.subjectType === 'admin'
    ? `员工后台账号 ${revokeDialog.username}`
    : `客户 ${revokeDialog.username}（ID #${revokeDialog.userId}）的全部有效会话`,
)

function openRevoke(row: RiskEventInfo) {
  // 员工域在打开前就拦掉并说明原因：后端也会拒绝，但让运营先看到原因比
  // 点了再弹错误好。这里不静默放行 —— 静默假成功正是改造前的坑。
  if (row.subject_type === 'admin') {
    MessagePlugin.warning('员工后台账号暂不支持强制下线，请改用拉黑，或到员工管理里禁用该账号')
    return
  }
  revokeDialog.id = row.id
  revokeDialog.summary = row.summary || `#${row.id}`
  revokeDialog.subjectType = row.subject_type
  revokeDialog.userId = row.user_id
  revokeDialog.username = row.username
  revokeDialog.note = ''
  revokeDialog.close_event = false
  revokeDialog.visible = true
}

async function submitRevoke() {
  revokeDialog.saving = true
  try {
    const response = await revokeRiskEventSessions(revokeDialog.id, {
      note: revokeDialog.note,
      close_event: revokeDialog.close_event,
    })
    if (response.meta.total > 0) {
      MessagePlugin.success(`已失效 ${response.meta.total} 个会话，该账号需重新登录`)
    } else {
      // 一个都没踢到也是有效结果（本来就不在线），但要如实说，
      // 不能让运营以为已经踢下线了。
      MessagePlugin.info('该账号当前没有有效会话，无需失效')
    }
    revokeDialog.visible = false
    await refreshAfterAction(revokeDialog.id)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '失效会话失败')
  } finally {
    revokeDialog.saving = false
  }
}

// refreshAfterAction 管控动作之后的统一刷新：列表 + 打开着的详情抽屉。
//
// 必须重新拉单条事件（而不是拿旧对象改）：动作会追加处置流水，
// 列表的「已做动作」列与抽屉里的时间线都要跟着变，否则页面显示的还是旧快照。
async function refreshAfterAction(eventID: number) {
  await loadData()
  if (detailRow.value && detailRow.value.id === eventID) {
    const fresh = tableData.value.find((item) => item.id === eventID)
    if (fresh) {
      detailRow.value = fresh
    }
    void loadTimeline(eventID)
  }
}

// —— 等级调整 ——
const levelDialog = reactive({
  visible: false,
  saving: false,
  id: 0,
  summary: '',
  level: 'low',
  note: '',
  closeEvent: false,
  closeAs: 'handled' as 'handled' | 'ignored',
})

function openLevel(row: RiskEventInfo) {
  levelDialog.id = row.id
  levelDialog.summary = row.summary || `#${row.id}`
  levelDialog.level = row.risk_level || 'low'
  levelDialog.note = ''
  levelDialog.closeEvent = false
  levelDialog.closeAs = 'handled'
  levelDialog.visible = true
}

async function submitLevel() {
  levelDialog.saving = true
  try {
    const updated = await updateRiskEventLevel(levelDialog.id, {
      risk_level: levelDialog.level,
      note: levelDialog.note,
      close_event: levelDialog.closeEvent,
      close_as: levelDialog.closeEvent ? levelDialog.closeAs : undefined,
    })
    MessagePlugin.success(`风险等级已调整为「${RISK_LEVEL_LABEL[updated.risk_level] || updated.risk_level}」`)
    levelDialog.visible = false
    if (detailRow.value && detailRow.value.id === updated.id) {
      detailRow.value = updated
      void loadTimeline(updated.id)
    }
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '调整等级失败')
  } finally {
    levelDialog.saving = false
  }
}

// 行内「处置 / 忽略」打开对话框（而不是直接调接口）：
// 关单时通常要写一句原因，也顺带给了「同时拉黑/踢会话」的入口。
// 这两个状态可以互相改回（忽略后仍能再处置），所以不需要二次确认框。
onMounted(() => {
  void loadData()
})

// mobileActions 移动端动作项：已关闭的待办不再重复给出「再次关单」入口。
function mobileActions(row: RiskEventInfo): MobileActionItem[] {
  const items: MobileActionItem[] = [
    { content: '详情', value: 'detail' },
    { content: '调整等级', value: 'level' },
  ]
  if (row.status !== 'ignored') {
    items.push({ content: '忽略', value: 'ignore' })
  }
  if (row.status !== 'handled') {
    items.push({ content: '处置', value: 'resolve' })
  }
  items.push({ content: '拉黑', value: 'blacklist', theme: 'error' })
  items.push({ content: '失效会话', value: 'revoke' })
  return items
}

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
      openClose(row, 'ignored')
      break
    case 'resolve':
      openClose(row, 'handled')
      break
    case 'blacklist':
      openBlacklist(row)
      break
    case 'revoke':
      openRevoke(row)
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

.cell-muted,
.detail-muted {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.detail-muted {
  margin-left: 6px;
}

.detail-payload,
.detail-timeline {
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
  max-height: 220px;
  overflow: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}

.timeline {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.timeline__item {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding-bottom: 12px;
  border-bottom: 1px dashed var(--color-border);
}

.timeline__item:last-child {
  padding-bottom: 0;
  border-bottom: none;
}

.timeline__body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.timeline__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.timeline__note {
  font-size: 13px;
  color: var(--color-foreground);
  word-break: break-word;
}

.timeline__detail {
  font-size: 12px;
  color: var(--color-muted-foreground);
  word-break: break-word;
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
