package handler

import (
	"errors"
	"strconv"

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

// writeError 将工单域业务错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrTicketNotFound),
		errors.Is(err, service.ErrReplyNotFound),
		errors.Is(err, service.ErrCategoryNotFound),
		errors.Is(err, service.ErrAdminNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrStatusConflict):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrTicketAssigned),
		errors.Is(err, service.ErrSameAssignee):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrCategoryCodeExists):
		return apperrors.New(30005, err.Error())
	case errors.Is(err, service.ErrCategoryInUse):
		return apperrors.New(30005, err.Error())
	case errors.Is(err, service.ErrInvalidPriority):
		return apperrors.New(20001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}

// unauthorized 用户未登录或身份缺失。
func unauthorized(c *gin.Context) {
	response.Error(c, apperrors.New(10001, "unauthorized"))
}
