<template>
  <div class="page-body log-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <DeleteIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">日志清理</h2>
          <p class="page-header__desc">
            清理固定是「先导出备份 → 校验 sha256 → 分批删除」。任一批次出错整个任务标记失败：
            <b>已删批次不回滚，但备份文件可回灌</b>。保留期是硬下界，<code>now() - retention_days</code>
            之内的行永不进入候选。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="previewing" @click="handlePreview">
          <template #icon><SearchIcon aria-hidden="true" /></template>
          预演
        </t-button>
        <t-button theme="danger" :disabled="!canCleanup" @click="openRun">
          <template #icon><DeleteIcon aria-hidden="true" /></template>
          执行清理
        </t-button>
        <t-button variant="outline" :loading="jobsLoading" @click="reload">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <!-- 预演结果：算候选行数与预估体积，不导出不删除 -->
    <section v-if="preview" class="surface-card table-card">
      <div class="table-card__head">
        <h3 class="card-title">预演结果（未导出、未删除）</h3>
        <t-space size="small" align="center">
          <span class="table-card__meta">
            候选 {{ formatRows(preview.eligible_rows) }}
            <template v-if="preview.guarded_rows > 0">
              · 受保护跳过 {{ formatRows(preview.guarded_rows) }}
            </template>
          </span>
          <t-link theme="default" hover="color" @click="preview = null">收起</t-link>
        </t-space>
      </div>
      <p v-if="preview.message" :class="preview.export_before_delete ? 'warn-hint' : 'danger-hint'">
        {{ preview.message }}
      </p>
      <t-table
        row-key="source_key"
        :data="preview.items"
        :columns="previewColumns"
        size="small"
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="null"
      >
        <template #source_key="{ row }">
          <div class="stack-cell">
            <span class="cell-strong">{{ row.display_name }}</span>
            <span class="sub-text">{{ row.source_key }}</span>
          </div>
        </template>
        <template #action="{ row }">
          <t-tag :theme="policyActionTheme(row.action)" variant="light" size="small" shape="round">
            {{ policyActionLabel(row.action) }}
          </t-tag>
        </template>
        <template #total_rows="{ row }">
          <span class="cell-muted">{{ formatNumber(row.total_rows) }}</span>
        </template>
        <template #eligible_rows="{ row }">
          <span class="cell-strong">{{ formatNumber(row.eligible_rows) }}</span>
        </template>
        <template #guarded_rows="{ row }">
          <span :class="row.guarded_rows > 0 ? 'warn-hint' : 'cell-muted'">
            {{ row.guarded_rows > 0 ? formatNumber(row.guarded_rows) : '—' }}
          </span>
        </template>
        <template #earliest_time="{ row }">
          <span class="time-text">{{ formatTime(row.earliest_time) }}</span>
        </template>
        <template #latest_time="{ row }">
          <span class="time-text">{{ formatTime(row.latest_time) }}</span>
        </template>
        <template #before_time="{ row }">
          <span class="time-text">{{ formatTime(row.before_time) }}</span>
        </template>
        <template #estimated_bytes="{ row }">
          <span class="time-text">{{ formatBytes(row.estimated_bytes) }}</span>
        </template>
        <template #empty>
          <t-empty description="无满足条件的记录，无需清理" />
        </template>
      </t-table>
    </section>

    <section class="surface-card table-card">
      <t-tabs v-model="activeTab">
        <t-tab-panel value="jobs" label="清理任务" />
        <t-tab-panel value="files" label="导出文件" />
      </t-tabs>

      <!-- ---------- 清理任务 ---------- -->
      <div v-if="activeTab === 'jobs'" class="tab-body">
        <div class="table-card__head">
          <h3 class="card-title">清理任务</h3>
          <span class="table-card__meta">
            共 {{ formatNumber(jobsTotal) }} 个
            <template v-if="hasActiveJob"> · 有任务执行中，进度每 3 秒刷新</template>
          </span>
        </div>

        <t-table
          row-key="id"
          :data="jobs"
          :columns="jobColumns"
          :loading="jobsLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="isMobile ? undefined : jobsPagination"
          @page-change="handleJobsPageChange"
        >
          <template #job_no="{ row }">
            <span class="cell-strong">{{ row.job_no }}</span>
          </template>
          <template #trigger_type="{ row }">
            <t-tag variant="light" size="small" shape="round">{{ triggerLabel(row.trigger_type) }}</t-tag>
          </template>
          <template #status="{ row }">
            <div class="stack-cell">
              <t-tag :theme="cleanupStatusTheme(row.status)" variant="light" size="small" shape="round">
                {{ cleanupStatusLabel(row.status) }}
              </t-tag>
              <div v-if="cleanupStatusActive(row.status)" class="progress-cell">
                <t-progress :percentage="row.progress" :label="false" size="small" />
                <span class="sub-text">{{ row.current_source || '准备中' }} · {{ row.progress }}%</span>
              </div>
            </div>
          </template>
          <template #before_time="{ row }">
            <span class="time-text">{{ formatTime(row.before_time) }}</span>
          </template>
          <template #scanned_rows="{ row }">
            <span class="cell-muted">{{ formatNumber(row.scanned_rows) }}</span>
          </template>
          <template #deleted_rows="{ row }">
            <span class="cell-strong">{{ formatNumber(row.deleted_rows) }}</span>
          </template>
          <template #export_files="{ row }">
            <span :class="row.export_file_ids?.length ? 'cell-strong' : 'cell-muted'">
              {{ row.export_file_ids?.length || 0 }}
            </span>
          </template>
          <template #operator_name="{ row }">
            <span class="cell-muted">{{ row.operator_name || '系统调度' }}</span>
          </template>
          <template #started_at="{ row }">
            <span class="time-text">{{ formatTime(row.started_at) }}</span>
          </template>
          <template #duration="{ row }">
            <span class="time-text">{{ elapsedText(row) }}</span>
          </template>
          <template #action="{ row }">
            <div class="action-cell">
              <t-link theme="primary" hover="color" @click="openJobDetail(row)">详情</t-link>
              <t-link
                v-if="canCleanup && cleanupStatusActive(row.status)"
                theme="danger"
                hover="color"
                @click="handleCancel(row)"
              >
                取消
              </t-link>
            </div>
          </template>
          <template #empty>
            <t-empty description="暂无清理任务：调度到点后自动创建，也可手动执行" />
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
      </div>

      <!-- ---------- 导出文件 ---------- -->
      <div v-else class="tab-body">
        <FilterCard embedded>
          <div class="field">
            <span class="field__label">日志源</span>
            <t-select v-model="fileFilter.source" clearable placeholder="全部来源" :options="fileSourceOptions" />
          </div>
          <div class="field">
            <span class="field__label">文件状态</span>
            <t-select v-model="fileFilter.status" clearable placeholder="全部状态" :options="fileStatusOptions" />
          </div>
          <div class="field">
            <span class="field__label">导出时间区间</span>
            <t-date-range-picker
              v-model="fileRange"
              format="YYYY-MM-DD HH:mm:ss"
              enable-time-picker
              clearable
              allow-input
              @change="handleFileSearch"
            />
          </div>
          <template #actions>
            <t-space size="small">
              <t-button theme="primary" :loading="filesLoading" @click="handleFileSearch">
                <template #icon><SearchIcon aria-hidden="true" /></template>
                查询
              </t-button>
              <t-button variant="outline" @click="handleFileReset">重置</t-button>
            </t-space>
          </template>
        </FilterCard>

        <t-table
          row-key="id"
          :data="files"
          :columns="fileColumns"
          :loading="filesLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="isMobile ? undefined : filesPagination"
          @page-change="handleFilesPageChange"
        >
          <template #file_name="{ row }">
            <div class="stack-cell">
              <span class="cell-strong cell-ellipsis" :title="row.file_name">{{ row.file_name }}</span>
              <span class="sub-text">{{ sourceLabel(row.source_key) }} · {{ (row.format || '').toUpperCase() }}</span>
            </div>
          </template>
          <template #row_count="{ row }">
            <span class="cell-strong">{{ formatNumber(row.row_count) }}</span>
          </template>
          <template #file_size="{ row }">
            <span class="time-text">{{ formatBytes(row.file_size) }}</span>
          </template>
          <template #job_id="{ row }">
            <t-tag v-if="row.bound_job" theme="warning" variant="light" size="small" shape="round">
              备份凭据
            </t-tag>
            <span v-else class="cell-muted">人工导出</span>
          </template>
          <template #sha256="{ row }">
            <span class="cell-ellipsis cell-muted" :title="row.sha256">{{ shortHash(row.sha256) }}</span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="exportStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ exportStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #created_at="{ row }">
            <span class="time-text">{{ formatTime(row.created_at) }}</span>
          </template>
          <template #action="{ row }">
            <div class="action-cell">
              <t-link
                v-if="canExport && row.status !== 'deleted'"
                theme="primary"
                hover="color"
                @click="handleDownload(row)"
              >
                下载
              </t-link>
              <t-tooltip
                v-if="canCleanup && row.status !== 'deleted'"
                :content="row.bound_job ? '该文件是清理动作的备份凭据，不可删除' : ''"
                :disabled="!row.bound_job"
              >
                <t-link
                  :theme="row.bound_job ? 'default' : 'danger'"
                  :disabled="row.bound_job"
                  hover="color"
                  @click="handleDeleteFile(row)"
                >
                  删除
                </t-link>
              </t-tooltip>
            </div>
          </template>
          <template #empty>
            <t-empty description="暂无导出文件" />
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
          清理任务产生的备份文件标记为「备份凭据」并永久保留 —— 它是回答「某次清理删了哪些数据」的唯一凭据，
          因此后端拒绝删除（人工导出的文件可按保留期自动过期）。
        </p>
      </div>
    </section>

    <!-- 执行清理：高危操作，必须勾选 + 输入 DELETE -->
    <t-dialog
      v-model:visible="runVisible"
      header="执行日志清理"
      width="580px"
      :confirm-btn="{ content: '确认执行清理', theme: 'danger', disabled: !runConfirmable }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleRun"
    >
      <p class="danger-hint">
        该操作将 <b>永久删除</b> 水位线之前的日志行。删除前会先把候选行导出为 JSONL 备份、校验 sha256 并登记文件；
        导出失败则整个任务失败，<b>一行都不会删</b>。
      </p>

      <div class="confirm-field">
        <span class="field__label">水位线（删除记录时间早于该时刻的数据）</span>
        <t-date-picker
          v-model="runBeforeTime"
          format="YYYY-MM-DD HH:mm:ss"
          enable-time-picker
          allow-input
          clearable
          placeholder="留空则按各源保留期自动计算"
          style="width: 100%"
        />
      </div>

      <div class="confirm-field">
        <span class="field__label">参与源（仅保留策略为「清理」的源）</span>
        <t-select
          v-model="runSources"
          multiple
          clearable
          filterable
          placeholder="留空则清理全部可清理源"
          :options="cleanableOptions"
        />
      </div>

      <div v-if="!exportBeforeDelete" class="confirm-field">
        <p class="danger-hint">
          删除前导出已关闭（<code>log_export_before_delete=false</code>），后端会直接拒绝清理。
          请先在「系统配置」中开启后再执行。
        </p>
      </div>
      <p v-else class="warn-hint">
        备份文件将落盘到服务端存储目录的 <code>log-exports/</code> 下，并登记在「导出文件」列表中。
      </p>

      <div class="confirm-field">
        <t-checkbox v-model="runAck">我已确认导出备份会保留，且理解删除不可回滚</t-checkbox>
      </div>
      <div class="confirm-field">
        <span class="field__label">请输入 DELETE 以确认</span>
        <t-input v-model="runConfirmText" placeholder="DELETE" />
      </div>
    </t-dialog>

    <!-- 任务详情 -->
    <t-drawer
      v-model:visible="jobDetailVisible"
      header="清理任务详情"
      size="680px"
      :footer="false"
      :loading="jobDetailLoading"
    >
      <template v-if="jobDetail">
        <div class="detail-block">
          <div class="recon-row">
            <span class="recon-row__label">任务号</span>
            <span class="recon-row__value">{{ jobDetail.job.job_no }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">触发方式</span>
            <span class="recon-row__value">{{ triggerLabel(jobDetail.job.trigger_type) }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">状态</span>
            <span class="recon-row__value">
              <t-tag :theme="cleanupStatusTheme(jobDetail.job.status)" variant="light" size="small" shape="round">
                {{ cleanupStatusLabel(jobDetail.job.status) }}
              </t-tag>
            </span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">进度</span>
            <span class="recon-row__value progress-cell">
              <t-progress :percentage="jobDetail.job.progress" :label="true" size="small" />
            </span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">水位线</span>
            <span class="recon-row__value">{{ formatTime(jobDetail.job.before_time) }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">扫描 / 删除行数</span>
            <span class="recon-row__value">
              {{ formatNumber(jobDetail.job.scanned_rows) }} / {{ formatNumber(jobDetail.job.deleted_rows) }}
            </span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">操作人</span>
            <span class="recon-row__value">{{ jobDetail.job.operator_name || '系统调度' }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">开始 / 结束</span>
            <span class="recon-row__value">
              {{ formatTime(jobDetail.job.started_at) }} → {{ formatTime(jobDetail.job.finished_at) }}
            </span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">耗时</span>
            <span class="recon-row__value">{{ elapsedText(jobDetail.job) }}</span>
          </div>
        </div>

        <div v-if="jobDetail.job.error_message" class="detail-block">
          <h4 class="detail-block__title">失败原因</h4>
          <p class="danger-hint">{{ jobDetail.job.error_message }}</p>
        </div>

        <div class="detail-block">
          <h4 class="detail-block__title">参与源（按批次串行推进）</h4>
          <div class="source-progress">
            <div
              v-for="key in jobDetail.job.sources"
              :key="key"
              class="source-progress__item"
              :class="{
                'source-progress__item--done': isSourceDone(jobDetail.job, key),
                'source-progress__item--current': jobDetail.job.current_source === key,
              }"
            >
              <span class="cell-strong">{{ sourceLabel(key) }}</span>
              <span class="sub-text">{{ key }}</span>
            </div>
            <t-empty v-if="!jobDetail.job.sources?.length" description="该任务未记录参与源" />
          </div>
        </div>

        <div class="detail-block">
          <h4 class="detail-block__title">导出文件（删除动作的备份凭据）</h4>
          <t-table
            row-key="id"
            :data="jobDetail.export_files"
            :columns="jobFileColumns"
            size="small"
            table-layout="fixed"
            cell-empty-content="—"
            :pagination="null"
          >
            <template #file_name="{ row }">
              <span class="cell-strong cell-ellipsis" :title="row.file_name">{{ row.file_name }}</span>
            </template>
            <template #row_count="{ row }">
              <span class="cell-strong">{{ formatNumber(row.row_count) }}</span>
            </template>
            <template #file_size="{ row }">
              <span class="time-text">{{ formatBytes(row.file_size) }}</span>
            </template>
            <template #sha256="{ row }">
              <span class="cell-ellipsis cell-muted" :title="row.sha256">{{ shortHash(row.sha256) }}</span>
            </template>
            <template #action="{ row }">
              <t-link v-if="canExport" theme="primary" hover="color" @click="handleDownload(row)">下载</t-link>
            </template>
            <template #empty>
              <t-empty description="该任务未产生导出文件" />
            </template>
          </t-table>
        </div>

        <div v-if="canCleanup && cleanupStatusActive(jobDetail.job.status)" class="detail-block">
          <t-button theme="danger" variant="outline" @click="handleCancel(jobDetail.job)">取消该任务</t-button>
        </div>
      </template>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'

import { DeleteIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  cancelCleanupJob,
  deleteExportFile,
  downloadExportFile,
  getCleanupJob,
  getCleanupJobs,
  getExportFiles,
  previewCleanup,
  runCleanup,
  type LogCleanupJobDetailResponse,
  type LogCleanupJobInfo,
  type LogCleanupPreviewResponse,
  type LogExportFileInfo,
} from '@/api/logcenter'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import FilterCard from '@/components/filter-card/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import {
  cleanupStatusActive,
  cleanupStatusLabel,
  cleanupStatusTheme,
  exportStatusLabel,
  exportStatusTheme,
  formatBytes,
  formatNumber,
  formatRows,
  formatTime,
  policyActionLabel,
  policyActionTheme,
  triggerLabel,
} from '@/pages/system/logs/constants'
import { useUserStore } from '@/store'

defineOptions({ name: 'SystemLogsCleanup' })

const userStore = useUserStore()
const canCleanup = computed(() => userStore.permissions?.includes('log:cleanup') || userStore.isSuperAdmin)
const canExport = computed(() => userStore.permissions?.includes('log:export') || userStore.isSuperAdmin)
const { isMobile } = useIsMobile()

const activeTab = ref('jobs')

// ===== 源展示名：从预演结果与导出文件里累积，不必额外要目录权限 =====
const sourceNames = reactive<Record<string, string>>({})

function rememberSource(key?: string, name?: string) {
  if (key && name && !sourceNames[key]) sourceNames[key] = name
}

function sourceLabel(key?: string): string {
  if (!key) return '—'
  return sourceNames[key] || key
}

// ===========================================================================
// 预演
// ===========================================================================
const previewing = ref(false)
const preview = ref<LogCleanupPreviewResponse | null>(null)
const exportBeforeDelete = ref(true)

const previewColumns: PrimaryTableCol<Record<string, unknown>>[] = [
  { colKey: 'source_key', title: '日志源', minWidth: 190 },
  { colKey: 'action', title: '动作', width: 120 },
  { colKey: 'retention_days', title: '保留期(天)', width: 110, align: 'center' },
  { colKey: 'total_rows', title: '现有行数', width: 110, align: 'right' },
  { colKey: 'eligible_rows', title: '候选行数', width: 110, align: 'right' },
  { colKey: 'guarded_rows', title: '受保护跳过', width: 120, align: 'right' },
  { colKey: 'earliest_time', title: '最早时间', width: 170 },
  { colKey: 'latest_time', title: '最晚时间', width: 170 },
  { colKey: 'before_time', title: '水位线', width: 170 },
  { colKey: 'estimated_bytes', title: '预估大小', width: 110, align: 'right' },
]

async function handlePreview() {
  previewing.value = true
  try {
    const resp = await previewCleanup({})
    preview.value = resp
    exportBeforeDelete.value = resp.export_before_delete
    for (const item of resp.items || []) rememberSource(item.source_key, item.display_name)
    if (!resp.eligible_rows) {
      MessagePlugin.info('无满足条件的记录，无需清理')
    } else {
      MessagePlugin.success(`候选 ${formatRows(resp.eligible_rows)}`)
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '预演失败')
  } finally {
    previewing.value = false
  }
}

// ===========================================================================
// 清理任务列表
// ===========================================================================
const jobsLoading = ref(false)
const jobs = ref<LogCleanupJobInfo[]>([])
const jobsTotal = ref(0)
const jobsPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })
const filesMobilePage = reactive({ current: 1, pageSize: 10, total: 0 })
const activeMobile = computed(() => (activeTab.value === 'files' ? filesMobilePage : mobilePage))
const hasActiveJob = ref(false)
let pollTimer: ReturnType<typeof setInterval> | null = null

