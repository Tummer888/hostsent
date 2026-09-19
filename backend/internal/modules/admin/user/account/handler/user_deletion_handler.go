package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/repository"
	"hostsent/backend/internal/modules/admin/user/account/service"
	"hostsent/backend/internal/pkg/middleware"
)

// UserDeletionHandler 用户注销 / 恢复 / 回收站 / 留存期清理（doc104 §4）。
//
// 独立于 UserHandler：注销相关接口的操作人必须来自登录态（adminAuth 中间件），
// 且全部是写操作 + 高危，集中在一个 handler 里便于统一审查权限与留痕。
type UserDeletionHandler struct {
	deletionService service.DeletionService
}

func NewUserDeletionHandler(deletionService service.DeletionService) *UserDeletionHandler {
	return &UserDeletionHandler{deletionService: deletionService}
}

// operatorID 取当前管理员 ID。
//
// 与既有 Recharge/CreateOrder 的 `if claims, ok := ...` 静默回落 0 不同，这里
// 返回错误：注销是高危不可逆动作，拿不到操作人就不该放行（doc104 §3.1 F5 ——
// 安全模块里那 7 处硬编码 operatorID=1 正是静默兜底的后果）。
func operatorID(c *gin.Context) (uint64, error) {
	claims, ok := middleware.GetAdminClaims(c)
	if !ok || claims.AdminID == 0 {
		return 0, service.ErrInvalidOperator
	}
	return claims.AdminID, nil
}

// CheckDeletion godoc
// @Summary 用户注销前置校验
// @Description 返回阻断项（在管实例）与警告项（余额/未结账单/工单/订单），供前端弹窗展示
// @Tags 用户管理
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} dto.APIResponse[dto.UserDeletionCheckResponse]
// @Router /api/v1/admin/users/{id}/deletion-check [get]
func (h *UserDeletionHandler) CheckDeletion(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}
	resp, err := h.deletionService.Check(c.Request.Context(), id)
	if err != nil {
		respondDeletionErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

// DeleteUser godoc
// @Summary 注销用户（软删除）
// @Description 软删除用户并收敛状态为 cancelled；存在警告项时需 force=true。留存期到期后由清理任务硬删除
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body dto.UserDeleteRequest true "注销参数"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/users/{id} [delete]
func (h *UserDeletionHandler) DeleteUser(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}
	var req dto.UserDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, err.Error())
		return
	}
	op, err := operatorID(c)
	if err != nil {
		respondDeletionErr(c, err)
		return
	}
	if err := h.deletionService.SoftDelete(c.Request.Context(), id, req.Reason, req.Force, op); err != nil {
		respondDeletionErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

// RestoreUser godoc
// @Summary 恢复已注销用户
// @Description 清除删除标记，状态还原为注销前的状态（快照为空则回落 disabled）
// @Tags 用户管理
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/users/{id}/restore [post]
func (h *UserDeletionHandler) RestoreUser(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}
	if err := h.deletionService.Restore(c.Request.Context(), id); err != nil {
		respondDeletionErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

// BatchDeleteUsers godoc
// @Summary 批量注销用户
// @Description 逐个注销，返回成功数与逐条跳过原因（单条失败不影响其余）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param request body dto.UserBatchDeleteRequest true "批量注销参数"
// @Success 200 {object} dto.APIResponse[dto.UserBatchResult]
// @Router /api/v1/admin/users/batch-delete [post]
func (h *UserDeletionHandler) BatchDeleteUsers(c *gin.Context) {
	var req dto.UserBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, err.Error())
		return
	}
	op, err := operatorID(c)
	if err != nil {
		respondDeletionErr(c, err)
		return
	}
	resp, err := h.deletionService.BatchSoftDelete(c.Request.Context(), req.IDs, req.Reason, req.Force, op)
	if err != nil {
		respondDeletionErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

// BatchRestoreUsers godoc
// @Summary 批量恢复用户
// @Description 逐个恢复，返回成功数与逐条跳过原因
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param request body dto.UserBatchRestoreRequest true "批量恢复参数"
// @Success 200 {object} dto.APIResponse[dto.UserBatchResult]
// @Router /api/v1/admin/users/batch-restore [post]
func (h *UserDeletionHandler) BatchRestoreUsers(c *gin.Context) {
	var req dto.UserBatchRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, err.Error())
		return
	}
	resp, err := h.deletionService.BatchRestore(c.Request.Context(), req.IDs)
	if err != nil {
		respondDeletionErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

// PurgeUsers godoc
// @Summary 留存期到期用户清理
// @Description 硬删除留存期已过的注销用户；dry_run=true 只预览不写库
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param request body dto.UserPurgeRequest true "清理参数"
// @Success 200 {object} dto.APIResponse[dto.UserPurgeResponse]
// @Router /api/v1/admin/users/purge [post]
func (h *UserDeletionHandler) PurgeUsers(c *gin.Context) {
	var req dto.UserPurgeRequest
	// 空 body 是合法调用（按默认参数跑一轮），因此绑定失败只在有 body 时才算错误。
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			abortBadRequest(c, err.Error())
			return
		}
	}
	if _, err := operatorID(c); err != nil {
		respondDeletionErr(c, err)
		return
	}
	resp, err := h.deletionService.Purge(c.Request.Context(), req.DryRun, req.Limit)
	if err != nil {
		respondDeletionErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

// respondDeletionErr 注销相关错误的 HTTP 映射。
//
// 单独一份而不是复用 respondUserErr：注销的失败大多是「业务前置条件不满足」
// （409）而不是「用户不存在」（404）或「服务器错误」（500），混在一起会让
// 前端只能靠 message 文本判断该不该弹 force 确认框。
func respondDeletionErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		respondUserErr(c, err)
	case errors.Is(err, service.ErrDeletionBlocked), errors.Is(err, repository.ErrUserHasInstances):
		c.JSON(http.StatusConflict, gin.H{"code": 40902, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrDeletionNeedsForce):
		c.JSON(http.StatusConflict, gin.H{"code": 40903, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrUserDeleted), errors.Is(err, service.ErrUserNotDeleted):
		c.JSON(http.StatusConflict, gin.H{"code": 40901, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrRestoreConflict):
		// 40904 单独一个码：前端要能把它与「重复注销」区分开，
		// 提示运营「先去改掉新用户的账号名或邮箱」而不是「这单不用恢复了」。
		c.JSON(http.StatusConflict, gin.H{"code": 40904, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrInvalidOperator):
		c.JSON(http.StatusForbidden, gin.H{"code": 40301, "message": err.Error(), "timestamp": time.Now().Unix()})
	default:
		respondUserErr(c, err)
	}
}
