<template>
  <div class="page-body log-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">日志保留策略</h2>
          <p class="page-header__desc">
            逐源配置「动作 / 保留天数 / 批次大小」。<b>只有 ops 类源允许「清理」</b>，
            audit 类只能备份不能删；<b>{{ MIN_RETENTION_DAYS }} 天是代码硬地板</b>，
            设置里再小也会被后端夹回，且水位线永远不会越过保留期下界。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-input v-model="keyword" placeholder="搜索源名称 / key" clearable style="width: 220px">
          <template #prefix-icon><SearchIcon aria-hidden="true" /></template>
        </t-input>
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">日志分组</span>
        <t-select v-model="groupFilter" clearable placeholder="全部分组" :options="groupOptions" />
      </div>
      <div class="field">
        <span class="field__label">数据分级</span>
        <t-select v-model="classFilter" clearable placeholder="全部分级" :options="classOptions" />
      </div>
      <div class="field">
        <span class="field__label">清理动作</span>
        <t-select v-model="actionFilter" clearable placeholder="全部动作" :options="POLICY_ACTION_OPTIONS" />
      </div>
      <div class="field">
        <span class="field__label">启用状态</span>
        <t-select v-model="enabledFilter" clearable placeholder="全部" :options="enabledOptions" />
      </div>
      <template #actions>
        <span class="table-card__meta">
          共 {{ formatNumber(filtered.length) }} 个源 · 其中可清理 {{ cleanableCount }} 个
        </span>
      </template>
    </FilterCard>

    <section class="surface-card table-card">
      <t-table
        row-key="source_key"
        :data="tableData"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #source_key="{ row }">
          <div class="stack-cell">
            <span class="cell-strong">{{ row.display_name }}</span>
            <span class="sub-text">{{ row.source_key }}</span>
            <span v-if="row.independent_page" class="sub-text">
              另有独立页面：{{ row.independent_page }}
            </span>
          </div>
        </template>
        <template #group_label="{ row }">
          <div class="stack-cell">
            <span class="cell-muted">{{ row.group_label }}</span>
            <t-tag :theme="classTheme(row.class)" variant="light" size="small" shape="round">
              {{ classLabel(row.class) }}
            </t-tag>
          </div>
        </template>
        <template #action="{ row }">
          <t-tag :theme="policyActionTheme(row.action)" variant="light" size="small" shape="round">
            {{ row.action_label || policyActionLabel(row.action) }}
          </t-tag>
        </template>
        <template #retention_days="{ row }">
          <div class="stack-cell">
            <span class="cell-strong">{{ row.retention_days }} 天</span>
            <span v-if="row.retention_days !== row.default_retention_days" class="sub-text">
              默认 {{ row.default_retention_days }} 天
            </span>
            <span
              v-if="retentionWarn(row.source_key, row.retention_days).text"
              :class="retentionWarn(row.source_key, row.retention_days).level === 'danger' ? 'danger-hint' : 'warn-hint'"
            >
              {{ retentionWarn(row.source_key, row.retention_days).text }}
            </span>
          </div>
        </template>
        <template #batch_size="{ row }">
          <span class="time-text">{{ formatNumber(row.batch_size) }}</span>
        </template>
        <template #enabled="{ row }">
          <t-switch
            :value="row.enabled"
            :disabled="!canPolicy || savingKey === row.source_key"
            :loading="savingKey === row.source_key"
            @change="(v: unknown) => toggleEnabled(row, Boolean(v))"
          />
        </template>
        <template #last_run_at="{ row }">
          <div class="stack-cell">
            <span class="time-text">{{ formatTime(row.last_run_at) }}</span>
            <span v-if="row.last_deleted" class="sub-text">上次删除 {{ formatNumber(row.last_deleted) }} 行</span>
          </div>
        </template>
        <template #configured="{ row }">
          <t-tag v-if="row.configured" variant="light" size="small" theme="success" shape="round">已配置</t-tag>
          <t-tag v-else variant="light" size="small" theme="default" shape="round">默认值</t-tag>
        </template>
        <template #operation="{ row }">
          <div class="action-cell">
            <t-link v-if="canPolicy" theme="primary" hover="color" @click="openEdit(row)">配置</t-link>
            <t-link
              v-if="canExport"
              theme="default"
              hover="color"
              :disabled="exportingKey === row.source_key"
              @click="handleExport(row)"
            >
              导出备份
            </t-link>
          </div>
        </template>
        <template #empty>
          <t-empty description="没有匹配的日志源" />
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

      <p class="warn-hint">
        「导出备份」会按当前保留策略把该源水位线之前的数据导出为 JSONL 并登记到「清理任务」页的导出文件列表，
        <b>不会删除任何数据</b>；想真正删除请到「清理任务」页执行。
      </p>
    </section>

    <!-- 逐源配置 -->
    <t-dialog
      v-model:visible="editVisible"
      :header="`配置保留策略 · ${editing?.display_name || ''}`"
      width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
    >
      <template v-if="editing">
        <div class="recon-row">
          <span class="recon-row__label">源 key</span>
          <span class="recon-row__value">{{ editing.source_key }}</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">数据表 / 时间列</span>
          <span class="recon-row__value">{{ editing.table }} · {{ editing.time_column }}</span>
        </div>
        <div v-if="editing.delete_guard" class="recon-row">
          <span class="recon-row__label">删除保护条件</span>
          <span class="recon-row__value">{{ editing.delete_guard }}</span>
        </div>
        <div v-if="editing.remark" class="recon-row">
          <span class="recon-row__label">备注</span>
          <span class="recon-row__value">{{ editing.remark }}</span>
        </div>

        <div class="confirm-field">
          <span class="field__label">动作</span>
          <t-select v-model="form.action" :options="actionOptions" />
          <p v-if="!editing.cleanable" class="warn-hint">
            {{ editing.display_name }} 属于{{ classLabel(editing.class) }}，只允许「仅备份不清理」或「保留不处理」。
          </p>
          <p v-else-if="form.action === 'keep'" class="warn-hint">该源将不再被自动清理。</p>
          <p v-else-if="form.action === 'archive_only'" class="warn-hint">
            该源只做冷备导出，不会删除任何行；请同时配置冷备周期。
          </p>
        </div>

        <div class="confirm-field">
          <span class="field__label">
            保留天数（硬地板 {{ MIN_RETENTION_DAYS }} 天，不足会被后端夹回）
          </span>
          <t-input-number v-model="form.retention_days" :min="MIN_RETENTION_DAYS" :max="3650" theme="normal" />
          <p
            v-if="formWarning.text"
            :class="formWarning.level === 'danger' ? 'danger-hint' : 'warn-hint'"
          >
            {{ formWarning.text }}
          </p>
          <p v-if="isInbox(editing.source_key)" class="warn-hint">
            站内信是用户可见内容，seed 默认 {{ INBOX_RETENTION_DAYS }} 天；低于它用户的「我的消息」历史会消失。
          </p>
        </div>

        <div class="confirm-field">
          <span class="field__label">批大小（单事务删除行数，5000 为推荐值）</span>
          <t-input-number v-model="form.batch_size" :min="100" :max="50000" :step="500" theme="normal" />
        </div>

        <div class="confirm-field">
          <span class="field__label">冷备周期（仅「仅备份不清理」使用）</span>
          <t-input v-model="form.archive_cron" placeholder="monthly / weekly / daily" />
        </div>

        <div class="confirm-field">
          <t-checkbox v-model="form.enabled">启用该源的策略（关闭后不会被调度清理）</t-checkbox>
        </div>

        <div class="confirm-field">
          <span class="field__label">备注</span>
          <t-textarea v-model="form.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="为何这样配置" />
        </div>
      </template>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'

