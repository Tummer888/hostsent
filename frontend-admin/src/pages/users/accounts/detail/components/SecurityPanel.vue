<template>
  <div class="tabs-section">
    <t-tabs v-model="subTab" theme="normal" class="sub-tabs">
      <t-tab-panel value="logins" :label="`登录日志（${summary?.login_count ?? 0}）`">
        <div class="table-card__head">
          <h3 class="card-title">登录日志</h3>
          <span class="table-card__meta">共 {{ loginPagination.total }} 条</span>
        </div>
        <t-table
          row-key="id"
          :data="loginLogs"
          :columns="loginColumns"
          :loading="loginLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="isMobile ? undefined : loginPagination"
          @page-change="onLoginPageChange"
        >
          <template #result="{ row }">
            <div>
              <t-tag :theme="loginResultTheme(row.result)" variant="light" size="small" shape="round">
                {{ loginResultLabel(row.result) }}
              </t-tag>
              <div v-if="row.failure_reason" class="cell-sub">{{ row.failure_reason }}</div>
            </div>
          </template>
          <template #login_type="{ row }">
            <span>{{ loginTypeLabel(row.login_type) }}</span>
          </template>
          <template #ip="{ row }">
            <div>{{ row.ip || '—' }}</div>
            <div class="cell-sub">{{ row.ip_region || '—' }}</div>
          </template>
          <template #user_agent="{ row }">
            <span class="cell-sub">{{ row.user_agent || '—' }}</span>
          </template>
          <template #created_at="{ row }">
            <span class="time-text">{{ formatDateTime(row.created_at) }}</span>
          </template>
          <template #empty>
            <t-empty description="该用户暂无登录记录" />
          </template>
        </t-table>
        <MobilePagination
          v-if="isMobile"
          :current="loginMobile.current"
          :page-size="loginMobile.pageSize"
          :total="loginMobile.total"
          @go="goLoginMobilePage"
          @page-size="onLoginMobileSize"
        />
      </t-tab-panel>

      <t-tab-panel value="sessions" :label="`在线会话（${summary?.active_session_count ?? 0}）`">
        <div class="table-card__head">
          <h3 class="card-title">在线会话</h3>
          <div class="head-actions">
            <span class="table-card__meta">共 {{ sessionPagination.total }} 条</span>
            <t-button
              v-permission="'security:session:manage'"
              size="small"
              variant="outline"
              theme="danger"
              :disabled="!sessionPagination.total"
              :title="sessionPagination.total ? '强制下线该用户全部会话' : '当前没有可下线的会话'"
              @click="openRevokeAll"
            >
              强制下线全部会话
            </t-button>
          </div>
        </div>
        <t-table
          row-key="id"
          :data="sessions"
          :columns="sessionColumns"
          :loading="sessionLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="isMobile ? undefined : sessionPagination"
          @page-change="onSessionPageChange"
        >
          <template #platform="{ row }">
            <span>{{ platformLabel(row.platform) }}</span>
          </template>
          <template #ip="{ row }">
            <div>{{ row.ip || '—' }}</div>
            <div class="cell-sub">{{ row.ip_region || '—' }}</div>
          </template>
          <template #status="{ row }">
            <t-tag :theme="sessionStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ sessionStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #last_active_at="{ row }">
            <span class="time-text">{{ formatDateTime(row.last_active_at) }}</span>
          </template>
          <template #op="{ row }">
            <t-link
              v-permission="'security:session:manage'"
              theme="danger"
              hover="color"
              :disabled="row.status !== 'active'"
              @click="revokeOne(row)"
            >
              下线
            </t-link>
          </template>
          <template #empty>
            <t-empty description="该用户当前没有在线会话" />
          </template>
        </t-table>
        <MobilePagination
          v-if="isMobile"
          :current="sessionMobile.current"
          :page-size="sessionMobile.pageSize"
          :total="sessionMobile.total"
          @go="goSessionMobilePage"
          @page-size="onSessionMobileSize"
        />
      </t-tab-panel>

      <t-tab-panel value="risks" :label="`风险事件（${summary?.risk_event_count ?? 0}）`">
        <div class="table-card__head">
          <h3 class="card-title">风险事件</h3>
          <span class="table-card__meta">共 {{ riskPagination.total }} 条</span>
        </div>
        <t-table
          row-key="id"
          :data="riskEvents"
          :columns="riskColumns"
          :loading="riskLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="isMobile ? undefined : riskPagination"
          @page-change="onRiskPageChange"
        >
          <template #risk_type="{ row }">
            <span class="cell-strong">{{ row.risk_type || '—' }}</span>
            <div class="cell-sub">{{ row.rule_code || '—' }}</div>
          </template>
          <template #risk_level="{ row }">
            <t-tag :theme="riskLevelTheme(row.risk_level)" variant="light" size="small" shape="round">
              {{ riskLevelLabel(row.risk_level) }}
            </t-tag>
          </template>
          <template #summary="{ row }">
            <span>{{ row.summary || '—' }}</span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="riskEventStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ riskEventStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #last_occurred_at="{ row }">
            <div class="time-text">{{ formatDateTime(row.last_occurred_at) }}</div>
            <div class="cell-sub">累计 {{ row.occur_count }} 次</div>
          </template>
          <template #empty>
            <t-empty description="该用户暂无风险事件" />
          </template>
        </t-table>
        <MobilePagination
          v-if="isMobile"
          :current="riskMobile.current"
          :page-size="riskMobile.pageSize"
          :total="riskMobile.total"
          @go="goRiskMobilePage"
          @page-size="onRiskMobileSize"
        />
      </t-tab-panel>
    </t-tabs>

    <t-dialog
      v-model:visible="revokeAllVisible"
      header="强制下线全部会话"
      width="440px"
      :confirm-btn="{ content: '确认下线', theme: 'danger', loading: revokeAllSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="confirmRevokeAll"
      @close="revokeAllVisible = false"
    >
      <t-form label-align="top" :data="revokeAllForm" @submit.prevent>
        <t-form-item label="用户">
          <t-input :model-value="username || '—'" disabled />
        </t-form-item>
        <t-form-item label="下线原因">
          <t-textarea
            v-model="revokeAllForm.reason"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="选填，将记入会话的撤销原因"
          />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import MobilePagination from '@/components/mobile-pagination/index.vue'
