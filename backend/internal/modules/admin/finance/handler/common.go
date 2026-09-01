package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/service"
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

// writeError 将财务域业务错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrWalletNotFound),
		errors.Is(err, service.ErrRechargeNotFound),
		errors.Is(err, service.ErrWithdrawNotFound),
		errors.Is(err, service.ErrBillNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrStatusConflict):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrBizExist):
		return apperrors.New(30005, err.Error())
	case errors.Is(err, service.ErrInsufficientBalance):
		return apperrors.New(30001, err.Error())
	case errors.Is(err, service.ErrFrozenInsufficient):
		return apperrors.New(30006, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
