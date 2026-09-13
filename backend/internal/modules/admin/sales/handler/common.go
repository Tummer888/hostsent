// Package handler 提供销售归属与提成的 HTTP 接口（管理端，doc86 §2.3）。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/sales/dto"
	"hostsent/backend/internal/modules/admin/sales/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// —— 公共工具 ——

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

// resolveScope 从鉴权快照推导数据范围（doc86 §2.4）：
//   - 超管：All=true，可看全部并可按 admin_id/department_id 过滤；
//   - 持有 sales:commission:audit（主管/财务）：按部门范围；
//   - 其余（销售本人）：只看到自己。
func resolveScope(c *gin.Context) dto.Scope {
	grant, ok := middleware.GetAdminGrant(c)
	if !ok {
		return dto.Scope{}
	}
	if grant.IsSuper() {
		return dto.Scope{All: true}
	}
	if grant.HasAny("sales:commission:audit") && grant.DepartmentID > 0 {
		return dto.Scope{DepartmentID: grant.DepartmentID}
	}
	adminID, _ := operatorFromContext(c)
	return dto.Scope{AdminID: adminID}
}

// writeError 将销售子域业务错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrCustomerNotFound), errors.Is(err, service.ErrRelationNotFound),
		errors.Is(err, service.ErrWithdrawNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrInvalidSales), errors.Is(err, service.ErrSameOwner),
		errors.Is(err, service.ErrReasonRequired), errors.Is(err, service.ErrInvalidAmount),
		errors.Is(err, service.ErrInvalidPeriod), errors.Is(err, service.ErrInvalidTarget),
		errors.Is(err, service.ErrPayoutReceiptRequired):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, service.ErrRelationProtected), errors.Is(err, service.ErrWithdrawStatus),
		errors.Is(err, service.ErrSalesDisabled), errors.Is(err, service.ErrPayoutNotConfigured):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrOutOfScope):
		return apperrors.New(40301, err.Error())
	case errors.Is(err, service.ErrInsufficientCommission), errors.Is(err, service.ErrBelowMinWithdraw):
		return apperrors.New(30001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