import {
  getLoginLogList,
  getRiskEventList,
  getSessionList,
  revokeSession,
  revokeUserAllSessions,
} from '@/api/security'
import {
  formatDateTime,
  loginResultLabel,
  loginResultTheme,
  riskEventStatusLabel,
  riskEventStatusTheme,
  riskLevelLabel,
  riskLevelTheme,
  sessionStatusLabel,
  sessionStatusTheme,
} from '@/pages/users/constants'
import type { LoginLogInfo, RiskEventInfo, SessionInfo } from '@/api/security'
import type { UserDetailSummary } from '@/api/user'

const props = defineProps<{
  userId: number
  username: string
  isMobile: boolean
  summary?: UserDetailSummary | null
}>()

const subTab = ref<'logins' | 'sessions' | 'risks'>('logins')

const loginTypeLabels: Record<string, string> = {
  password: '密码登录',
  sms: '短信登录',
  oauth: '第三方登录',
  wechat: '微信',
  qq: 'QQ',
  refresh: '令牌续期',
}

function loginTypeLabel(type: string): string {
  return loginTypeLabels[type] || type || '—'
}

const platformLabels: Record<string, string> = {
  web: 'Web',
  admin: '管理端',
  mobile: '移动端',
  api: 'API',
}

function platformLabel(platform: string): string {
  return platformLabels[platform] || platform || '—'
}

// ---------- 登录日志 ----------
const loginLogs = ref<LoginLogInfo[]>([])
const loginLoading = ref(false)
const loginPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const loginMobile = reactive({ current: 1, pageSize: 10, total: 0 })
const loginColumns: PrimaryTableCol<LoginLogInfo>[] = [
  { colKey: 'created_at', title: '时间', width: 150 },
  { colKey: 'result', title: '结果', width: 120 },
  { colKey: 'login_type', title: '方式', width: 110 },
  { colKey: 'ip', title: 'IP / 归属地', minWidth: 170 },
  { colKey: 'user_agent', title: '客户端', minWidth: 180 },
]

async function loadLoginLogs() {
  loginLoading.value = true
  try {
    const data = await getLoginLogList({
      user_id: props.userId,
      page: loginPagination.current,
      page_size: loginPagination.pageSize,
    })
    loginLogs.value = data.items || []
    loginPagination.total = data.meta?.total || 0
    loginMobile.total = loginPagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载登录日志失败')
  } finally {
    loginLoading.value = false
  }
}

function onLoginPageChange(pageInfo: PageInfo) {
  loginPagination.current = pageInfo.current
  loginPagination.pageSize = pageInfo.pageSize
  Object.assign(loginMobile, { current: pageInfo.current, pageSize: pageInfo.pageSize })
  void loadLoginLogs()
}

function goLoginMobilePage(target: number) {
  loginPagination.current = target
  loginMobile.current = target
  void loadLoginLogs()
}

function onLoginMobileSize(size: number) {
  loginPagination.pageSize = size
  loginPagination.current = 1
  loginMobile.pageSize = size
  loginMobile.current = 1
  void loadLoginLogs()
}

// ---------- 会话 ----------
const sessions = ref<SessionInfo[]>([])
const sessionLoading = ref(false)
const sessionPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const sessionMobile = reactive({ current: 1, pageSize: 10, total: 0 })
const sessionColumns: PrimaryTableCol<SessionInfo>[] = [
  { colKey: 'platform', title: '平台', width: 100 },
  { colKey: 'ip', title: 'IP / 归属地', minWidth: 170 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'login_at', title: '登录时间', width: 150 },
  { colKey: 'last_active_at', title: '最近活跃', width: 150 },
  { colKey: 'op', title: '操作', width: 80, fixed: 'right' },
]

