// Package repository 提供用户中心成员（子账号）模块的数据访问实现。
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/member/dto"
	"hostsent/backend/internal/modules/uc/member/model"
)

// MemberRepository 成员（子账号）数据访问能力。
type MemberRepository interface {
	List(ctx context.Context, ownerID uint64, query dto.MemberListQuery) ([]model.Member, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Member, error)
	FindByUsername(ctx context.Context, username string) (*model.Member, error)
	FindByEmail(ctx context.Context, email string) (*model.Member, error)
	FindByPhone(ctx context.Context, phone string) (*model.Member, error)
	Create(ctx context.Context, member *model.Member) error
	UpdateProfileFields(ctx context.Context, id uint64, fields map[string]any) error
	CountSubAccounts(ctx context.Context, ownerID uint64) (int64, error)
	// MaxSubAccountsOf 返回主账号当前等级的子账号上限；无等级配置时返回 0（表示不限制）。
	MaxSubAccountsOf(ctx context.Context, ownerID uint64) (int, error)
	// PermissionsOf 返回指定用户已授予的权限码（满足 middleware.SubAccountPermissionResolver）。
	PermissionsOf(ctx context.Context, userID uint64) ([]string, error)
	ReplacePermissions(ctx context.Context, userID uint64, codes []string) error
	// —— 操作日志（P4-08）——
	CreateOperationLog(ctx context.Context, log *model.OperationLog) error
	ListOperationLogs(ctx context.Context, actorUserID uint64, page, pageSize int) ([]model.OperationLog, int64, error)
}

type memberRepository struct {
	db *gorm.DB
}

// NewMemberRepository 创建成员仓储实现。
func NewMemberRepository(db *gorm.DB) MemberRepository {
	return &memberRepository{db: db}
}

func (r *memberRepository) List(ctx context.Context, ownerID uint64, query dto.MemberListQuery) ([]model.Member, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.Member{}).
		Where("owner_user_id = ? AND is_sub_account = true", ownerID)
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("username ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR sub_account_remark ILIKE ?", like, like, like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Member
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *memberRepository) FindByID(ctx context.Context, id uint64) (*model.Member, error) {
	var item model.Member
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *memberRepository) FindByUsername(ctx context.Context, username string) (*model.Member, error) {
	var item model.Member
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *memberRepository) FindByEmail(ctx context.Context, email string) (*model.Member, error) {
	var item model.Member
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *memberRepository) FindByPhone(ctx context.Context, phone string) (*model.Member, error) {
	var item model.Member
	if err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *memberRepository) Create(ctx context.Context, member *model.Member) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *memberRepository) UpdateProfileFields(ctx context.Context, id uint64, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.Member{}).Where("id = ?", id).Updates(fields).Error
}

func (r *memberRepository) CountSubAccounts(ctx context.Context, ownerID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Member{}).
		Where("owner_user_id = ? AND is_sub_account = true", ownerID).
		Count(&count).Error
	return count, err
}

func (r *memberRepository) MaxSubAccountsOf(ctx context.Context, ownerID uint64) (int, error) {
	var row struct {
		MaxSubAccounts int
	}
	err := r.db.WithContext(ctx).
		Table("users").
		Select("COALESCE(user_levels.max_sub_accounts, 0) AS max_sub_accounts").
		Joins("LEFT JOIN user_levels ON user_levels.id = users.user_level_id").
		Where("users.id = ?", ownerID).
		Take(&row).Error
	if err != nil {
		return 0, err
	}
	return row.MaxSubAccounts, nil
}

func (r *memberRepository) PermissionsOf(ctx context.Context, userID uint64) ([]string, error) {
	var codes []string
	if err := r.db.WithContext(ctx).Model(&model.SubAccountPermission{}).
		Where("user_id = ?", userID).
		Pluck("permission_code", &codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// ReplacePermissions 覆盖式写入权限：先清空再插入（同事务）。
func (r *memberRepository) ReplacePermissions(ctx context.Context, userID uint64, codes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.SubAccountPermission{}).Error; err != nil {
			return err
		}
		if len(codes) == 0 {
			return nil
		}
		rows := make([]model.SubAccountPermission, 0, len(codes))
		for _, code := range codes {
			rows = append(rows, model.SubAccountPermission{UserID: userID, PermissionCode: code})
		}
		return tx.Create(&rows).Error
	})
}

func (r *memberRepository) CreateOperationLog(ctx context.Context, log *model.OperationLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *memberRepository) ListOperationLogs(ctx context.Context, actorUserID uint64, page, pageSize int) ([]model.OperationLog, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	base := r.db.WithContext(ctx).Model(&model.OperationLog{}).Where("actor_user_id = ?", actorUserID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.OperationLog
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
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
