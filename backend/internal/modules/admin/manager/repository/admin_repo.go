package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/model"
)

// AdminRepository 员工账号数据访问。
type AdminRepository interface {
	Create(ctx context.Context, admin *model.Admin) error
	Update(ctx context.Context, admin *model.Admin) error
	Delete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*model.Admin, error)
	FindByUsername(ctx context.Context, username string) (*model.Admin, error)
	List(ctx context.Context, query dto.AdminListQuery) ([]model.Admin, int64, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	UpdatePassword(ctx context.Context, id uint64, passwordHash string) error
	// UpdatePasswordAndFlag 更新密码并同步强制改密标记（重置=true / 自助改密=false）。
	UpdatePasswordAndFlag(ctx context.Context, id uint64, passwordHash string, mustChange bool) error
	UpdateLoginProfile(ctx context.Context, id uint64, ip string, loginAt time.Time) error
	// —— S1 员工体系 ——
	// UpdateDepartmentID 单独改部门归属（部门删除/员工批量转移用）。
	UpdateDepartmentID(ctx context.Context, id uint64, departmentID uint64) error
	// Resign 置离职：写 resigned_at 并把状态改为 disabled（保留账号数据用于审计与提成追溯）。
	Resign(ctx context.Context, id uint64, resignedAt time.Time) error
	// ListByDepartment 取某部门在职员工（派单候选 / 销售归属候选）。
	ListByDepartment(ctx context.Context, departmentID uint64) ([]model.Admin, error)
	// ListSalesCandidates 取开启销售能力且在职的员工（销售分配下拉）。
	ListSalesCandidates(ctx context.Context) ([]model.Admin, error)
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
	// role 走 admin_roles 联表：admins.role 只是兼容展示列，鉴权已改读角色绑定，
	// 按兼容列筛选会与实际权限不一致（doc86 §3.1）。
	if role := strings.TrimSpace(query.Role); role != "" {
		base = base.Where(
			"EXISTS (SELECT 1 FROM admin_roles ar JOIN roles r ON r.id = ar.role_id AND r.status = 'active' WHERE ar.admin_id = admins.id AND r.code = ?)",
			role,
		)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if departmentID := query.DepartmentID; departmentID > 0 {
		base = base.Where("department_id = ?", departmentID)
	}
	if staffType := strings.TrimSpace(query.StaffType); staffType != "" {
		base = base.Where("staff_type = ?", staffType)
	}
	switch strings.TrimSpace(query.SalesEnabled) {
	case "1":
		base = base.Where("sales_enabled = ?", true)
	case "0":
		base = base.Where("sales_enabled = ?", false)
	}
	switch strings.TrimSpace(query.IsResigned) {
	case "1":
		base = base.Where("resigned_at IS NOT NULL")
	case "0":
		base = base.Where("resigned_at IS NULL")
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where(
			"username ILIKE ? OR email ILIKE ? OR department ILIKE ? OR real_name ILIKE ? OR phone ILIKE ?",
			like, like, like, like, like,
		)
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

func (r *adminRepository) UpdatePasswordAndFlag(ctx context.Context, id uint64, passwordHash string, mustChange bool) error {
	return r.db.WithContext(ctx).Model(&model.Admin{}).Where("id = ?", id).Updates(map[string]any{
		"password_hash":        passwordHash,
		"must_change_password": mustChange,
	}).Error
}

func (r *adminRepository) UpdateLoginProfile(ctx context.Context, id uint64, ip string, loginAt time.Time) error {
	updates := map[string]any{
		"last_login_at": loginAt,
		"last_login_ip": ip,
	}
	return r.db.WithContext(ctx).Model(&model.Admin{}).Where("id = ?", id).Updates(updates).Error
}

func (r *adminRepository) UpdateDepartmentID(ctx context.Context, id uint64, departmentID uint64) error {
	return r.db.WithContext(ctx).Model(&model.Admin{}).Where("id = ?", id).Update("department_id", departmentID).Error
}

// Resign 置离职：写 resigned_at 并禁用账号。不清空角色绑定，
// 保证历史工单/提成仍能解析出归属人（doc86 §2.1）。
func (r *adminRepository) Resign(ctx context.Context, id uint64, resignedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Admin{}).Where("id = ?", id).Updates(map[string]any{
		"resigned_at": resignedAt,
		"status":      "disabled",
	}).Error
}

func (r *adminRepository) ListByDepartment(ctx context.Context, departmentID uint64) ([]model.Admin, error) {
	var admins []model.Admin
	if err := r.db.WithContext(ctx).
		Where("department_id = ? AND status = ? AND resigned_at IS NULL", departmentID, "active").
		Order("id ASC").Find(&admins).Error; err != nil {
		return nil, err
	}
	return admins, nil
}

func (r *adminRepository) ListSalesCandidates(ctx context.Context) ([]model.Admin, error) {
	var admins []model.Admin
	if err := r.db.WithContext(ctx).
		Where("sales_enabled = ? AND status = ? AND resigned_at IS NULL", true, "active").
		Order("department_id ASC, id ASC").Find(&admins).Error; err != nil {
		return nil, err
	}
	return admins, nil
}