const jobColumns: PrimaryTableCol<LogCleanupJobInfo>[] = [
  { colKey: 'job_no', title: '任务号', width: 160 },
  { colKey: 'trigger_type', title: '触发方式', width: 90 },
  { colKey: 'status', title: '状态', minWidth: 190 },
  { colKey: 'before_time', title: '水位线', width: 170 },
  { colKey: 'scanned_rows', title: '扫描行数', width: 110, align: 'right' },
  { colKey: 'deleted_rows', title: '删除行数', width: 110, align: 'right' },
  { colKey: 'export_files', title: '导出文件', width: 100, align: 'right' },
  { colKey: 'operator_name', title: '操作人', width: 110 },
  { colKey: 'started_at', title: '开始时间', width: 170 },
  { colKey: 'duration', title: '耗时', width: 100 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 130, fixed: 'right', align: 'center' },
]

async function loadJobs() {
  jobsLoading.value = true
  try {
    const data = await getCleanupJobs({
      page: jobsPagination.current,
      page_size: jobsPagination.pageSize,
    })
    jobs.value = data.items || []
    jobsTotal.value = data.total || 0
    jobsPagination.total = jobsTotal.value
    mobilePage.current = jobsPagination.current
    mobilePage.pageSize = jobsPagination.pageSize
    mobilePage.total = jobsTotal.value
    hasActiveJob.value = jobs.value.some((item) => cleanupStatusActive(item.status))
    syncPolling()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载清理任务失败')
  } finally {
    jobsLoading.value = false
  }
}

