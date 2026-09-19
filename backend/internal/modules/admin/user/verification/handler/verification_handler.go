package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/user/verification/dto"
	"hostsent/backend/internal/modules/admin/user/verification/repository"
	"hostsent/backend/internal/modules/admin/user/verification/service"
	"hostsent/backend/internal/pkg/middleware"
)

// VerificationHandler 处理实名认证的管理端请求（列表 / 详情 / 整单审核 / 配置 / 服务商）。
type VerificationHandler struct{ service service.VerificationService }

// NewVerificationHandler 创建实名认证处理器。
func NewVerificationHandler(service service.VerificationService) *VerificationHandler {
	return &VerificationHandler{service: service}
}

// ---------- 列表与详情 ----------

// ListPending godoc
// @Summary 实名待审核列表
// @Description 获取实名认证待审核列表
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param verification_type query string false "认证类型"
// @Param reviewer_name query string false "审核人"
// @Param keyword query string false "关键词"
// @Param start_time query string false "开始时间(RFC3339)"
// @Param end_time query string false "结束时间(RFC3339)"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.VerificationInfo]]
// @Router /api/v1/admin/verifications/pending [get]
func (h *VerificationHandler) ListPending(c *gin.Context) {
	h.list(c, h.service.ListPending)
}

// ListApproved godoc
// @Summary 实名审核通过列表
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.VerificationInfo]]
// @Router /api/v1/admin/verifications/approved [get]
func (h *VerificationHandler) ListApproved(c *gin.Context) {
	h.list(c, h.service.ListApproved)
}

// ListRejected godoc
// @Summary 实名审核拒绝列表
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.VerificationInfo]]
// @Router /api/v1/admin/verifications/rejected [get]
func (h *VerificationHandler) ListRejected(c *gin.Context) {
	h.list(c, h.service.ListRejected)
}

// list 三个列表接口的公共骨架：绑定查询参数 → 调服务 → 统一响应。
func (h *VerificationHandler) list(c *gin.Context, fn func(context.Context, dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error)) {
	var query dto.VerificationListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := fn(c.Request.Context(), query)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Detail godoc
// @Summary 实名申请详情
// @Description 返回主记录 + 附件 + 企业信息 + 审核轨迹
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Success 200 {object} dto.APIResponse[dto.VerificationDetail]
// @Router /api/v1/admin/verifications/{id} [get]
func (h *VerificationHandler) Detail(c *gin.Context) {
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	data, err := h.service.Detail(c.Request.Context(), id)
	if err != nil {
		respondVerificationErr(c, err)
		return
	}
	success(c, data)
}

// DownloadDocument godoc
// @Summary 下载实名认证材料（审核员）
// @Description 审核员可下载任意申请下的材料；用户端走 /uc/verification/documents/{id}/download
// @Tags 实名认证
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path int true "材料ID"
// @Success 200 {file} file
// @Router /api/v1/admin/verifications/documents/{id}/download [get]
func (h *VerificationHandler) DownloadDocument(c *gin.Context) {
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	doc, reader, err := h.service.OpenDocument(c.Request.Context(), id, true, 0)
	if err != nil {
		respondVerificationErr(c, err)
		return
	}
	defer reader.Close()
	// 与用户端同一处理：落盘随机名不外露，Content-Type 由扩展名白名单推导，
	// 未知类型按二进制流下发（绝不内联渲染上传内容）。
	fileName := service.DocumentFileName(doc.DocumentType, doc.FileURL)
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(fileName))
	c.DataFromReader(http.StatusOK, -1, service.DocumentContentType(doc.FileURL), reader, nil)
}

// ReviewLogs godoc
// @Summary 实名申请审核轨迹
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Success 200 {object} dto.APIResponse[[]dto.ReviewLogInfo]
// @Router /api/v1/admin/verifications/{id}/logs [get]
func (h *VerificationHandler) ReviewLogs(c *gin.Context) {
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	data, err := h.service.Detail(c.Request.Context(), id)
	if err != nil {
		respondVerificationErr(c, err)
		return
	}
	success(c, data.Logs)
}

// ---------- 整单审核 ----------

