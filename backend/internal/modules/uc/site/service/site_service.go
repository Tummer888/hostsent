// Package service 提供官网门户公开只读数据的业务编排。
package service

import (
	"context"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"

	"hostsent/backend/internal/modules/uc/site/dto"
)

const (
	// announcementDefaultLimit 未指定 limit 时返回的公告条数。
	announcementDefaultLimit = 5
	// announcementMaxLimit 单次请求上限，避免公开接口被当成翻全量数据的入口。
	announcementMaxLimit = 50
)

// announcementReader 公告读取能力的最小暴露接口（由装配层注入，避免 uc 依赖 admin service 实现）。
type announcementReader interface {
	ListPublished(ctx context.Context, platform string) ([]notifydto.AnnouncementInfo, error)
}

// SiteService 官网门户公开数据能力。
type SiteService interface {
	// ListAnnouncements 已发布公告，按发布时间倒序，按 limit 截断。
	ListAnnouncements(ctx context.Context, limit int) ([]dto.AnnouncementItem, error)
}

type siteService struct {
	announcements announcementReader
}

// NewSiteService 创建官网门户公开数据服务。
func NewSiteService(announcements announcementReader) SiteService {
	return &siteService{announcements: announcements}
}

func (s *siteService) ListAnnouncements(ctx context.Context, limit int) ([]dto.AnnouncementItem, error) {
	if limit <= 0 {
		limit = announcementDefaultLimit
	}
	if limit > announcementMaxLimit {
		limit = announcementMaxLimit
	}

	items, err := s.announcements.ListPublished(ctx, notifymodel.AnnouncementPlatformUser)
	if err != nil {
		return nil, err
	}
	if len(items) > limit {
		items = items[:limit]
	}

	out := make([]dto.AnnouncementItem, 0, len(items))
	for _, it := range items {
		out = append(out, dto.AnnouncementItem{
			ID:        it.ID,
			Title:     it.Title,
			Content:   it.Content,
			Level:     it.Level,
			Popup:     it.Popup,
			PublishAt: derefString(it.PublishAt),
		})
	}
	return out, nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
