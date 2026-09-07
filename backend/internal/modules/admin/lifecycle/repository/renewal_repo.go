package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
)

// RenewalRepository 续费记录仓库。
type RenewalRepository interface {
	Create(ctx context.Context, renewal *lifecyclemodel.InstanceRenewal) error
	Update(ctx context.Context, renewal *lifecyclemodel.InstanceRenewal) error
	FindByID(ctx context.Context, id uint64) (*lifecyclemodel.InstanceRenewal, error)
	FindByRenewalNo(ctx context.Context, renewalNo string) (*lifecyclemodel.InstanceRenewal, error)
	FindByOrderID(ctx context.Context, orderID uint64) (*lifecyclemodel.InstanceRenewal, error)
	List(ctx context.Context, q RenewalListParams) ([]lifecyclemodel.InstanceRenewal, int64, error)
	ListByUser(ctx context.Context, userID uint64, status string, page, pageSize int) ([]lifecyclemodel.InstanceRenewal, int64, error)
	ListPendingAutoByInstance(ctx context.Context, instanceID uint64) ([]lifecyclemodel.InstanceRenewal, error)
	CountByStatus(ctx context.Context) (map[string]int64, error)
}

// RenewalListParams 续费记录列表查询参数。
type RenewalListParams struct {
	Keyword   string
	UserID    uint64
	Status    string
	Source    string
	StartTime string
	EndTime   string
	Page      int
	PageSize  int
}

type renewalRepository struct {
	db *gorm.DB
}

// NewRenewalRepository 创建续费记录仓库。
func NewRenewalRepository(db *gorm.DB) RenewalRepository {
	return &renewalRepository{db: db}
}

func (r *renewalRepository) Create(ctx context.Context, renewal *lifecyclemodel.InstanceRenewal) error {
	return r.db.WithContext(ctx).Create(renewal).Error
}

func (r *renewalRepository) Update(ctx context.Context, renewal *lifecyclemodel.InstanceRenewal) error {
	return r.db.WithContext(ctx).Save(renewal).Error
}

func (r *renewalRepository) FindByID(ctx context.Context, id uint64) (*lifecyclemodel.InstanceRenewal, error) {
	var renewal lifecyclemodel.InstanceRenewal
	if err := r.db.WithContext(ctx).First(&renewal, id).Error; err != nil {
		return nil, err
	}
	return &renewal, nil
}

func (r *renewalRepository) FindByRenewalNo(ctx context.Context, renewalNo string) (*lifecyclemodel.InstanceRenewal, error) {
	var renewal lifecyclemodel.InstanceRenewal
	if err := r.db.WithContext(ctx).Where("renewal_no = ?", renewalNo).First(&renewal).Error; err != nil {
		return nil, err
	}
	return &renewal, nil
}

func (r *renewalRepository) FindByOrderID(ctx context.Context, orderID uint64) (*lifecyclemodel.InstanceRenewal, error) {
	var renewal lifecyclemodel.InstanceRenewal
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&renewal).Error; err != nil {
		return nil, err
	}
	return &renewal, nil
}

func (r *renewalRepository) List(ctx context.Context, q RenewalListParams) ([]lifecyclemodel.InstanceRenewal, int64, error) {
	tx := r.db.WithContext(ctx).Model(&lifecyclemodel.InstanceRenewal{})
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		tx = tx.Where("renewal_no LIKE ? OR order_no LIKE ? OR instance_mark LIKE ?", kw, kw, kw)
	}
	if q.UserID > 0 {
		tx = tx.Where("user_id = ?", q.UserID)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Source != "" {
		tx = tx.Where("source = ?", q.Source)
	}
	if q.StartTime != "" {
		tx = tx.Where("created_at >= ?", q.StartTime)
	}
	if q.EndTime != "" {
		tx = tx.Where("created_at <= ?", q.EndTime+" 23:59:59")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 10
	}
	var items []lifecyclemodel.InstanceRenewal
	if err := tx.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *renewalRepository) ListByUser(ctx context.Context, userID uint64, status string, page, pageSize int) ([]lifecyclemodel.InstanceRenewal, int64, error) {
	return r.List(ctx, RenewalListParams{UserID: userID, Status: status, Page: page, PageSize: pageSize})
}

// ListPendingAutoByInstance 查询实例待支付的自动续费记录。
func (r *renewalRepository) ListPendingAutoByInstance(ctx context.Context, instanceID uint64) ([]lifecyclemodel.InstanceRenewal, error) {
	var items []lifecyclemodel.InstanceRenewal
	err := r.db.WithContext(ctx).
		Where("instance_id = ? AND source = ? AND status = ?", instanceID, lifecyclemodel.RenewalSourceAuto, lifecyclemodel.RenewalStatusPending).
		Find(&items).Error
	return items, err
}

func (r *renewalRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Status string `gorm:"column:status"`
		Count  int64  `gorm:"column:count"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&lifecyclemodel.InstanceRenewal{}).
		Select("status, COUNT(*) AS count").Group("status").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, item := range rows {
		result[item.Status] = item.Count
	}
	return result, nil
}

// genRenewalNo 生成续费单号：RN + yyyymmddHHMMSS + 6 位随机。
func genRenewalNo() string {
	return fmt.Sprintf("RN%s%06d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000)
}

var _ = errors.Is // 保留 errors 引用（未来扩展）
