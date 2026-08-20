// Package repository 提供实名认证模块的数据访问实现。
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/verification/dto"
	"hostsent/backend/internal/modules/verification/model"
)

// VerificationRepository 定义实名认证记录查询能力。
type VerificationRepository interface {
	ListByStatus(ctx context.Context, status string, query dto.VerificationListQuery) ([]model.VerificationApplication, int64, error)
}

type verificationRepository struct {
	db *gorm.DB
}

// NewVerificationRepository 创建实名认证记录仓储。
func NewVerificationRepository(db *gorm.DB) VerificationRepository {
	return &verificationRepository{db: db}
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// ListByStatus 查询指定状态的实名认证记录列表。
func (r *verificationRepository) ListByStatus(ctx context.Context, status string, query dto.VerificationListQuery) ([]model.VerificationApplication, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.VerificationApplication{}).Where("status = ?", status)
	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}
	if username := strings.TrimSpace(query.Username); username != "" {
		base = base.Where("username ILIKE ?", "%"+username+"%")
	}
	if verificationType := strings.TrimSpace(query.VerificationType); verificationType != "" {
		base = base.Where("verification_type = ?", verificationType)
	}
	if reviewerName := strings.TrimSpace(query.ReviewerName); reviewerName != "" {
		base = base.Where("reviewer_name ILIKE ?", "%"+reviewerName+"%")
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("username ILIKE ? OR subject_name ILIKE ? OR real_name ILIKE ? OR id_number_masked ILIKE ?", like, like, like, like)
	}
	if startTime := strings.TrimSpace(query.StartTime); startTime != "" {
		if parsed, err := time.Parse(time.RFC3339, startTime); err == nil {
			base = base.Where("submitted_at >= ?", parsed)
		}
	}
	if endTime := strings.TrimSpace(query.EndTime); endTime != "" {
		if parsed, err := time.Parse(time.RFC3339, endTime); err == nil {
			base = base.Where("submitted_at <= ?", parsed)
		}
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.VerificationApplication
	if err := base.Order("submitted_at desc, id desc").Offset((page-1)*pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