import { RefreshIcon, SearchIcon, SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, DialogPlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { exportLogs, getPolicies, updatePolicy, type LogPolicyInfo } from '@/api/logcenter'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import {
  INBOX_RETENTION_DAYS,
  MIN_RETENTION_DAYS,
  POLICY_ACTION_OPTIONS,
  classLabel,
  classTheme,
  formatNumber,
  formatTime,
  policyActionLabel,
  policyActionTheme,
  retentionWarn,
} from '@/pages/system/logs/constants'
import { useUserStore } from '@/store'

defineOptions({ name: 'SystemLogsPolicy' })

const userStore = useUserStore()
const canPolicy = computed(() => userStore.permissions?.includes('log:policy') || userStore.isSuperAdmin)
const canExport = computed(() => userStore.permissions?.includes('log:export') || userStore.isSuperAdmin)
const { isMobile } = useIsMobile()

// ===========================================================================
// 列表与筛选
// ===========================================================================
const loading = ref(false)
const policies = ref<LogPolicyInfo[]>([])
const keyword = ref('')
const groupFilter = ref<string>()
const classFilter = ref<string>()
const actionFilter = ref<string>()
const enabledFilter = ref<string>()
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })
let searchTimer: ReturnType<typeof setTimeout> | null = null

const groupOptions = computed(() => {
  const seen = new Map<string, string>()
  for (const item of policies.value) seen.set(item.group, item.group_label)
  return Array.from(seen, ([value, label]) => ({ value, label }))
})

