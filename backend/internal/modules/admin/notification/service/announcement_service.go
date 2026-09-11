package service

import (
	"context"
	"time"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
)

// AnnouncementService 公告服务接口。
type AnnouncementService interface {
	List(ctx context.Context, q notifydto.AnnouncementListQuery) (*announcementListResponse, error)
	GetByID(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error)
	Create(ctx context.Context, req *notifydto.AnnouncementCreateRequest, operatorID uint64) (*notifydto.AnnouncementInfo, error)
	Update(ctx context.Context, id uint64, req *notifydto.AnnouncementUpdateRequest) (*notifydto.AnnouncementInfo, error)
	Publish(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error)
	Offline(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error)
	Delete(ctx context.Context, id uint64) error
	ListPublished(ctx context.Context, platform string) ([]notifydto.AnnouncementInfo, error)
}

type announcementListResponse struct {
	Total    int64                        `json:"total"`
	List     []notifydto.AnnouncementInfo `json:"list"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}

type announcementService struct {
	repo notifyrepo.AnnouncementRepository
}

func NewAnnouncementService(repo notifyrepo.AnnouncementRepository) AnnouncementService {
	return &announcementService{repo: repo}
}

func (s *announcementService) List(ctx context.Context, q notifydto.AnnouncementListQuery) (*announcementListResponse, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	items, total, err := s.repo.List(ctx, notifyrepo.AnnouncementListParams{
		Status:   q.Status,
		Platform: q.Platform,
		Keyword:  q.Keyword,
		Offset:   offset,
		Limit:    pageSize,
	})
	if err != nil {
		return nil, err
	}
	return &announcementListResponse{
		Total:    total,
		List:     toAnnouncementInfoList(items),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *announcementService) GetByID(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrAnnouncementNotFound
	}
	return toAnnouncementInfo(a), nil
}

func (s *announcementService) Create(ctx context.Context, req *notifydto.AnnouncementCreateRequest, operatorID uint64) (*notifydto.AnnouncementInfo, error) {
	platform := req.Platform
	if platform == "" {
		platform = notifymodel.AnnouncementPlatformUser
	}
	level := req.Level
	if level == "" {
		level = notifymodel.AnnouncementLevelInfo
	}
	status := notifymodel.AnnouncementDraft
	var publishAt *time.Time
	if req.PublishAt != "" {
		t, err := time.Parse("2006-01-02 15:04:05", req.PublishAt)
		if err == nil {
			publishAt = &t
			// 过去时间直接发布
			if t.Before(time.Now()) {
				status = notifymodel.AnnouncementPublished
			}
		}
	} else {
		// 无 publish_at = 立即发布
		status = notifymodel.AnnouncementPublished
		now := time.Now()
		publishAt = &now
	}
	a := &notifymodel.Announcement{
		Title:      req.Title,
		Content:    req.Content,
		Platform:   platform,
		Level:      level,
		Popup:      req.Popup,
		Status:     status,
		PublishAt:  publishAt,
		OperatorID: operatorID,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return toAnnouncementInfo(a), nil
}

func (s *announcementService) Update(ctx context.Context, id uint64, req *notifydto.AnnouncementUpdateRequest) (*notifydto.AnnouncementInfo, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrAnnouncementNotFound
	}
	if req.Title != "" {
		a.Title = req.Title
	}
	if req.Content != "" {
		a.Content = req.Content
	}
	if req.Platform != "" {
		a.Platform = req.Platform
	}
	if req.Level != "" {
		a.Level = req.Level
	}
	if req.Popup != nil {
		a.Popup = *req.Popup
	}
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	return toAnnouncementInfo(a), nil
}

func (s *announcementService) Publish(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrAnnouncementNotFound
	}
	if a.Status == notifymodel.AnnouncementPublished {
		// 已发布但可能残留 offline_at，清除之
		if a.OfflineAt != nil {
			_ = s.repo.ClearOfflineAt(ctx, id)
		}
		return toAnnouncementInfo(a), nil
	}
	a.Status = notifymodel.AnnouncementPublished
	now := time.Now()
	a.PublishAt = &now
	a.OfflineAt = nil // 清除下线标记
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	// 显式清除 offline_at（GORM Save 对 nil 指针不生成 NULL 语句）
	if err := s.repo.ClearOfflineAt(ctx, id); err != nil {
		return nil, err
	}
	return toAnnouncementInfo(a), nil
}

func (s *announcementService) Offline(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrAnnouncementNotFound
	}
	a.Status = notifymodel.AnnouncementOffline
	now := time.Now()
	a.OfflineAt = &now
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	return toAnnouncementInfo(a), nil
}

func (s *announcementService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *announcementService) ListPublished(ctx context.Context, platform string) ([]notifydto.AnnouncementInfo, error) {
	items, err := s.repo.ListPublished(ctx, platform)
	if err != nil {
		return nil, err
	}
	return toAnnouncementInfoList(items), nil
}

func toAnnouncementInfo(a *notifymodel.Announcement) *notifydto.AnnouncementInfo {
	var publishAt, offlineAt *string
	if a.PublishAt != nil {
		t := a.PublishAt.Format("2006-01-02 15:04:05")
		publishAt = &t
	}
	if a.OfflineAt != nil {
		t := a.OfflineAt.Format("2006-01-02 15:04:05")
		offlineAt = &t
	}
	return &notifydto.AnnouncementInfo{
		ID:         a.ID,
		Title:      a.Title,
		Content:    a.Content,
		Platform:   a.Platform,
		Level:      a.Level,
		Popup:      a.Popup,
		Status:     a.Status,
		PublishAt:  publishAt,
		OfflineAt:  offlineAt,
		OperatorID: a.OperatorID,
		CreatedAt:  a.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:  a.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toAnnouncementInfoList(items []notifymodel.Announcement) []notifydto.AnnouncementInfo {
	result := make([]notifydto.AnnouncementInfo, 0, len(items))
	for i := range items {
		result = append(result, *toAnnouncementInfo(&items[i]))
	}
	return result
}
