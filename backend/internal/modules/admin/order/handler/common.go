package handler

import (
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/service"
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

// operatorFromContext 从鉴权上下文提取操作者 ID 与名称。
func operatorFromContext(c *gin.Context) (uint64, string) {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID, claims.Username
	}
	return 0, ""
}

// writeAttachment 统一附件下载响应：还原原始文件名并给出标准 Content-Type
// （与工单附件同口径：RFC 5987 filename* 传中文名，未知类型按二进制流避免内联渲染）。
func writeAttachment(c *gin.Context, fileName, fileType string, size int64, reader io.Reader) {
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(fileName))
	c.DataFromReader(200, size, downloadContentType(fileType), reader, nil)
}

// downloadContentType 扩展名 → MIME；未知一律 application/octet-stream。
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
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

// bindBatchIDs 绑定批量操作请求体（ids）。
func bindBatchIDs(c *gin.Context) ([]uint64, bool) {
	var req dto.BatchIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return nil, false
	}
	return req.IDs, true
}

// writeError 将业务错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrOrderNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrRefundNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrStatusConflict),
		errors.Is(err, service.ErrVerifyNotAllowed),
		errors.Is(err, service.ErrOrderHasRefund),
		errors.Is(err, service.ErrRefundNotAllowed),
		errors.Is(err, service.ErrRefundChannelUnavailable):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrRefundPolicyNotFound),
		errors.Is(err, service.ErrRefundAttachmentNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrRefundPolicyDuplicate),
		errors.Is(err, service.ErrRefundAttachmentLimit),
		errors.Is(err, service.ErrRefundAttachmentTooLarge),
		errors.Is(err, service.ErrRefundAttachmentType):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, service.ErrRefundExceeded),
		errors.Is(err, service.ErrRefundModeInvalid),
		errors.Is(err, service.ErrRefundFeeInvalid):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrBatchEmpty),
		errors.Is(err, service.ErrBatchTooLarge):
		return apperrors.New(20001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