const classOptions = [
  { value: 'ops', label: '运维流水' },
  { value: 'audit', label: '审计留痕' },
]

const enabledOptions = [
  { value: 'on', label: '已启用' },
  { value: 'off', label: '已停用' },
]

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return policies.value.filter((item) => {
    if (groupFilter.value && item.group !== groupFilter.value) return false
    if (classFilter.value && item.class !== classFilter.value) return false
    if (actionFilter.value && item.action !== actionFilter.value) return false
    if (enabledFilter.value === 'on' && !item.enabled) return false
    if (enabledFilter.value === 'off' && item.enabled) return false
    if (kw && !`${item.display_name} ${item.source_key}`.toLowerCase().includes(kw)) return false
    return true
  })
})

const cleanableCount = computed(() => policies.value.filter((item) => item.cleanable).length)

/** 桌面端表格分页切片；移动端由 MobilePagination 驱动同一份页码状态 */
const tableData = computed(() => {
  const start = (pagination.current - 1) * pagination.pageSize
  return filtered.value.slice(start, start + pagination.pageSize)
})

const columns: PrimaryTableCol<LogPolicyInfo>[] = [
  { colKey: 'source_key', title: '日志源', minWidth: 200 },
  { colKey: 'group_label', title: '分组 / 分级', width: 150 },
  { colKey: 'action', title: '动作', width: 130 },
  { colKey: 'retention_days', title: '保留期', minWidth: 180 },
  { colKey: 'batch_size', title: '批大小', width: 100, align: 'right' },
  { colKey: 'enabled', title: '启用', width: 80, align: 'center' },
  { colKey: 'last_run_at', title: '上次执行', width: 170 },
  { colKey: 'configured', title: '配置', width: 90, align: 'center' },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 90 : 150, fixed: 'right', align: 'center' },
]

async function load() {
  loading.value = true
  try {
    const resp = await getPolicies()
    policies.value = resp.items || []
    syncTotal()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载保留策略失败')
  } finally {
    loading.value = false
  }
}

/** 筛选条件变化后总数与页码都要跟着收敛（否则会出现「第 3 页 0 条」） */
function syncTotal() {
  pagination.total = filtered.value.length
  mobilePage.total = filtered.value.length
  const maxPage = Math.max(1, Math.ceil(filtered.value.length / pagination.pageSize))
  if (pagination.current > maxPage) pagination.current = maxPage
  mobilePage.current = pagination.current
  mobilePage.pageSize = pagination.pageSize
}

watch([keyword, groupFilter, classFilter, actionFilter, enabledFilter], () => {
  if (searchTimer) clearTimeout(searchTimer)
  // 输入框防抖：避免每敲一个字都重算一遍 26 行的筛选与切片
  searchTimer = setTimeout(() => {
    pagination.current = 1
    mobilePage.current = 1
    syncTotal()
  }, 200)
})

onUnmounted(() => {
  if (searchTimer) clearTimeout(searchTimer)
})

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
}

function goMobilePage(target: number) {
  const maxPage = Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize))
  const clamped = Math.min(Math.max(target, 1), maxPage)
  if (clamped === mobilePage.current) return
  handlePageChange({ current: clamped, pageSize: mobilePage.pageSize } as PageInfo)
}

function handleMobilePageSizeChange(pageSize: number) {
  handlePageChange({ current: 1, pageSize } as PageInfo)
}

function isInbox(sourceKey: string): boolean {
  return sourceKey === 'notify_inbox' || sourceKey === 'notify_read'
}

