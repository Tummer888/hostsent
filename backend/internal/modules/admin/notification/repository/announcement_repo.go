package repository

import (
	"context"
	"errors"
	"time"

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
	// FindPublishedByID 取单条已发布且未下线公告；不存在返回 (nil, nil)，供门户判 404。
	FindPublishedByID(ctx context.Context, id uint64) (*notifymodel.Announcement, error)
	ClearOfflineAt(ctx context.Context, id uint64) error
	// PublishDue 把 publish_at 已到点、仍处于草稿态的公告置为已发布，返回推进条数。
	// 定时发布调度器（internal/pkg/publishsched）用它，见该包顶部说明。
	PublishDue(ctx context.Context, now time.Time) (int64, error)
}

// PublishDue 批量推进到点的草稿公告。
//
// 只挑 status=draft 且 publish_at 非空且已到点的行：已发布/已下线的公告不参与，
// 否则「下线了一条定时公告」会被下一轮扫描重新发布回来。
// 用 UPDATE ... WHERE 而不是先查后写，是为了在调度器重复触发时不产生竞态。
func (r *announcementRepository) PublishDue(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&notifymodel.Announcement{}).
		Where("status = ?", notifymodel.AnnouncementDraft).
		Where("publish_at IS NOT NULL AND publish_at <= ?", now).
		Updates(map[string]any{
			"status":     notifymodel.AnnouncementPublished,
			"offline_at": nil,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
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
	// 置顶优先，其次按发布时间倒序：重要通知必须压过常规公告。
	if err := query.Order("pinned DESC, publish_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// FindPublishedByID 取单条已发布且未下线的公告（门户详情用）。
// 不存在返回 nil 而不是错误：门户据此判 404，不能把「内容不存在」当成服务故障。
func (r *announcementRepository) FindPublishedByID(ctx context.Context, id uint64) (*notifymodel.Announcement, error) {
	var a notifymodel.Announcement
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ? AND offline_at IS NULL", id, notifymodel.AnnouncementPublished).
		First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *announcementRepository) ClearOfflineAt(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&notifymodel.Announcement{}).
		Where("id = ?", id).
		Update("offline_at", nil).Error
}
