<template>
  <div class="page-body log-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <FileIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">日志中心</h2>
          <p class="page-header__desc">
            26 类日志的统一浏览入口。列与筛选项由后端目录下发，<b>脱敏列在页面上始终打码</b>，只有导出文件里是明文。
            业务视图请前往
            <router-link class="header-link" to="/users/security/login-logs">登录日志</router-link>、
            <router-link class="header-link" to="/system/audit-logs">操作审计</router-link>、
            <router-link class="header-link" to="/resource/sync-center">同步中心</router-link>、
            <router-link class="header-link" to="/payment/callbacks">支付回调</router-link>。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button v-if="canExport" theme="primary" :disabled="!activeSource" @click="openExport">
          <template #icon><DownloadIcon aria-hidden="true" /></template>
          导出
        </t-button>
        <t-button variant="outline" :loading="catalogLoading || loading" @click="reloadAll">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <div class="browser-layout">
      <!-- 左：源列表（按 group 分组） -->
      <aside class="source-panel surface-card">
        <div v-for="group in groups" :key="group.key">
          <div class="source-group__title">{{ group.label }}</div>
          <div
            v-for="src in sourcesOf(group.key)"
            :key="src.key"
            class="source-item"
            :class="{ 'source-item--active': src.key === activeKey }"
            @click="selectSource(src.key)"
          >
            <span class="cell-ellipsis" :title="src.display_name">{{ src.display_name }}</span>
            <span class="source-item__badge">{{ shortNumber(stats.rows_by_source?.[src.key]) }}</span>
          </div>
        </div>
        <t-empty v-if="!catalog.items.length" description="暂无日志源" />
      </aside>

      <!-- 右：查询与结果 -->
      <div class="browser-main">
        <section v-if="activeSource" class="filter-card surface-card">
          <div class="filter-card__head">
            <div class="source-desc">
              <span class="card-title">{{ activeSource.display_name }}</span>
              <t-tag :theme="classTheme(activeSource.class)" variant="light" size="small" shape="round">
                {{ classLabel(activeSource.class) }}
              </t-tag>
              <span>{{ formatRows(activeSource.rows) }}</span>
              <span v-if="activeSource.remark">· {{ activeSource.remark }}</span>
              <span v-if="activeSource.independent_page" class="warn-hint">
                · 该源另有独立业务页面查看
              </span>
            </div>
          </div>
          <div class="filter-card__grid">
            <div class="field">
              <span class="field__label">时间区间</span>
              <t-date-range-picker v-model="dateRange" clearable allow-input @change="handleSearch" />
            </div>
            <div v-if="activeSource.searchable" class="field">
              <span class="field__label">关键词</span>
              <t-input v-model="keyword" placeholder="模糊匹配可搜索列" clearable @enter="handleSearch" />
            </div>
            <div v-for="filter in filterFields" :key="filter.param" class="field">
              <span class="field__label">{{ filter.label }}</span>
              <t-input
                v-model="filterValues[filter.param]"
                :placeholder="`按 ${filter.label} 精确筛选`"
                clearable
                @enter="handleSearch"
              />
            </div>
          </div>
          <div class="filter-card__actions">
            <t-space size="small">
              <t-button theme="primary" :loading="loading" @click="handleSearch">
                <template #icon><SearchIcon aria-hidden="true" /></template>
                查询
              </t-button>
              <t-button variant="outline" @click="handleReset">重置</t-button>
            </t-space>
          </div>
        </section>

        <section class="table-card surface-card">
          <div class="table-card__head">
            <h3 class="card-title">{{ activeSource ? `${activeSource.display_name}记录` : '请选择日志源' }}</h3>
            <span class="table-card__meta">共 {{ formatNumber(total) }} 条</span>
          </div>

          <t-table
            v-if="activeSource"
            row-key="id"
            :data="rows"
            :columns="tableColumns"
            :loading="loading"
            size="small"
            hover
            table-layout="fixed"
            cell-empty-content="—"
            :pagination="isMobile ? undefined : pagination"
            @page-change="handlePageChange"
          >
            <!-- 动态列插槽：列定义来自后端 catalog，前端不写死任何列 -->
            <template v-for="col in visibleColumns" :key="col.key" #[col.key]="{ row }">
              <span v-if="col.masked" class="cell-muted" :title="String(row[col.key] ?? '')">
                {{ maskValue(row[col.key]) }}
              </span>
              <t-tag
                v-else-if="col.type === 'bool'"
                :theme="row[col.key] ? 'success' : 'default'"
                variant="light"
                size="small"
                shape="round"
              >
                {{ row[col.key] ? '是' : '否' }}
              </t-tag>
              <t-tag
                v-else-if="col.type === 'enum'"
                variant="light"
                size="small"
                shape="round"
              >
                {{ enumLabel(col, row[col.key]) }}
              </t-tag>
              <span v-else-if="col.type === 'time'" class="time-text">{{ formatTime(String(row[col.key] ?? '')) }}</span>
              <span v-else-if="col.type === 'json'" class="cell-ellipsis cell-muted" :title="prettyJSON(row[col.key])">
                {{ compactJSON(row[col.key]) }}
              </span>
              <span v-else-if="col.type === 'number'" class="cell-strong">{{ formatNumber(Number(row[col.key] ?? 0)) }}</span>
              <span v-else class="cell-ellipsis" :title="String(row[col.key] ?? '')">{{ row[col.key] ?? '—' }}</span>
            </template>
            <template #action="{ row }">
              <div class="action-cell">
                <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              </div>
            </template>
            <template #empty>
              <t-empty :description="`该源在所选时间范围内无记录`" />
            </template>
          </t-table>

          <t-empty v-else description="请从左侧选择一个日志源" />

          <MobilePagination
            v-if="isMobile && activeSource"
            :current="mobilePage.current"
            :page-size="mobilePage.pageSize"
            :total="mobilePage.total"
            @go="goMobilePage"
            @page-size="handleMobilePageSizeChange"
          />
        </section>
      </div>
    </div>

    <!-- 详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="记录详情" size="640px" :footer="false" :loading="detailLoading">
      <template v-if="detail">
        <div class="detail-block">
          <div v-for="col in detailColumns" :key="col.key" class="recon-row">
            <span class="recon-row__label">{{ col.label }}</span>
            <span class="recon-row__value">
              <template v-if="col.masked">{{ maskValue(detail[col.key]) }}</template>
              <template v-else-if="col.type === 'time'">{{ formatTime(String(detail[col.key] ?? '')) }}</template>
              <template v-else-if="col.type === 'bool'">{{ detail[col.key] ? '是' : '否' }}</template>
              <template v-else-if="col.type === 'enum'">{{ enumLabel(col, detail[col.key]) }}</template>
              <template v-else-if="col.type === 'json'">{{ prettyJSON(detail[col.key]) }}</template>
              <template v-else>{{ detail[col.key] ?? '—' }}</template>
            </span>
          </div>
        </div>
      </template>
    </t-drawer>

    <!-- 导出对话框 -->
    <t-dialog
      v-model:visible="exportVisible"
      header="导出日志"
      width="480px"
      :confirm-btn="{ content: '开始导出', loading: exporting }"
      @confirm="handleExport"
    >
      <p class="warn-hint">
        导出文件落盘并留痕（谁、何时、导了哪个源、多少行）。文件内含脱敏字段的<b>明文</b>，请谨慎分发。
      </p>
      <div class="confirm-field">
        <span class="field__label">导出格式</span>
        <t-radio-group v-model="exportFormat" variant="default-filled">
          <t-radio-button value="csv">CSV（Excel 友好）</t-radio-button>
          <t-radio-button value="jsonl">JSONL（可回灌）</t-radio-button>
        </t-radio-group>
      </div>
      <div class="confirm-field">
        <span class="field__label">时间区间</span>
        <t-date-range-picker v-model="exportRange" clearable allow-input />
      </div>
      <p class="warn-hint">导出完成后可在「清理任务」页的导出文件列表中下载。</p>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { DownloadIcon, FileIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  exportLogs,
  getLogCatalog,
  getLogDetail,
  getLogStats,
  queryLogs,
  type LogCatalogResponse,
  type LogColumnInfo,
  type LogSourceInfo,
  type LogStatsResponse,
} from '@/api/logcenter'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import { classLabel, classTheme, formatNumber, formatRows, formatTime, prettyJSON } from '@/pages/system/logs/constants'
import { useUserStore } from '@/store'

