package repository

import (
	"context"

	"gorm.io/gorm"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// AnnouncementRepository 公告仓库。
type AnnouncementRepository interface {
	Create(ctx context.Context, a *notifymodel.Announcement) error
	Update(ctx context.Context, a *notifymodel.Announcement) error
	FindByID(ctx context.Context, id uint64) (*notifymodel.Announcement, error)
	Delete(ctx context.Context, id uint64) error
	List(ctx context.Context, q AnnouncementListParams) ([]notifymodel.Announcement, int64, error)
	ListPublished(ctx context.Context, platform string) ([]notifymodel.Announcement, error)
	ClearOfflineAt(ctx context.Context, id uint64) error
}

// AnnouncementListParams 公告列表查询参数。
type AnnouncementListParams struct {
	Status   string
	Platform string
	Keyword  string
	Offset   int
	Limit    int
}

type announcementRepository struct {
	db *gorm.DB
}

func NewAnnouncementRepository(db *gorm.DB) AnnouncementRepository {
	return &announcementRepository{db: db}
}

func (r *announcementRepository) Create(ctx context.Context, a *notifymodel.Announcement) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *announcementRepository) Update(ctx context.Context, a *notifymodel.Announcement) error {
	return r.db.WithContext(ctx).Save(a).Error
}

func (r *announcementRepository) FindByID(ctx context.Context, id uint64) (*notifymodel.Announcement, error) {
	var a notifymodel.Announcement
	if err := r.db.WithContext(ctx).First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *announcementRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&notifymodel.Announcement{}, id).Error
}

func (r *announcementRepository) List(ctx context.Context, q AnnouncementListParams) ([]notifymodel.Announcement, int64, error) {
	query := r.db.WithContext(ctx).Model(&notifymodel.Announcement{})
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.Platform != "" {
		query = query.Where("platform = ?", q.Platform)
	}
	if q.Keyword != "" {
		query = query.Where("title LIKE ?", "%"+q.Keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []notifymodel.Announcement
	if err := query.Order("created_at DESC").Offset(q.Offset).Limit(q.Limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *announcementRepository) ListPublished(ctx context.Context, platform string) ([]notifymodel.Announcement, error) {
	var items []notifymodel.Announcement
	query := r.db.WithContext(ctx).Model(&notifymodel.Announcement{}).
		Where("status = ?", notifymodel.AnnouncementPublished).
		Where("platform IN ?", []string{platform, notifymodel.AnnouncementPlatformBoth})
	// 排除已下线的
	query = query.Where("offline_at IS NULL")
	if err := query.Order("publish_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *announcementRepository) ClearOfflineAt(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&notifymodel.Announcement{}).
		Where("id = ?", id).
		Update("offline_at", nil).Error
}
