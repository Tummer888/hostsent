package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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
	c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": message, "timestamp": time.Now().Unix()})
}

// respondUserErr 把 service 层错误映射为 HTTP 语义，并避免把原始 GORM 文本回给前端。
// 参照 manager/handler/department_handler.go 的 mapDepartmentErr。
func respondUserErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "用户不存在", "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrUsernameTaken):
		c.JSON(http.StatusConflict, gin.H{"code": 40901, "message": service.ErrUsernameTaken.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"code": 40901, "message": service.ErrEmailTaken.Error(), "timestamp": time.Now().Unix()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
	}
}