defineOptions({ name: 'SystemLogs' })

const router = useRouter()
const userStore = useUserStore()
const canExport = computed(() => userStore.permissions?.includes('log:export') || userStore.isSuperAdmin)
const { isMobile } = useIsMobile()

const catalogLoading = ref(false)
const catalog = reactive<LogCatalogResponse>({ groups: [], items: [], actions: [], classes: [] })
const stats = reactive<LogStatsResponse>({
  rows_by_group: {},
  rows_by_source: {},
  total_rows: 0,
  upstream_recorded: 0,
  upstream_dropped: 0,
  job_runs_24h: 0,
  job_failed_24h: 0,
  active_cleanup_job: 0,
  export_files: 0,
  export_total_size: 0,
})

const activeKey = ref('')
const activeSource = computed<LogSourceInfo | undefined>(() =>
  catalog.items.find((item) => item.key === activeKey.value),
)
const groups = computed(() => catalog.groups)

function sourcesOf(group: string): LogSourceInfo[] {
  return catalog.items.filter((item) => item.group === group)
}

/** 列元数据：卡片列用；隐藏列不进表格 */
const visibleColumns = computed<LogColumnInfo[]>(() =>
  (activeSource.value?.columns || []).filter((col) => !col.hidden),
)

const tableColumns = computed<PrimaryTableCol<Record<string, unknown>>[]>(() => {
  const cols: PrimaryTableCol<Record<string, unknown>>[] = visibleColumns.value.map((col) => ({
    colKey: col.key,
    title: col.label,
    width: col.width > 0 ? col.width : undefined,
    minWidth: col.width > 0 ? undefined : 140,
    ellipsis: col.type === 'string',
  }))
  cols.push({
    colKey: 'action',
    title: '操作',
    width: 80,
    fixed: 'right' as const,
    align: 'center' as const,
  })
  return cols
})

