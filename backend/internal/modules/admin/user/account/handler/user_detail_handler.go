package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/user/account/service"
)

type UserDetailHandler struct {
	detailService service.UserDetailService
}

func NewUserDetailHandler(detailService service.UserDetailService) *UserDetailHandler {
	return &UserDetailHandler{detailService: detailService}
}

// GetAggregate godoc
// @Summary 用户详情聚合
// @Description 获取用户详情页所需的资料、后台角色、客户侧权限、各域计数与近期摘要。
// @Description 除 profile 外的各段独立容错，采集失败时记入 degraded 而不使整页失败。
// @Tags 用户管理
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} dto.APIResponse[dto.UserDetailAggregateResponse]
// @Failure 400 {object} dto.APIResponse[string] "用户 ID 不合法"
// @Failure 404 {object} dto.APIResponse[string] "用户不存在"
// @Router /api/v1/admin/users/{id}/detail-aggregate [get]
func (h *UserDetailHandler) GetAggregate(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}
	data, err := h.detailService.GetAggregate(c.Request.Context(), id)
	if err != nil {
		respondUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}