// Package handler 提供官网门户公开接口的 HTTP 处理。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	siteservice "hostsent/backend/internal/modules/uc/site/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// SiteHandler 官网门户公开数据 HTTP 处理器。
type SiteHandler struct {
	siteSvc siteservice.SiteService
}

// NewSiteHandler 创建官网门户公开数据处理器。
func NewSiteHandler(siteSvc siteservice.SiteService) *SiteHandler {
	return &SiteHandler{siteSvc: siteSvc}
}

// Announcements 公开公告列表（仅已发布，无需登录）。
//
// 该接口供官网门户 SSR/BFF 消费，因此显式声明可共享缓存；
// 后台发布/下线公告后由主动失效接口清理，不必等 TTL。
func (h *SiteHandler) Announcements(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	items, err := h.siteSvc.ListAnnouncements(c.Request.Context(), limit)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	c.Header("Cache-Control", "public, max-age=300, stale-while-revalidate=600")
	response.Success(c, gin.H{"items": items})
}
