// Package handler 提供官网门户公开接口的 HTTP 处理。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	siteservice "hostsent/backend/internal/modules/site/service"
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
//
// @Summary 官网公开公告列表
// @Tags 官网门户-公开
// @Produce json
// @Param limit query int false "返回条数（默认 5，上限 50）"
// @Success 200 {object} response.Body{data=dto.AnnouncementListResponse}
// @Router /api/v1/public/announcements [get]
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

// SiteContent 站点品牌配置（品牌名 / Logo / 版权 / 备案号 / 联系方式 / 首页文案）。
//
// 供官网门户 SSR/BFF 消费：品牌信息变更需尽快可见，故 TTL 取 60s（doc80 §4.3）；
// 后台保存后由主动失效接口清理，不必等 TTL。
// 字段经白名单过滤（internal.* 永不外露），无配置时返回空 items，由前端回落代码默认值。
//
// @Summary 官网品牌与站点配置
// @Tags 官网门户-公开
// @Produce json
// @Success 200 {object} response.Body{data=dto.SiteContentResponse}
// @Router /api/v1/public/site-content [get]
func (h *SiteHandler) SiteContent(c *gin.Context) {
	content, err := h.siteSvc.SiteContent(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	c.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	response.Success(c, content)
}