function handleJobsPageChange(pageInfo: PageInfo) {
  jobsPagination.current = pageInfo.current
  jobsPagination.pageSize = pageInfo.pageSize
  loadJobs()
}

/** 有非终态任务时每 3 秒刷新进度（doc92 §8.3）；终态后自动停表 */
function syncPolling() {
  if (hasActiveJob.value && !pollTimer) {
    pollTimer = setInterval(loadJobs, 3000)
  } else if (!hasActiveJob.value && pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

/** 耗时 = 结束 - 开始；执行中则以当前时间估算 */
function elapsedText(row: LogCleanupJobInfo): string {
  if (!row.started_at) return '—'
  const started = new Date(row.started_at.replace(' ', 'T')).getTime()
  if (Number.isNaN(started)) return '—'
  const end = row.finished_at ? new Date(row.finished_at.replace(' ', 'T')).getTime() : Date.now()
  if (Number.isNaN(end) || end < started) return '—'
  const seconds = Math.round((end - started) / 1000)
  if (seconds < 60) return `${seconds} 秒`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分 ${seconds % 60} 秒`
  return `${Math.floor(minutes / 60)} 时 ${minutes % 60} 分`
}

/** 参与源里已推进过去的那些（用于抽屉里的串行进度） */
function isSourceDone(job: LogCleanupJobInfo, key: string): boolean {
  if (job.status === 'done') return true
  const current = job.current_source
  const order = job.sources || []
  if (!current) return false
  if (current === key) return false
  return order.indexOf(key) >= 0 && order.indexOf(key) < order.indexOf(current)
}

// ===========================================================================
// 执行清理
// ===========================================================================
const runVisible = ref(false)
const runBeforeTime = ref('')
const runSources = ref<string[]>([])
const runAck = ref(false)
const runConfirmText = ref('')

/** 可清理源选项来自预演结果（只有 action=clean 的源才出现在 items 里） */
const cleanableOptions = computed(() =>
  (preview.value?.items || []).map((item) => ({ label: item.display_name, value: item.source_key })),
)

const runConfirmable = computed(
  () => runAck.value && runConfirmText.value.trim().toUpperCase() === 'DELETE' && exportBeforeDelete.value,
)

function openRun() {
  if (!exportBeforeDelete.value) {
    MessagePlugin.warning('删除前导出已关闭（log_export_before_delete=false），为安全起见禁止清理')
    return
  }
  // 预演是执行的前置动作：先算清候选，避免盲删
  if (!preview.value) handlePreview()
  runBeforeTime.value = ''
  runSources.value = []
  runAck.value = false
  runConfirmText.value = ''
  runVisible.value = true
}

async function handleRun() {
  try {
    const info = await runCleanup({
      sources: runSources.value.length ? runSources.value : undefined,
      before_time: runBeforeTime.value || undefined,
      confirm_text: runConfirmText.value.trim(),
    })
    MessagePlugin.success(`清理任务已创建：${info.job_no}`)
    runVisible.value = false
    activeTab.value = 'jobs'
    jobsPagination.current = 1
    await loadJobs()
    handlePreview()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建清理任务失败')
  }
}

function handleCancel(job: LogCleanupJobInfo) {
  const dialog = DialogPlugin.confirm({
    header: '取消清理任务',
    body: `确认取消任务 ${job.job_no} 吗？已删除的批次不会回滚，但备份文件保留、可回灌。`,
    confirmBtn: { content: '确认取消', theme: 'danger' },
    cancelBtn: { content: '返回' },
    onConfirm: async () => {
      try {
        await cancelCleanupJob(job.id)
        MessagePlugin.success('已取消')
        dialog.destroy()
        await loadJobs()
        if (jobDetailVisible.value) loadJobDetail(job.id)
      } catch (error) {
        MessagePlugin.error((error as Error).message || '取消失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

// ===========================================================================
// 任务详情
// ===========================================================================
const jobDetailVisible = ref(false)
const jobDetailLoading = ref(false)
const jobDetail = ref<LogCleanupJobDetailResponse | null>(null)

const jobFileColumns: PrimaryTableCol<LogExportFileInfo>[] = [
  { colKey: 'file_name', title: '文件名', minWidth: 200 },
  { colKey: 'row_count', title: '行数', width: 100, align: 'right' },
  { colKey: 'file_size', title: '大小', width: 100, align: 'right' },
  { colKey: 'sha256', title: 'sha256', width: 150 },
  { colKey: 'action', title: '操作', width: 80, align: 'center' },
]

async function openJobDetail(row: LogCleanupJobInfo) {
  jobDetailVisible.value = true
  await loadJobDetail(row.id)
}

async function loadJobDetail(id: number) {
  jobDetailLoading.value = true
  try {
    const data = await getCleanupJob(id)
    jobDetail.value = data
    for (const file of data.export_files || []) rememberSource(file.source_key, file.display_name)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载任务详情失败')
  } finally {
    jobDetailLoading.value = false
  }
}

// ===========================================================================
// 导出文件
// ===========================================================================
const filesLoading = ref(false)
const files = ref<LogExportFileInfo[]>([])
const filesPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const fileRange = ref<string[] | null>(null)
const fileFilter = reactive<{ source?: string; status?: string }>({})
const fileSourceOptions = ref<Array<{ label: string; value: string }>>([])

const fileStatusOptions = [
  { value: 'success', label: '可用' },
  { value: 'failed', label: '失败' },
  { value: 'deleted', label: '已删除' },
]

const fileColumns: PrimaryTableCol<LogExportFileInfo>[] = [
  { colKey: 'file_name', title: '文件', minWidth: 240 },
  { colKey: 'row_count', title: '行数', width: 100, align: 'right' },
  { colKey: 'file_size', title: '大小', width: 100, align: 'right' },
  { colKey: 'job_id', title: '来源', width: 110 },
  { colKey: 'sha256', title: 'sha256', width: 150 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'created_at', title: '导出时间', width: 170 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 130, fixed: 'right', align: 'center' },
]

async function loadFiles() {
  filesLoading.value = true
  try {
    const data = await getExportFiles({
      source: fileFilter.source || undefined,
      status: fileFilter.status || undefined,
      start_time: fileRange.value?.[0] || undefined,
      end_time: fileRange.value?.[1] || undefined,
      page: filesPagination.current,
      page_size: filesPagination.pageSize,
    })
    files.value = data.items || []
    filesPagination.total = data.total || 0
    filesMobilePage.current = filesPagination.current
    filesMobilePage.pageSize = filesPagination.pageSize
    filesMobilePage.total = filesPagination.total
    for (const item of files.value) rememberSource(item.source_key, item.display_name)
    const seen = new Map(fileSourceOptions.value.map((opt) => [opt.value, opt.label]))
    for (const [key, name] of Object.entries(sourceNames)) seen.set(key, name)
    fileSourceOptions.value = Array.from(seen, ([value, label]) => ({ value, label }))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载导出文件失败')
  } finally {
    filesLoading.value = false
  }
}

function handleFileSearch() {
  filesPagination.current = 1
  loadFiles()
}

function handleFileReset() {
  fileFilter.source = undefined
  fileFilter.status = undefined
  fileRange.value = null
  handleFileSearch()
}

function handleFilesPageChange(pageInfo: PageInfo) {
  filesPagination.current = pageInfo.current
  filesPagination.pageSize = pageInfo.pageSize
  loadFiles()
}

async function handleDownload(row: LogExportFileInfo) {
  try {
    await downloadExportFile(row.id, row.file_name)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '下载失败')
  }
}

function handleDeleteFile(row: LogExportFileInfo) {
  if (row.bound_job) {
    MessagePlugin.warning('该文件是清理动作的备份凭据，不可删除')
    return
  }
  const dialog = DialogPlugin.confirm({
    header: '删除导出文件',
    body: `确认删除 ${row.file_name} 吗？文件将从磁盘移除，登记行保留（状态置为已删除）。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteExportFile(row.id)
        MessagePlugin.success('已删除')
        dialog.destroy()
        loadFiles()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

function shortHash(hash?: string): string {
  if (!hash) return '—'
  return hash.length > 16 ? `${hash.slice(0, 8)}…${hash.slice(-6)}` : hash
}

// ===========================================================================
// 移动端翻页：与桌面端共用页码状态
// ===========================================================================
function goMobilePage(target: number) {
  const state = activeMobile.value
  const total = state.total
  const maxPage = Math.max(1, Math.ceil(total / state.pageSize))
  const clamped = Math.min(Math.max(target, 1), maxPage)
  if (clamped === state.current) return
  if (activeTab.value === 'files') handleFilesPageChange({ current: clamped, pageSize: state.pageSize } as PageInfo)
  else handleJobsPageChange({ current: clamped, pageSize: state.pageSize } as PageInfo)
}

function handleMobilePageSizeChange(pageSize: number) {
  if (activeTab.value === 'files') handleFilesPageChange({ current: 1, pageSize } as PageInfo)
  else handleJobsPageChange({ current: 1, pageSize } as PageInfo)
}

// ===========================================================================
// 生命周期
// ===========================================================================
watch(activeTab, (tab) => {
  if (tab === 'files') loadFiles()
})

function reload() {
  if (activeTab.value === 'files') loadFiles()
  else loadJobs()
}

onMounted(loadJobs)
onUnmounted(stopPolling)
</script>

<style scoped>
.tab-body {
  padding-top: var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

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

.progress-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.source-progress {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.source-progress__item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 10px;
  border-radius: 8px;
  border: 1px solid var(--log-border);
  font-size: 12px;
  min-width: 140px;
}

.source-progress__item--done {
  opacity: 0.55;
}

.source-progress__item--current {
  border-color: var(--td-brand-color);
  background: var(--td-brand-color-1);
}
</style>

<style lang="css">
@import '../shared.css';
</style>
