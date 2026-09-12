// Package handler 提供支付中心管理端 HTTP 接口。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/payment/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/payment"
	"hostsent/backend/internal/pkg/response"
)

// pathID 解析路径参数数字 ID。
func pathID(c *gin.Context, param string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid "+param))
		return 0, false
	}
	return id, true
}

// operatorFromContext 提取后台操作者。
func operatorFromContext(c *gin.Context) (uint64, string) {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID, claims.Username
	}
	return 0, ""
}

// pageParams 解析分页参数（page/page_size）。
func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	return page, pageSize
}

// writeError 支付域业务错误 → 统一错误码。
func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrChannelNotFound), errors.Is(err, service.ErrOrderNotFound),
		errors.Is(err, service.ErrPayoutNotFound), errors.Is(err, service.ErrAccountNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrChannelCodeExists):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrChannelTypeUnknown):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, service.ErrOrderNotPayable), errors.Is(err, service.ErrPayoutStatusConflict):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrNoChannelAvailable):
		return apperrors.New(30007, err.Error())
	case errors.Is(err, service.ErrRefundAmountExceeded):
		return apperrors.New(30008, err.Error())
	case errors.Is(err, service.ErrBalanceInsufficient):
		return apperrors.New(30001, err.Error())
	case errors.Is(err, payment.ErrAmountMismatch), errors.Is(err, payment.ErrSignatureInvalid):
		return apperrors.New(30009, err.Error())
	case errors.Is(err, payment.ErrNotImplemented), errors.Is(err, service.ErrRefundNotSupported),
		errors.Is(err, service.ErrPayoutNotSupported):
		return apperrors.New(30010, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