// ===========================================================================
// 启用开关：整行回传，避免只发 enabled 把动作/天数覆盖成零值
// ===========================================================================
const savingKey = ref('')
const exportingKey = ref('')

async function toggleEnabled(row: LogPolicyInfo, enabled: boolean) {
  savingKey.value = row.source_key
  try {
    const updated = await updatePolicy(row.source_key, {
      action: row.action,
      retention_days: row.retention_days,
      batch_size: row.batch_size,
      enabled,
      archive_cron: row.archive_cron,
      remark: row.remark,
    })
    MessagePlugin.success(`${row.display_name} 已${enabled ? '启用' : '停用'}`)
    if (updated) {
      const idx = policies.value.findIndex((item) => item.source_key === row.source_key)
      if (idx >= 0) policies.value[idx] = updated
    } else {
      await load()
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新失败')
    await load()
  } finally {
    savingKey.value = ''
  }
}

// ===========================================================================
// 逐源配置
// ===========================================================================
const editVisible = ref(false)
const saving = ref(false)
const editing = ref<LogPolicyInfo | null>(null)
const form = reactive({
  action: 'keep',
  retention_days: MIN_RETENTION_DAYS,
  batch_size: 5000,
  enabled: true,
  archive_cron: '',
  remark: '',
})

/** audit 类源的下拉里不能出现「清理」（后端也会拒） */
const actionOptions = computed(() => {
  if (editing.value && !editing.value.cleanable) {
    return POLICY_ACTION_OPTIONS.filter((opt) => opt.value !== 'clean')
  }
  return POLICY_ACTION_OPTIONS
})

const formWarning = computed(() => {
  if (!editing.value) return { level: 'none' as const, text: '' }
  return retentionWarn(editing.value.source_key, form.retention_days)
})

function openEdit(row: LogPolicyInfo) {
  editing.value = row
  form.action = row.action
  form.retention_days = row.retention_days
  form.batch_size = row.batch_size
  form.enabled = row.enabled
  form.archive_cron = row.archive_cron
  form.remark = row.remark
  editVisible.value = true
}

async function handleSave() {
  const row = editing.value
  if (!row) return
  if (!row.cleanable && form.action === 'clean') {
    MessagePlugin.warning('该源属于审计留痕，不允许清理')
    return
  }
  if (form.retention_days < MIN_RETENTION_DAYS) {
    MessagePlugin.warning(`保留期不得低于 ${MIN_RETENTION_DAYS} 天`)
    return
  }
  // 高危区间（<30 天，或站内信 <365 天）要求二次确认；运营仍有最终决定权。
  if (formWarning.value.level === 'danger') {
    const dialog = DialogPlugin.confirm({
      header: '确认缩短保留期',
      body: `${row.display_name}：${formWarning.value.text}。缩短后超期的日志会在下次清理时被删除，确认继续？`,
      confirmBtn: { content: '确认保存', theme: 'danger' },
      cancelBtn: { content: '返回修改' },
      onConfirm: () => {
        dialog.destroy()
        persistPolicy(row)
      },
      onClose: () => dialog.destroy(),
    })
    return
  }
  await persistPolicy(row)
}

async function persistPolicy(row: LogPolicyInfo) {
  saving.value = true
  try {
    await updatePolicy(row.source_key, {
      action: form.action,
      retention_days: form.retention_days,
      batch_size: form.batch_size,
      enabled: form.enabled,
      archive_cron: form.archive_cron,
      remark: form.remark,
    })
    MessagePlugin.success('已保存')
    editVisible.value = false
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

// ===========================================================================
// 立即导出备份（只导出不删除）
// ===========================================================================
async function handleExport(row: LogPolicyInfo) {
  exportingKey.value = row.source_key
  try {
    const resp = await exportLogs({ source: row.source_key, format: 'jsonl' })
    MessagePlugin.success(`已导出 ${formatNumber(resp.row_count)} 行（${resp.file_name}）`)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '导出失败')
  } finally {
    exportingKey.value = ''
  }
}

onMounted(load)
</script>

<style scoped>
.stack-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.sub-text {
  font-size: 11px;
  color: var(--td-text-color-placeholder, #9aa2af);
  font-variant-numeric: tabular-nums;
}
</style>

<style lang="css">
@import '../shared.css';
</style>
