package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/user/service"
)

type UserDetailHandler struct {
	detailService service.UserDetailService
}

func NewUserDetailHandler(detailService service.UserDetailService) *UserDetailHandler {
	return &UserDetailHandler{detailService: detailService}
}

// GetAggregate godoc
// @Summary 用户详情聚合
// @Description 获取用户详情页所需的资料、权限、实例、订单、账单、流水、工单聚合数据
// @Tags 用户管理
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} dto.APIResponse[dto.UserDetailAggregateResponse]
// @Router /api/v1/users/{id}/detail-aggregate [get]
func (h *UserDetailHandler) GetAggregate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.detailService.GetAggregate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}
