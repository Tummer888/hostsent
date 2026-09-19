package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/user/verification/dto"
	"hostsent/backend/internal/modules/admin/user/verification/service"
	"hostsent/backend/internal/pkg/middleware"
)

// VerificationUserHandler 处理用户端的实名认证请求（doc104 §5.7）。
//
// 与 VerificationHandler 分开：用户端只有「自己」这一个主体，取用户 ID 的方式、
// 错误码口径（越权一律 404 不泄露存在性）与管理端不同，混在一个 handler 里
// 很容易在某个分支上漏掉归属校验。
type VerificationUserHandler struct{ service service.VerificationService }

// NewVerificationUserHandler 创建用户端实名处理器。
func NewVerificationUserHandler(service service.VerificationService) *VerificationUserHandler {
	return &VerificationUserHandler{service: service}
}

// userID 取当前登录用户 ID（UserAuth 中间件已保证存在）。
func userID(c *gin.Context) (uint64, bool) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok || claims.UserID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 10001, "message": "unauthorized", "timestamp": time.Now().Unix()})
		return 0, false
	}
	return claims.UserID, true
}

// Status godoc
// @Summary 我的实名状态
// @Description 未提交 / 待审核 / 已通过 / 已驳回 + 驳回理由 + 冷却期剩余秒数
// @Tags 用户中心-实名认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[dto.UserStatusResponse]
// @Router /api/v1/uc/verification [get]
func (h *VerificationUserHandler) Status(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	data, err := h.service.UserStatus(c.Request.Context(), uid)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Submit godoc
// @Summary 提交实名认证申请
// @Description 证件号加密落库并返回脱敏；策略要求时需带 captcha_key/captcha_code 与 verify_ticket
// @Tags 用户中心-实名认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.SubmitRequest true "实名申请"
// @Success 200 {object} dto.APIResponse[dto.VerificationInfo]
// @Router /api/v1/uc/verification [post]
func (h *VerificationUserHandler) Submit(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	var req dto.SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	username := ""
	if claims, ok := middleware.GetUserClaims(c); ok {
		username = claims.Username
	}
	data, err := h.service.Submit(c.Request.Context(), uid, username, req)
	if err != nil {
		respondUserVerificationErr(c, err)
		return
	}
	success(c, data)
}

// ListMine godoc
// @Summary 我的实名申请历史
// @Tags 用户中心-实名认证
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.VerificationInfo]]
// @Router /api/v1/uc/verification/applications [get]
func (h *VerificationUserHandler) ListMine(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	data, err := h.service.ListMyApplications(c.Request.Context(), uid, page, pageSize)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Detail godoc
// @Summary 我的实名申请详情
// @Description 越权访问他人申请返回 404（不泄露存在性）
// @Tags 用户中心-实名认证
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Success 200 {object} dto.APIResponse[dto.VerificationDetail]
// @Router /api/v1/uc/verification/applications/{id} [get]
func (h *VerificationUserHandler) Detail(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	data, err := h.service.MyApplication(c.Request.Context(), uid, id)
	if err != nil {
		respondUserVerificationErr(c, err)
		return
	}
	success(c, data)
}

// Authorize godoc
// @Summary 获取跳转式核验的认证入口
// @Description 支付宝实名认证：返回 auth_url 让用户跳转完成人脸/证件照认证
// @Tags 用户中心-实名认证
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Success 200 {object} dto.APIResponse[dto.AuthorizeResponse]
// @Router /api/v1/uc/verification/applications/{id}/authorize [post]
func (h *VerificationUserHandler) Authorize(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	data, err := h.service.Authorize(c.Request.Context(), uid, id)
	if err != nil {
		respondUserVerificationErr(c, err)
		return
	}
	success(c, data)
}

// ProviderCallback godoc
// @Summary 三方核验回调入口
// @Description 免登录。以服务端落库的 certify_id 为唯一关联依据，不接受请求体里的任何身份信息
// @Tags 用户中心-实名认证
// @Produce json
// @Param application_id query int true "申请ID"
// @Param certify_id query string true "核验流水号"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/uc/verification/{provider}/callback [get]
func (h *VerificationUserHandler) ProviderCallback(c *gin.Context) {
	applicationID, err := strconv.ParseUint(strings.TrimSpace(c.Query("application_id")), 10, 64)
	if err != nil || applicationID == 0 {
		badRequest(c, "application_id 不合法")
		return
	}
	certifyID := strings.TrimSpace(c.Query("certify_id"))
	if certifyID == "" {
		badRequest(c, "certify_id 不能为空")
		return
	}
	// 该入口免登录：服务层会校验 certify_id 与服务端落库的流水号一致，
	// 且核验结果只从三方接口查询获得，请求本身携带的任何信息都不被采信。
	if err := h.service.HandleProviderCallback(c.Request.Context(), applicationID, certifyID); err != nil {
		respondUserVerificationErr(c, err)
		return
	}
	success(c, "ok")
}

// UploadDocument godoc
// @Summary 上传实名认证材料
// @Description 证件照/营业执照。仅可上传到本人仍处于待审核的申请；同类型重传即覆盖
// @Tags 用户中心-实名认证
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请ID"
// @Param document_type formData string true "材料类型" Enums(id_front,id_back,handheld,business_license,authorization)
// @Param file formData file true "材料文件"
// @Success 200 {object} dto.APIResponse[dto.DocumentInfo]
// @Router /api/v1/uc/verification/applications/{id}/documents [post]
func (h *VerificationUserHandler) UploadDocument(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	applicationID, ok := parseVerificationID(c)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "请选择要上传的文件")
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		badRequest(c, "无法读取上传文件")
		return
	}
	defer f.Close()
	data, err := h.service.UploadDocument(
		c.Request.Context(), uid, applicationID, c.PostForm("document_type"), fileHeader.Filename, f)
	if err != nil {
		respondUserVerificationErr(c, err)
		return
	}
	success(c, data)
}

