// 日志中心（doc92）· 管理端 API
// 对应后端 internal/modules/admin/logcenter/handler，路由前缀 /api/v1/admin/logs。
//
// 26 个日志源共用一套页面：列定义与筛选项全部由 getLogCatalog 下发，
// 前端不写死任何列（这是本模块前端设计的核心）。
import { request } from '@/utils/request'

// ---------- 目录 / 列元数据 ----------

/** 列元数据：前端据此动态渲染表格列 */
export interface LogColumnInfo {
  key: string
  label: string
  /** 渲染类型：string / time / json / bool / enum / number */
  type: string
  width: number
  hidden: boolean
  /** 脱敏列：列表与详情都打码，只有导出文件里是明文 */
  masked: boolean
  /** type=enum 时的值→中文映射 */
  enum_map?: Record<string, string>
}

/** 日志源定义 */
export interface LogSourceInfo {
  key: string
  display_name: string
  group: string
  group_label: string
  /** ops 可清理 / audit 仅备份 */
  class: string
  cleanable: boolean
  time_column: string
  columns: LogColumnInfo[]
  /** 源特有筛选：参数名 → 数据库列名 */
  filter_columns: Record<string, string>
  searchable: boolean
  default_action: string
  default_retention_days: number
  /** 非空时该源另有独立业务页面 */
  independent_page: string
  remark: string
  rows: number
  has_index: boolean
}

export interface LogGroupInfo {
  key: string
  label: string
}

export interface LogEnumOption {
  value: string
  label: string
}

export interface LogCatalogResponse {
  groups: LogGroupInfo[]
  items: LogSourceInfo[]
  actions: LogEnumOption[]
  classes: LogEnumOption[]
}

// ---------- 统计 ----------

export interface LogStatsResponse {
  rows_by_group: Record<string, number>
  rows_by_source: Record<string, number>
  total_rows: number
  upstream_recorded: number
  upstream_dropped: number
  job_runs_24h: number
  job_failed_24h: number
  active_cleanup_job: number
  export_files: number
  export_total_size: number
}

// ---------- 查询 / 详情 ----------

export interface LogQueryParams {
  source: string
  from?: string
  to?: string
  keyword?: string
  page?: number
  page_size?: number
  /** 源特有筛选（键取 filter_columns 的参数名） */
  [key: string]: string | number | undefined
}

export interface LogQueryResponse {
  source: string
  items: Array<Record<string, unknown>>
  total: number
  page: number
  page_size: number
  from: string
  to: string
}

export interface LogDetailResponse {
  source: string
  item: Record<string, unknown>
}

// ---------- 导出 ----------

export interface LogExportRequest {
  source: string
  format?: 'csv' | 'jsonl'
  from?: string
  to?: string
  keyword?: string
  filters?: Record<string, string>
}

export interface LogExportResult {
  file_id: number
  file_name: string
  row_count: number
  file_size: number
  sha256: string
}

export interface LogExportFileInfo {
  id: number
  source_key: string
  display_name: string
  job_id: number
  format: string
  file_name: string
  file_size: number
  row_count: number
  sha256: string
  period_from: string
  period_to: string
  operator_id: number
  operator_name: string
  status: string
  fail_reason: string
  expires_at: string
  created_at: string
  /** true 时该文件是清理动作的备份凭据，不可删除 */
  bound_job: boolean
}

export interface LogExportFileListResponse {
  items: LogExportFileInfo[]
  total: number
  page: number
  page_size: number
}

// ---------- 清理 ----------

export interface LogCleanupPreviewRequest {
  sources?: string[]
  before_time?: string
}

export interface LogCleanupPreviewItem {
  source_key: string
  display_name: string
  action: string
  retention_days: number
  before_time: string
  total_rows: number
  eligible_rows: number
  guarded_rows: number
  earliest_time: string
  latest_time: string
  estimated_bytes: number
  enabled: boolean
}

export interface LogCleanupPreviewResponse {
  items: LogCleanupPreviewItem[]
  eligible_rows: number
  guarded_rows: number
  export_before_delete: boolean
  message: string
}

export interface LogCleanupRunRequest {
  sources?: string[]
  before_time?: string
  /** 必须为 DELETE（高危操作防误点） */
  confirm_text: string
}

export interface LogCleanupJobInfo {
  id: number
  job_no: string
  trigger_type: string
  status: string
  status_label: string
  /** 参与清理的源 key 列表（按批次推进顺序） */
  sources: string[]
  before_time: string
  dry_run: boolean
  export_required: boolean
  export_file_ids: number[]
  scanned_rows: number
  deleted_rows: number
  expired_rows: number
  progress: number
  current_source: string
  error_message: string
  operator_id: number
  operator_name: string
  started_at: string
  finished_at: string
  created_at: string
  /** true 时按 3 秒轮询刷新进度 */
  active: boolean
}

export interface LogCleanupJobListResponse {
  items: LogCleanupJobInfo[]
  total: number
  page: number
  page_size: number
}

