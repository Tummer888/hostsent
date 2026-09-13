package handler

import (
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/ticket/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// pathID 解析路径参数指定的数字 ID，解析失败时直接写入错误响应。
func pathID(c *gin.Context, param string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid "+param))
		return 0, false
	}
	return id, true
}

// operatorFromContext 从鉴权上下文提取管理员操作者 ID 与名称。
func operatorFromContext(c *gin.Context) (uint64, string) {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID, claims.Username
	}
	return 0, ""
}

// currentUserID 返回数据归属账号 ID（子账号为主账号）与操作人账号名（P4-04）。
// 工单列表/详情按归属查询，子账号能看到主账号的全部工单。
func currentUserID(c *gin.Context) (uint64, string, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, "", false
	}
	return userID, middleware.ActorUsername(c), true
}

// currentActorID 返回真实操作人 ID 与账号名（P4-04）。
// 子账号提交工单时，提交人记操作人自身，便于主账号区分是谁提的。
func currentActorID(c *gin.Context) (uint64, string, bool) {
	userID := middleware.ActorUserID(c)
	if userID == 0 {
		return 0, "", false
	}
	return userID, middleware.ActorUsername(c), true
}

// hasAdminPerm 判断当前管理员是否持有某权限码（超管恒真）。
// 用于「同一接口内的能力降级」：如内部备注、内部附件，无权限时按普通内容处理。
func hasAdminPerm(c *gin.Context, code string) bool {
	grant, ok := middleware.GetAdminGrant(c)
	if !ok {
		return false
	}
	return grant.IsSuper() || grant.HasAny(code)
}

// writeAttachment 统一附件下载响应：还原原始文件名并给出标准 Content-Type。
func writeAttachment(c *gin.Context, fileName, fileType string, size int64, reader io.Reader) {
	// 文件名可能含中文，用 RFC 5987 的 filename* 传 UTF-8，同时保留 filename 兼容老客户端。
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(fileName))
	contentType := downloadContentType(fileType)
	c.DataFromReader(200, size, contentType, reader, nil)
}

// downloadContentType 把存储的扩展名映射为 MIME；未知类型一律按二进制流，
// 避免把用户上传的内容当 HTML 内联渲染（XSS/挂马风险）。
func downloadContentType(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".pdf":
		return "application/pdf"
	case ".txt", ".log", ".md":
		return "text/plain; charset=utf-8"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".json":
		return "application/json"
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/vnd.rar"
	case ".7z":
		return "application/x-7z-compressed"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

// writeError 将工单域业务错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrTicketNotFound),
		errors.Is(err, service.ErrReplyNotFound),
		errors.Is(err, service.ErrCategoryNotFound),
		errors.Is(err, service.ErrAdminNotFound),
		errors.Is(err, service.ErrAttachmentNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrStatusConflict):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrTicketAssigned),
		errors.Is(err, service.ErrSameAssignee),
		errors.Is(err, service.ErrNotPendingReview):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrReviewSelf),
		errors.Is(err, service.ErrInternalNoteForbidden):
		return apperrors.New(40301, err.Error())
	case errors.Is(err, service.ErrCategoryCodeExists):
		return apperrors.New(30005, err.Error())
	case errors.Is(err, service.ErrCategoryInUse):
		return apperrors.New(30005, err.Error())
	case errors.Is(err, service.ErrInvalidPriority),
		errors.Is(err, service.ErrReplyContentRequired),
		errors.Is(err, service.ErrRealnameRequired),
		errors.Is(err, service.ErrBindingRequired),
		errors.Is(err, service.ErrBindingNotOwned),
		errors.Is(err, service.ErrCategoryNotAllowed),
		errors.Is(err, service.ErrAttachmentTypeNotAllowed),
		errors.Is(err, service.ErrReviewNoteRequired),
		errors.Is(err, service.ErrInvalidReviewAction):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, service.ErrAttachmentTooLarge):
		return apperrors.New(20001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}

// unauthorized 用户未登录或身份缺失。
func unauthorized(c *gin.Context) {
	response.Error(c, apperrors.New(10001, "unauthorized"))
}