// DownloadDocument godoc
// @Summary 下载我的实名认证材料
// @Description 只能下载本人申请下的材料，越权按 404 处理
// @Tags 用户中心-实名认证
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path int true "材料ID"
// @Success 200 {file} file
// @Router /api/v1/uc/verification/documents/{id}/download [get]
func (h *VerificationUserHandler) DownloadDocument(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	id, ok := parseVerificationID(c)
	if !ok {
		return
	}
	doc, reader, err := h.service.OpenDocument(c.Request.Context(), id, false, uid)
	if err != nil {
		respondUserVerificationErr(c, err)
		return
	}
	defer reader.Close()
	// 落盘名是随机串，下载时还原成「材料类型 + 原扩展名」，避免用户拿到无语义文件名。
	fileName := service.DocumentFileName(doc.DocumentType, doc.FileURL)
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(fileName))
	c.DataFromReader(http.StatusOK, -1, service.DocumentContentType(doc.FileURL), reader, nil)
}

// respondUserVerificationErr 用户端实名错误映射。
//
// 与（用户不可见的）管理端口径的差别：所有「不是你的 / 不存在」都归 404，
// 「状态/前置条件不满足」归 409，让前端能直接按 code 决定是弹提示还是刷新状态。
func respondUserVerificationErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrApplicationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrPendingExists), errors.Is(err, service.ErrAlreadyVerified),
		errors.Is(err, service.ErrCooldown), errors.Is(err, service.ErrConfigNotAllowed),
		errors.Is(err, service.ErrStatusConflict), errors.Is(err, service.ErrNotInitializer):
		c.JSON(http.StatusConflict, gin.H{"code": 40901, "message": err.Error(), "timestamp": time.Now().Unix()})
	default:
		badRequest(c, err.Error())
	}
}