export interface LogCleanupJobDetailResponse {
  job: LogCleanupJobInfo
  export_files: LogExportFileInfo[]
}

// ---------- 保留策略 ----------

export interface LogPolicyInfo {
  source_key: string
  display_name: string
  group: string
  group_label: string
  class: string
  cleanable: boolean
  action: string
  action_label: string
  retention_days: number
  archive_cron: string
  batch_size: number
  enabled: boolean
  last_run_at: string
  last_deleted: number
  remark: string
  independent_page: string
  default_retention_days: number
  time_column: string
  table: string
  delete_guard: string
  /** false 表示库里还没有策略行，展示的是注册表默认值 */
  configured: boolean
}

export interface LogPolicyUpdateRequest {
  action: string
  retention_days?: number
  batch_size?: number
  enabled?: boolean
  archive_cron?: string
  remark?: string
}

// ===========================================================================
// 接口
// ===========================================================================

// 源目录与列元数据（26 个源共用一套页面）。
export function getLogCatalog(): Promise<LogCatalogResponse> {
  return request.get<LogCatalogResponse>({ url: '/logs/catalog' })
}

// 各源行数与采集侧计数。
export function getLogStats(): Promise<LogStatsResponse> {
  return request.get<LogStatsResponse>({ url: '/logs/stats' })
}

// 分页查询某源。
export function queryLogs(params: LogQueryParams): Promise<LogQueryResponse> {
  return request.get<LogQueryResponse>({ url: '/logs/query', params })
}

// 单行详情（含被截断正文的完整内容）。
export function getLogDetail(source: string, id: number): Promise<LogDetailResponse> {
  return request.get<LogDetailResponse>({ url: `/logs/${source}/detail/${id}` })
}

// 导出日志（落盘并留痕，返回 file_id）。
export function exportLogs(data: LogExportRequest): Promise<LogExportResult> {
  return request.post<LogExportResult>({ url: '/logs/export', data })
}

// 导出文件列表。
export function getExportFiles(params: {
  source?: string
  job_id?: number
  status?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}): Promise<LogExportFileListResponse> {
  return request.get<LogExportFileListResponse>({ url: '/logs/export-files', params })
}

// 下载导出文件（带鉴权，走 blob 保存）。
//
// 后端对错误也返回 HTTP 200 + JSON 信封，必须先把错误包识别出来，
// 否则会把一段错误 JSON 当成日志文件保存到本地。
export function downloadExportFile(id: number, fileName = 'log-export'): Promise<void> {
  return request
    .get<Blob>({ url: `/logs/export-files/${id}/download`, responseType: 'blob', _skipResultUnwrap: true } as never)
    .then(async (response) => {
      const blob = (response as unknown as { data: Blob }).data
      if (blob.type && blob.type.includes('application/json')) {
        const text = await blob.text()
        let message = '日志导出文件下载失败'
        try {
          message = JSON.parse(text)?.message || message
        } catch {
          /* 保留默认文案 */
        }
        throw new Error(message)
      }
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = fileName
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
    })
}

// 删除导出文件（清理任务产生的备份会被后端拒绝）。
export function deleteExportFile(id: number): Promise<void> {
  return request.delete<void>({ url: `/logs/export-files/${id}` })
}

// 保留策略列表。
export function getPolicies(): Promise<{ items: LogPolicyInfo[] }> {
  return request.get<{ items: LogPolicyInfo[] }>({ url: '/logs/policies' })
}

// 更新某源保留策略。
export function updatePolicy(source: string, data: LogPolicyUpdateRequest): Promise<LogPolicyInfo> {
  return request.put<LogPolicyInfo>({ url: `/logs/policies/${source}`, data })
}

// 清理预演（不导出不删除）。
export function previewCleanup(data: LogCleanupPreviewRequest): Promise<LogCleanupPreviewResponse> {
  return request.post<LogCleanupPreviewResponse>({ url: '/logs/cleanup/preview', data })
}

// 执行清理（需 confirm_text=DELETE）。
export function runCleanup(data: LogCleanupRunRequest): Promise<LogCleanupJobInfo> {
  return request.post<LogCleanupJobInfo>({ url: '/logs/cleanup', data })
}

// 清理任务列表。
export function getCleanupJobs(params: {
  status?: string
  trigger_type?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}): Promise<LogCleanupJobListResponse> {
  return request.get<LogCleanupJobListResponse>({ url: '/logs/cleanup-jobs', params })
}

// 清理任务详情（含逐源进度与导出文件）。
export function getCleanupJob(id: number): Promise<LogCleanupJobDetailResponse> {
  return request.get<LogCleanupJobDetailResponse>({ url: `/logs/cleanup-jobs/${id}` })
}

// 取消清理任务（仅非终态可取消）。
export function cancelCleanupJob(id: number): Promise<void> {
  return request.post<void>({ url: `/logs/cleanup-jobs/${id}/cancel` })
}