async function loadSessions() {
  sessionLoading.value = true
  try {
    const data = await getSessionList({
      user_id: props.userId,
      page: sessionPagination.current,
      page_size: sessionPagination.pageSize,
    })
    sessions.value = data.items || []
    sessionPagination.total = data.meta?.total || 0
    sessionMobile.total = sessionPagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载会话失败')
  } finally {
    sessionLoading.value = false
  }
}

function onSessionPageChange(pageInfo: PageInfo) {
  sessionPagination.current = pageInfo.current
  sessionPagination.pageSize = pageInfo.pageSize
  Object.assign(sessionMobile, { current: pageInfo.current, pageSize: pageInfo.pageSize })
  void loadSessions()
}

function goSessionMobilePage(target: number) {
  sessionPagination.current = target
  sessionMobile.current = target
  void loadSessions()
}

function onSessionMobileSize(size: number) {
  sessionPagination.pageSize = size
  sessionPagination.current = 1
  sessionMobile.pageSize = size
  sessionMobile.current = 1
  void loadSessions()
}

async function revokeOne(row: SessionInfo) {
  try {
    await revokeSession(row.id, { reason: '管理员在用户详情页手动下线' })
    MessagePlugin.success('会话已下线')
    void loadSessions()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '下线失败')
  }
}

const revokeAllVisible = ref(false)
const revokeAllSubmitting = ref(false)
const revokeAllForm = reactive({ reason: '' })

function openRevokeAll() {
  revokeAllForm.reason = ''
  revokeAllVisible.value = true
}

async function confirmRevokeAll() {
  revokeAllSubmitting.value = true
  try {
    const result = await revokeUserAllSessions({ user_id: props.userId, reason: revokeAllForm.reason })
    MessagePlugin.success(`已下线 ${result.items?.length ?? 0} 个会话`)
    revokeAllVisible.value = false
    void loadSessions()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '操作失败')
  } finally {
    revokeAllSubmitting.value = false
  }
}

// ---------- 风险事件 ----------
const riskEvents = ref<RiskEventInfo[]>([])
const riskLoading = ref(false)
const riskPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const riskMobile = reactive({ current: 1, pageSize: 10, total: 0 })
const riskColumns: PrimaryTableCol<RiskEventInfo>[] = [
  { colKey: 'risk_type', title: '风险类型', minWidth: 180 },
  { colKey: 'risk_level', title: '级别', width: 90 },
  { colKey: 'summary', title: '摘要', minWidth: 220 },
  { colKey: 'status', title: '处理状态', width: 110 },
  { colKey: 'last_occurred_at', title: '最近发生', width: 160 },
]

async function loadRiskEvents() {
  riskLoading.value = true
  try {
    const data = await getRiskEventList({
      user_id: props.userId,
      page: riskPagination.current,
      page_size: riskPagination.pageSize,
    })
    riskEvents.value = data.items || []
    riskPagination.total = data.meta?.total || 0
    riskMobile.total = riskPagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载风险事件失败')
  } finally {
    riskLoading.value = false
  }
}

function onRiskPageChange(pageInfo: PageInfo) {
  riskPagination.current = pageInfo.current
  riskPagination.pageSize = pageInfo.pageSize
  Object.assign(riskMobile, { current: pageInfo.current, pageSize: pageInfo.pageSize })
  void loadRiskEvents()
}

function goRiskMobilePage(target: number) {
  riskPagination.current = target
  riskMobile.current = target
  void loadRiskEvents()
}

function onRiskMobileSize(size: number) {
  riskPagination.pageSize = size
  riskPagination.current = 1
  riskMobile.pageSize = size
  riskMobile.current = 1
  void loadRiskEvents()
}

// 子 Tab 懒加载：一次只打一个接口，避免详情页一打开就并发三路安全查询。
watch(
  () => [props.userId, subTab.value] as const,
  ([uid, tab]) => {
    if (!uid) return
    if (tab === 'logins' && !loginLogs.value.length) void loadLoginLogs()
    if (tab === 'sessions' && !sessions.value.length) void loadSessions()
    if (tab === 'risks' && !riskEvents.value.length) void loadRiskEvents()
  },
  { immediate: true },
)

watch(
  () => props.userId,
  () => {
    loginPagination.current = 1
    sessionPagination.current = 1
    riskPagination.current = 1
    loginLogs.value = []
    sessions.value = []
    riskEvents.value = []
  },
)

defineExpose({ reload: loadLoginLogs })
</script>

<style scoped>
.head-actions {
  display: flex;
  align-items: center;
  gap: var(--space-md);
}
</style>