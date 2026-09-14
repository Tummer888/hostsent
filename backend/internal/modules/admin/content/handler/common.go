package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

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

// operatorID 取当前管理员 ID，用于文章的操作人留痕。
//
// 取不到时返回 0 而不是报错：留痕是审计信息，缺了它内容本身仍然有效，
// 不该因为 claims 结构变化就阻断发布。
func operatorID(c *gin.Context) uint64 {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID
	}
	return 0
}