// Approve godoc
// @Summary 整单通过实名认证
// @Description 状态 pending → approved，写 users.real_name_verified_at 与审核日志
// @Tags 实名认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Param request body dto.ReviewRequest true "审核参数"
// @Success 200 {object} dto.APIResponse[dto.VerificationInfo]
// @Router /api/v1/admin/verifications/{id}/approve [post]
func (h *VerificationHandler) Approve(c *gin.Context) {
	h.review(c, h.service.Approve)
}

// Reject godoc
// @Summary 整单驳回实名认证
// @Description 状态 pending → rejected；驳回理由必填
// @Tags 实名认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Param request body dto.ReviewRequest true "审核参数"
// @Success 200 {object} dto.APIResponse[dto.VerificationInfo]
// @Router /api/v1/admin/verifications/{id}/reject [post]
func (h *VerificationHandler) Reject(c *gin.Context) {
	h.review(c, h.service.Reject)
}

// Revoke godoc
// @Summary 撤销已通过的实名认证
// @Description 清空 users.real_name_verified_at，申请置为 rejected（action=revoke）
// @Tags 实名认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Param request body dto.ReviewRequest true "撤销参数"
// @Success 200 {object} dto.APIResponse[dto.VerificationInfo]
// @Router /api/v1/admin/verifications/{id}/revoke [post]
func (h *VerificationHandler) Revoke(c *gin.Context) {
	h.review(c, h.service.Revoke)
}

// review 三个审核动作的公共骨架：取操作人 → 绑定参数 → 调服务。
//
// 空 body 是合法调用（通过/撤销可以不带备注），因此绑定失败只在有 body 时才算错误。
func (h *VerificationHandler) review(c *gin.Context, fn func(context.Context, uint64, dto.ReviewRequest, uint64, string) (*dto.VerificationInfo, error)) {
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	var req dto.ReviewRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequest(c, err.Error())
			return
		}
	}
	op, name, ok := verificationOperator(c)
	if !ok {
		return
	}
	data, err := fn(c.Request.Context(), id, req, op, name)
	if err != nil {
		respondVerificationErr(c, err)
		return
	}
	success(c, data)
}

// ProviderCheck godoc
// @Summary 手动触发三方核验
// @Description 仅对实现了无跳转核验的 provider 有效；支付宝属跳转式，会返回 ok=false 与原因
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Success 200 {object} dto.APIResponse[dto.ProviderCheckResult]
// @Router /api/v1/admin/verifications/{id}/provider-check [post]
func (h *VerificationHandler) ProviderCheck(c *gin.Context) {
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	op, name, ok := verificationOperator(c)
	if !ok {
		return
	}
	data, err := h.service.ProviderCheck(c.Request.Context(), id, op, name)
	if err != nil {
		respondVerificationErr(c, err)
		return
	}
	success(c, data)
}

// ---------- 配置 ----------

