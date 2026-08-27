package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/model"
)

type AdminRepository interface {
	Create(ctx context.Context, admin *model.Admin) error
	Update(ctx context.Context, admin *model.Admin) error
	Delete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*model.Admin, error)
	FindByUsername(ctx context.Context, username string) (*model.Admin, error)
	List(ctx context.Context, query dto.AdminListQuery) ([]model.Admin, int64, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	UpdatePassword(ctx context.Context, id uint64, passwordHash string) error
	UpdateLoginProfile(ctx context.Context, id uint64, ip string, loginAt time.Time) error
}

type adminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) Create(ctx context.Context, admin *model.Admin) error {
	return r.db.WithContext(ctx).Omit("ID").Create(admin).Error
}

func (r *adminRepository) Update(ctx context.Context, admin *model.Admin) error {
	return r.db.WithContext(ctx).Save(admin).Error
}

func (r *adminRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Admin{}, id).Error
}

func (r *adminRepository) FindByID(ctx context.Context, id uint64) (*model.Admin, error) {
	var admin model.Admin
	if err := r.db.WithContext(ctx).First(&admin, id).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *adminRepository) FindByUsername(ctx context.Context, username string) (*model.Admin, error) {
	var admin model.Admin
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *adminRepository) List(ctx context.Context, query dto.AdminListQuery) ([]model.Admin, int64, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	base := r.db.WithContext(ctx).Model(&model.Admin{})
	if role := strings.TrimSpace(query.Role); role != "" {
		base = base.Where("role = ?", role)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("username ILIKE ? OR email ILIKE ? OR department ILIKE ?", like, like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var admins []model.Admin
	if err := base.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&admins).Error; err != nil {
		return nil, 0, err
	}

	return admins, total, nil
}

func (r *adminRepository) UpdateStatus(ctx context.Context, id uint64, status string) error {
	return r.db.WithContext(ctx).Model(&model.Admin{}).Where("id = ?", id).Update("status", status).Error
}

func (r *adminRepository) UpdatePassword(ctx context.Context, id uint64, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.Admin{}).Where("id = ?", id).Update("password_hash", passwordHash).Error
}

func (r *adminRepository) UpdateLoginProfile(ctx context.Context, id uint64, ip string, loginAt time.Time) error {
	updates := map[string]any{
		"last_login_at": loginAt,
		"last_login_ip": ip,
	}
	return r.db.WithContext(ctx).Model(&model.Admin{}).Where("id = ?", id).Updates(updates).Error
}
