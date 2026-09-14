// Package handler 日志中心的 HTTP 处理器（doc92 §9.2）。
//
// 路由注册顺序约束：固定段（catalog/stats/query/export/export-files/policies/
// cleanup/cleanup-jobs）必须注册在带通配段的详情路由之前，否则 Gin 会因通配冲突
// panic。
package handler

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/logcenter/dto"
	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
	logservice "hostsent/backend/internal/modules/admin/logcenter/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// DownloadAudit 一次导出文件下载的留痕信息。
//
// 下载是敏感操作（导出含 IP、手机号、上游请求体），必须能回答「谁在什么时候
// 下走了哪份文件」。审计中间件只覆盖写方法（POST/PUT/DELETE），GET 下载不在
// 其中，所以这里显式留痕（doc92 §5.3 / 验收场景 26）。
type DownloadAudit struct {
	OperatorID   uint64
	OperatorName string
	SourceKey    string
	FileName     string
	RowCount     int64
	FileID       uint64
	IP           string
	UserAgent    string
}

// DownloadAuditFunc 下载留痕落库（装配层注入，日志中心不依赖管理端审计模型）。
type DownloadAuditFunc func(ctx context.Context, entry DownloadAudit)

// Handler 日志中心处理器。
type Handler struct {
	query   logservice.QueryService
	export  logservice.ExportService
	cleanup logservice.CleanupService
	policy  logservice.PolicyService
	// exportBeforeDelete 读取 log_export_before_delete（页面对 false 时置灰执行按钮）。
	exportBeforeDelete func() bool
	// auditDownload 下载留痕（nil 时跳过，仅在未装配审计能力的部署里发生）。
	auditDownload DownloadAuditFunc
}

// HandlerDeps 处理器依赖。
type HandlerDeps struct {
	Query   logservice.QueryService
	Export  logservice.ExportService
	Cleanup logservice.CleanupService
	Policy  logservice.PolicyService
	// ExportBeforeDelete 读取 log_export_before_delete（nil 时恒为 true）。
	ExportBeforeDelete func() bool
	// AuditDownload 导出文件下载留痕（doc92 §5.3）。
	AuditDownload DownloadAuditFunc
}

// NewHandler 创建处理器。
func NewHandler(deps HandlerDeps) *Handler {
	fn := deps.ExportBeforeDelete
	if fn == nil {
		fn = func() bool { return true }
	}
	return &Handler{
		query: deps.Query, export: deps.Export, cleanup: deps.Cleanup,
		policy: deps.Policy, exportBeforeDelete: fn, auditDownload: deps.AuditDownload,
	}
}

// writeErr 统一错误映射：策略/参数类 → 20001，其余 → 50001。
func writeErr(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, logservice.ErrPolicyRejected) || errors.Is(err, logservice.ErrExportUnavailable) {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Error(c, apperrors.New(50001, err.Error()))
}

// ---------------------------------------------------------------------------
// 目录 / 统计 / 查询
// ---------------------------------------------------------------------------

// Catalog `GET /admin/logs/catalog`：源目录与列元数据（26 个源共用一套页面）。
func (h *Handler) Catalog(c *gin.Context) {
	resp, err := h.query.Catalog(c.Request.Context())
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, resp)
}

// Stats `GET /admin/logs/stats`。
func (h *Handler) Stats(c *gin.Context) {
	resp, err := h.query.Stats(c.Request.Context())
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, resp)
}

// Query `GET /admin/logs/query`。
//
// 源特有的等值筛选从任意 query 参数取，交给服务层按 FilterColumns 白名单过滤；
// 这里不做任何列名判断（列名只能来自代码常量，doc92 §0.3 硬约束 1）。
func (h *Handler) Query(c *gin.Context) {
	var req dto.QueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		response.Error(c, apperrors.New(20001, "source 不能为空"))
		return
	}
	filters := map[string]string{}
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			filters[key] = values[0]
		}
	}
	resp, err := h.query.QueryWithFilters(c.Request.Context(), req, filters)
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, resp)
}

// Detail `GET /admin/logs/:source/detail/:id`。
func (h *Handler) Detail(c *gin.Context) {
	source := strings.TrimSpace(c.Param("source"))
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(20001, "ID 不合法"))
		return
	}
	resp, err := h.query.Detail(c.Request.Context(), source, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	if resp == nil {
		response.Error(c, apperrors.New(40401, "记录不存在"))
		return
	}
	response.Success(c, resp)
}

// ---------------------------------------------------------------------------
// 导出
// ---------------------------------------------------------------------------

// Export `POST /admin/logs/export`：同步导出，返回 file_id。
//
// 导出即留痕：谁、什么时候、导了哪个源、多少行，全部落 log_export_files。
func (h *Handler) Export(c *gin.Context) {
	var req dto.ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.export.Export(c.Request.Context(), req, operatorID, operatorName)
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, resp)
}