// ListConfigs godoc
// @Summary 实名配置列表
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[[]dto.VerificationConfigInfo]
// @Router /api/v1/admin/verifications/configs [get]
func (h *VerificationHandler) ListConfigs(c *gin.Context) {
	var query dto.VerificationConfigListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.ListConfigs(c.Request.Context(), query)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// UpsertConfig godoc
// @Summary 新增或更新实名配置
// @Description 按 config_key 定位：不存在则创建，存在则更新
// @Tags 实名认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ConfigUpsertRequest true "配置参数"
// @Success 200 {object} dto.APIResponse[dto.VerificationConfigInfo]
// @Router /api/v1/admin/verifications/configs [put]
func (h *VerificationHandler) UpsertConfig(c *gin.Context) {
	var req dto.ConfigUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	op, _, ok := verificationOperator(c)
	if !ok {
		return
	}
	data, err := h.service.UpsertConfig(c.Request.Context(), req, op)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// DeleteConfig godoc
// @Summary 删除实名配置
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param id path int true "配置ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/verifications/configs/{id} [delete]
func (h *VerificationHandler) DeleteConfig(c *gin.Context) {
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteConfig(c.Request.Context(), id); err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, "ok")
}

// ---------- 服务商 ----------

// ProviderTypes godoc
// @Summary 实名核验服务商类型
// @Description 描述符驱动前端动态凭证表单
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[[]dto.ProviderTypeInfo]
// @Router /api/v1/admin/verifications/provider-types [get]
func (h *VerificationHandler) ProviderTypes(c *gin.Context) {
	success(c, h.service.ProviderTypes())
}

// ListProviders godoc
// @Summary 实名核验服务商列表
// @Description 凭证只回脱敏值
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[[]dto.ProviderInfo]
// @Router /api/v1/admin/verifications/providers [get]
func (h *VerificationHandler) ListProviders(c *gin.Context) {
	data, err := h.service.ListProviders(c.Request.Context())
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// UpsertProvider godoc
// @Summary 新增或更新实名核验服务商
// @Description id=0 新建；id>0 更新。凭证回传脱敏值表示未修改
// @Tags 实名认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ProviderUpsertRequest true "服务商参数"
// @Success 200 {object} dto.APIResponse[dto.ProviderInfo]
// @Router /api/v1/admin/verifications/providers [put]
func (h *VerificationHandler) UpsertProvider(c *gin.Context) {
	var req dto.ProviderUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.UpsertProvider(c.Request.Context(), req)
	if err != nil {
		respondVerificationErr(c, err)
		return
	}
	success(c, data)
}

// DeleteProvider godoc
// @Summary 删除实名核验服务商
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param id path int true "服务商ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/verifications/providers/{id} [delete]
func (h *VerificationHandler) DeleteProvider(c *gin.Context) {
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteProvider(c.Request.Context(), id); err != nil {
		respondVerificationErr(c, err)
		return
	}
	success(c, "ok")
}

// TestProvider godoc
// @Summary 实名核验服务商连通性测试
// @Description 失败时 HTTP 仍为 200，由 ok/message 表达结果（对齐支付/验证码渠道口径）
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param id path int true "服务商ID"
// @Success 200 {object} dto.APIResponse[map[string]any]
// @Router /api/v1/admin/verifications/providers/{id}/test [post]
func (h *VerificationHandler) TestProvider(c *gin.Context) {
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	if err := h.service.TestProvider(c.Request.Context(), id); err != nil {
		// 与支付/验证码渠道一致：测试失败是业务结果，HTTP 200 + ok=false。
		success(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	success(c, gin.H{"ok": true, "message": "连通性正常"})
}

// ---------- 工具 ----------

// verificationOperator 取当前管理员 ID 与显示名。
//
// 审核是高危不可逆动作（通过即写入实名信任信号），拿不到操作人就不放行 ——
// 这正是 doc104 §3.1 F5 要修掉的静默兜底（operatorID=1）。
func verificationOperator(c *gin.Context) (uint64, string, bool) {
	claims, ok := middleware.GetAdminClaims(c)
	if !ok || claims.AdminID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"code": 40301, "message": "无法识别当前操作人", "timestamp": time.Now().Unix()})
		return 0, "", false
	}
	name := claims.Username
	if name == "" {
		name = "admin"
	}
	return claims.AdminID, name, true
}

func parseVerificationID(c *gin.Context) (uint64, bool) {
	var id uint64
	for _, ch := range c.Param("id") {
		if ch < '0' || ch > '9' {
			badRequest(c, "ID 不合法")
			return 0, false
		}
		id = id*10 + uint64(ch-'0')
	}
	if id == 0 {
		badRequest(c, "ID 不合法")
		return 0, false
	}
	return id, true
}

// respondVerificationErr 实名模块错误的 HTTP 映射。
func respondVerificationErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrApplicationNotFound), errors.Is(err, repository.ErrProviderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrStatusConflict), errors.Is(err, service.ErrPendingExists),
		errors.Is(err, service.ErrAlreadyVerified), errors.Is(err, service.ErrCooldown),
		errors.Is(err, service.ErrConfigNotAllowed), errors.Is(err, service.ErrTypeNotAllowed):
		c.JSON(http.StatusConflict, gin.H{"code": 40901, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrOperatorRequired):
		c.JSON(http.StatusForbidden, gin.H{"code": 40301, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrRejectReasonRequired):
		// 参数问题不是服务端故障：驳回未填理由必须是 400，
		// 落进 500 会让监控把它算成真实故障，前端也只能靠 message 文本判断。
		badRequest(c, err.Error())
	default:
		serverError(c, err.Error())
	}
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": message, "timestamp": time.Now().Unix()})
}

func serverError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": message, "timestamp": time.Now().Unix()})
}

func success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}
