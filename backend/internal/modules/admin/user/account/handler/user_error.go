package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"

	"hostsent/backend/internal/modules/admin/user/account/service"
)

// parseUserID 解析路径参数中的用户 ID。
//
// 原实现一律 `id, _ := strconv.ParseUint(c.Param("id"), 10, 64)`：非法 id 静默变成 0，
// 再往下走就变成「查 id=0 的用户」→ record not found → 500，管理员看到的是
// 「服务器内部错误」而不是「用户不存在」。这里显式区分 400 与后续的 404。
func parseUserID(c *gin.Context) (uint64, bool) {
	raw := c.Param("id")
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		abortBadRequest(c, "用户 ID 不合法")
		return 0, false
	}
	return id, true
}

func abortBadRequest(c *gin.Context, message string) {
	response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, message))
}

// respondUserErr 把 service 层错误映射为 HTTP 语义，并避免把原始 GORM 文本回给前端。
// 参照 manager/handler/department_handler.go 的 mapDepartmentErr。
func respondUserErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		response.ErrorWithStatus(c, http.StatusNotFound, apperrors.New(40401, "用户不存在"))
	case errors.Is(err, service.ErrUsernameTaken):
		response.ErrorWithStatus(c, http.StatusConflict, apperrors.New(40901, service.ErrUsernameTaken.Error()))
	case errors.Is(err, service.ErrEmailTaken):
		response.ErrorWithStatus(c, http.StatusConflict, apperrors.New(40901, service.ErrEmailTaken.Error()))
	case errors.Is(err, service.ErrRoleScopeMismatch), errors.Is(err, service.ErrEmptyRoleList),
		errors.Is(err, service.ErrLevelDisabled), errors.Is(err, service.ErrLevelAssignUnavailable),
		errors.Is(err, service.ErrAgentLevelDisabled), errors.Is(err, service.ErrAgentLevelUnavailable):
		// 角色域不匹配 / 角色列表为空 / 目标等级已停用都是调用方入参问题，不是服务端故障：
		// 回落 default 会返回 50001，前端只能提示「服务器错误」，运营看不出
		// 自己选错了角色或等级。
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(40001, err.Error()))
	case errors.Is(err, service.ErrLevelNotFound), errors.Is(err, service.ErrAgentLevelNotFound):
		// 等级不存在单独成一条：上面的 ErrUserNotFound 分支会把它翻成
		// 「用户不存在」，改等级/代理等级时选中一个不存在的等级不该指错对象。
		code := service.ErrLevelNotFound
		if errors.Is(err, service.ErrAgentLevelNotFound) {
			code = service.ErrAgentLevelNotFound
		}
		response.ErrorWithStatus(c, http.StatusNotFound, apperrors.New(40401, code.Error()))
	default:
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
	}
}