const detailColumns = computed<LogColumnInfo[]>(() => activeSource.value?.columns || [])

/** 源特有筛选：参数名 → 列标签（列标签从 catalog 的列定义里取） */
const filterFields = computed(() => {
  const src = activeSource.value
  if (!src) return []
  return Object.entries(src.filter_columns || {}).map(([param, column]) => ({
    param,
    label: src.columns.find((col) => col.key === column)?.label || column,
  }))
})

function selectSource(key: string) {
  if (activeKey.value === key) return
  activeKey.value = key
  resetFilters()
  loadRows()
}

// ===== 查询 =====
const loading = ref(false)
const rows = ref<Array<Record<string, unknown>>>([])
const total = ref(0)
const dateRange = ref<string[] | null>(defaultRange())
const keyword = ref('')

/** 默认近 7 天：不做时间收敛的全表查询在 26 个源上是不可控的（doc92 §8.2） */
function defaultRange(): string[] {
  const end = new Date()
  const start = new Date(end.getTime() - 7 * 24 * 60 * 60 * 1000)
  return [formatDate(start), formatDate(end)]
}

function formatDate(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  )
}
const filterValues = reactive<Record<string, string>>({})
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

async function loadRows() {
  const src = activeSource.value
  if (!src) return
  loading.value = true
  try {
    const params: Record<string, string | number | undefined> = {
      source: src.key,
      from: dateRange.value?.[0] || undefined,
      to: dateRange.value?.[1] || undefined,
      keyword: keyword.value || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    for (const [param, value] of Object.entries(filterValues)) {
      if (value) params[param] = value
    }
    const data = await queryLogs(params as never)
    rows.value = data.items || []
    total.value = data.total || 0
    pagination.total = total.value
    mobilePage.total = total.value
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载日志失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  mobilePage.current = 1
  loadRows()
}

function resetFilters() {
  dateRange.value = defaultRange()
  keyword.value = ''
  for (const key of Object.keys(filterValues)) delete filterValues[key]
  pagination.current = 1
  pagination.pageSize = 20
  mobilePage.current = 1
}

function handleReset() {
  resetFilters()
  loadRows()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadRows()
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

// ===== 详情 =====
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<Record<string, unknown> | null>(null)

async function openDetail(row: Record<string, unknown>) {
  const src = activeSource.value
  if (!src) return
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    const data = await getLogDetail(src.key, Number(row.id))
    detail.value = data.item || null
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载详情失败')
  } finally {
    detailLoading.value = false
  }
}

// ===== 导出 =====
const exportVisible = ref(false)
const exporting = ref(false)
const exportFormat = ref<'csv' | 'jsonl'>('csv')
const exportRange = ref<string[] | null>(null)

function openExport() {
  exportRange.value = dateRange.value ? [...dateRange.value] : null
  exportVisible.value = true
}

async function handleExport() {
  const src = activeSource.value
  if (!src) return
  exporting.value = true
  try {
    const filters: Record<string, string> = {}
    for (const [param, value] of Object.entries(filterValues)) {
      if (value) filters[param] = value
    }
    const resp = await exportLogs({
      source: src.key,
      format: exportFormat.value,
      from: exportRange.value?.[0] || undefined,
      to: exportRange.value?.[1] || undefined,
      keyword: keyword.value || undefined,
      filters: Object.keys(filters).length ? filters : undefined,
    })
    MessagePlugin.success(`已导出 ${formatRows(resp.row_count)}（${resp.file_name}）`)
    exportVisible.value = false
    router.push('/system/logs/cleanup')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '导出失败')
  } finally {
    exporting.value = false
  }
}

// ===== 渲染工具 =====
/** 脱敏列永远打码：列表与详情都不返回明文（明文只在导出文件里） */
function maskValue(value: unknown): string {
  if (value === null || value === undefined || value === '') return '—'
  const text = String(value)
  if (text.length <= 4) return '******'
  return `${text.slice(0, 2)}****${text.slice(-2)}`
}

function enumLabel(col: LogColumnInfo, value: unknown): string {
  const key = String(value ?? '')
  return col.enum_map?.[key] || key || '—'
}

function compactJSON(value: unknown): string {
  if (value === null || value === undefined || value === '') return '—'
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > 80 ? `${text.slice(0, 80)}…` : text
}

function shortNumber(value?: number): string {
  if (!value) return '0'
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 10_000) return `${(value / 10_000).toFixed(1)}万`
  return String(value)
}

// ===== 加载 =====
async function loadCatalog() {
  catalogLoading.value = true
  try {
    const [cat, st] = await Promise.all([getLogCatalog(), getLogStats()])
    catalog.groups = cat.groups || []
    catalog.items = cat.items || []
    catalog.actions = cat.actions || []
    catalog.classes = cat.classes || []
    stats.rows_by_group = st.rows_by_group || {}
    stats.rows_by_source = st.rows_by_source || {}
    stats.total_rows = st.total_rows || 0
    stats.upstream_recorded = st.upstream_recorded || 0
    stats.upstream_dropped = st.upstream_dropped || 0
    if (!activeKey.value && catalog.items.length) {
      activeKey.value = catalog.items[0].key
      loadRows()
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载日志目录失败')
  } finally {
    catalogLoading.value = false
  }
}

function reloadAll() {
  loadCatalog()
  loadRows()
}

onMounted(loadCatalog)
</script>

<style scoped>
.header-link {
  color: var(--td-brand-color);
  text-decoration: none;
}

.header-link:hover {
  text-decoration: underline;
}
</style>

<style lang="css">
@import './shared.css';
</style>