// ListExportFiles `GET /admin/logs/export-files`。
func (h *Handler) ListExportFiles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	jobID, _ := strconv.ParseUint(c.Query("job_id"), 10, 64)
	resp, err := h.export.List(c.Request.Context(), logrepo.ExportListQuery{
		SourceKey: strings.TrimSpace(c.Query("source")),
		JobID:     jobID,
		Status:    strings.TrimSpace(c.Query("status")),
		StartTime: c.Query("start_time"),
		EndTime:   c.Query("end_time"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, resp)
}

// DownloadExportFile `GET /admin/logs/export-files/:id/download`：流式下载。
//
// 导出文件含 IP 与上游请求体，下载是敏感操作，路由上挂审计中间件留痕。
func (h *Handler) DownloadExportFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(20001, "ID 不合法"))
		return
	}
	info, err := h.export.Get(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	if info == nil {
		response.Error(c, apperrors.New(40401, "导出文件不存在"))
		return
	}
	if info.Status == logmodel.ExportStatusDeleted {
		response.Error(c, apperrors.New(20001, "该导出文件已删除"))
		return
	}
	file, err := h.export.OpenByID(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	defer file.Close()
	if h.auditDownload != nil {
		operatorID, operatorName := operatorFromContext(c)
		h.auditDownload(c.Request.Context(), DownloadAudit{
			OperatorID: operatorID, OperatorName: operatorName,
			SourceKey: info.SourceKey, FileName: info.FileName,
			RowCount: info.RowCount, FileID: info.ID,
			IP: c.ClientIP(), UserAgent: c.Request.UserAgent(),
		})
	}
	// 中文文件名用 RFC 5987 的 filename* 传 UTF-8。
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(info.FileName))
	c.DataFromReader(200, info.FileSize, "application/octet-stream", file, nil)
}

// DeleteExportFile `DELETE /admin/logs/export-files/:id`。
func (h *Handler) DeleteExportFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(20001, "ID 不合法"))
		return
	}
	if err := h.export.Delete(c.Request.Context(), id); err != nil {
		writeErr(c, err)
		return
	}
	response.SuccessMessage(c, "删除成功")
}

// ---------------------------------------------------------------------------
// 保留策略
// ---------------------------------------------------------------------------

// ListPolicies `GET /admin/logs/policies`。
func (h *Handler) ListPolicies(c *gin.Context) {
	items, err := h.policy.List(c.Request.Context())
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// UpdatePolicy `PUT /admin/logs/policies/:source`。
func (h *Handler) UpdatePolicy(c *gin.Context) {
	source := strings.TrimSpace(c.Param("source"))
	if source == "" {
		response.Error(c, apperrors.New(20001, "source 不能为空"))
		return
	}
	var req dto.PolicyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	item, err := h.policy.Update(c.Request.Context(), source, req, operatorID)
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, item)
}

// ---------------------------------------------------------------------------
// 清理
// ---------------------------------------------------------------------------

// PreviewCleanup `POST /admin/logs/cleanup/preview`：预演（不导出不删除）。
func (h *Handler) PreviewCleanup(c *gin.Context) {
	var req dto.CleanupPreviewRequest
	_ = c.ShouldBindJSON(&req)
	resp, err := h.cleanup.Preview(c.Request.Context(), req)
	if err != nil {
		writeErr(c, err)
		return
	}
	resp.ExportBeforeDelete = h.exportBeforeDelete()
	if !resp.ExportBeforeDelete {
		resp.Message = "删除前导出已关闭（log_export_before_delete=false），为安全起见禁止清理"
	}
	response.Success(c, resp)
}

// RunCleanup `POST /admin/logs/cleanup`：执行清理（需 DELETE 确认）。
func (h *Handler) RunCleanup(c *gin.Context) {
	var req dto.CleanupRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	info, err := h.cleanup.Start(c.Request.Context(), req, logmodel.TriggerManual,
		operatorID, operatorName, h.exportBeforeDelete())
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, info)
}

// ListCleanupJobs `GET /admin/logs/cleanup-jobs`。
func (h *Handler) ListCleanupJobs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := h.cleanup.List(c.Request.Context(), logservice.CleanupListQuery{
		Status:    strings.TrimSpace(c.Query("status")),
		Trigger:   strings.TrimSpace(c.Query("trigger_type")),
		StartTime: c.Query("start_time"),
		EndTime:   c.Query("end_time"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	response.Success(c, resp)
}

// GetCleanupJob `GET /admin/logs/cleanup-jobs/:id`。
func (h *Handler) GetCleanupJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(20001, "ID 不合法"))
		return
	}
	resp, err := h.cleanup.Detail(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	if resp == nil {
		response.Error(c, apperrors.New(40401, "清理任务不存在"))
		return
	}
	response.Success(c, resp)
}

// CancelCleanupJob `POST /admin/logs/cleanup-jobs/:id/cancel`。
func (h *Handler) CancelCleanupJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(20001, "ID 不合法"))
		return
	}
	if err := h.cleanup.Cancel(c.Request.Context(), id); err != nil {
		writeErr(c, err)
		return
	}
	response.SuccessMessage(c, "已取消")
}

// operatorFromContext 取管理员操作者。
func operatorFromContext(c *gin.Context) (uint64, string) {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID, claims.Username
	}
	return 0, ""
}
