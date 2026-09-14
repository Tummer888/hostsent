package service

import (
	"context"
	"strings"
	"time"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	"hostsent/backend/internal/pkg/revalidate"
	"hostsent/backend/internal/pkg/sanitize"
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
	// GetPublishedByID 单条已发布且未下线的公告；不存在返回 (nil, nil)。
	// 门户据此判 404，而不是把「内容不存在」当服务故障报 503。
	GetPublishedByID(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error)
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
	content, format := prepareAnnouncementBody(req.Content, req.BodyFormat)
	a := &notifymodel.Announcement{
		Title:      req.Title,
		Content:    content,
		BodyFormat: format,
		Slug:       strings.TrimSpace(req.Slug),
		Platform:   platform,
		Level:      level,
		Popup:      req.Popup,
		Pinned:     req.Pinned,
		Status:     status,
		PublishAt:  publishAt,
		OperatorID: operatorID,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	// 已发布的公告（含 publish_at 落在过去的）改动后要让门户立刻可见；
	// 纯草稿不通知 —— 草稿在门户本来就不可见，白清一次缓存。
	if a.Status == notifymodel.AnnouncementPublished {
		revalidate.Notify(ctx, revalidate.KeyAnnouncement)
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
		content, format := prepareAnnouncementBody(req.Content, req.BodyFormat)
		a.Content = content
		a.BodyFormat = format
	}
	if req.Slug != "" {
		a.Slug = strings.TrimSpace(req.Slug)
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
	if req.Pinned != nil {
		a.Pinned = *req.Pinned
	}
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	// 门户上的公告列表/详情走 SWR，改完必须主动失效（doc80 §10.1）。
	revalidate.Notify(ctx, revalidate.KeyAnnouncement)
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
		revalidate.Notify(ctx, revalidate.KeyAnnouncement)
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
	// 「发布即见」：门户公告列表/详情/首页公告位都在 SWR 里，不清缓存最长滞后一个 TTL。
	revalidate.Notify(ctx, revalidate.KeyAnnouncement)
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
	// 下线同样要让门户立刻撤下，否则缓存里还留着已下线公告。
	revalidate.Notify(ctx, revalidate.KeyAnnouncement)
	return toAnnouncementInfo(a), nil
}

func (s *announcementService) Delete(ctx context.Context, id uint64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// 已删除的公告在门户可能还有缓存页面（详情/列表），一并失效。
	revalidate.Notify(ctx, revalidate.KeyAnnouncement)
	return nil
}

func (s *announcementService) ListPublished(ctx context.Context, platform string) ([]notifydto.AnnouncementInfo, error) {
	items, err := s.repo.ListPublished(ctx, platform)
	if err != nil {
		return nil, err
	}
	return toAnnouncementInfoList(items), nil
}

// GetPublishedByID 单条已发布公告（门户详情用）。不存在返回 (nil, nil)。
func (s *announcementService) GetPublishedByID(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error) {
	a, err := s.repo.FindPublishedByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil
	}
	return toAnnouncementInfo(a), nil
}

// prepareAnnouncementBody 收敛公告正文：html 走服务端净化后入库，其余一律按纯文本存。
//
// 存量公告（迁移 044 之前）body_format 为空/ text，前端按文本节点渲染；
// 管理端一旦用富文本编辑器提交，这里给出的就是已经过滤掉 script/onerror 等载荷的 HTML。
func prepareAnnouncementBody(content, bodyFormat string) (string, string) {
	if strings.TrimSpace(content) == "" {
		return "", notifymodel.AnnouncementFormatText
	}
	if strings.EqualFold(strings.TrimSpace(bodyFormat), notifymodel.AnnouncementFormatHTML) {
		return strings.TrimSpace(sanitize.HTML(content)), notifymodel.AnnouncementFormatHTML
	}
	return content, notifymodel.AnnouncementFormatText
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
	format := a.BodyFormat
	if format == "" {
		format = notifymodel.AnnouncementFormatText
	}
	return &notifydto.AnnouncementInfo{
		ID:         a.ID,
		Title:      a.Title,
		Content:    a.Content,
		BodyFormat: format,
		Slug:       a.Slug,
		Platform:   a.Platform,
		Level:      a.Level,
		Popup:      a.Popup,
		Pinned:     a.Pinned,
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
