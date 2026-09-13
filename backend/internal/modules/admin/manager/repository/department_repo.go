package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/model"
)

// DepartmentRepository 组织部门数据访问（S1 员工体系）。
type DepartmentRepository interface {
	Create(ctx context.Context, dept *model.Department) error
	Update(ctx context.Context, dept *model.Department) error
	Delete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*model.Department, error)
	FindByCode(ctx context.Context, code string) (*model.Department, error)
	FindByIDs(ctx context.Context, ids []uint64) ([]model.Department, error)
	List(ctx context.Context, query dto.DepartmentListQuery) ([]model.Department, error)
	// ListByKind 按部门类型取启用中的部门（派单/销售候选范围用）。
	ListByKind(ctx context.Context, kind string) ([]model.Department, error)
	// CountActiveAdmins 统计在职员工数（删除前置校验）。
	CountActiveAdmins(ctx context.Context, departmentID uint64) (int64, error)
	// CountCategoryRefs 统计引用该部门的工单分类数（删除前置校验）。
	CountCategoryRefs(ctx context.Context, departmentID uint64) (int64, error)
	// NameMap 批量取 id → 名称，供列表/详情拼接只读展示字段。
	NameMap(ctx context.Context, ids []uint64) (map[uint64]string, error)
}

type departmentRepository struct {
	db *gorm.DB
}

// NewDepartmentRepository 创建部门仓储实现。
func NewDepartmentRepository(db *gorm.DB) DepartmentRepository {
	return &departmentRepository{db: db}
}

func (r *departmentRepository) Create(ctx context.Context, dept *model.Department) error {
	return r.db.WithContext(ctx).Omit("ID").Create(dept).Error
}

func (r *departmentRepository) Update(ctx context.Context, dept *model.Department) error {
	return r.db.WithContext(ctx).Save(dept).Error
}

func (r *departmentRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Department{}, id).Error
}

func (r *departmentRepository) FindByID(ctx context.Context, id uint64) (*model.Department, error) {
	var dept model.Department
	if err := r.db.WithContext(ctx).First(&dept, id).Error; err != nil {
		return nil, err
	}
	return &dept, nil
}

func (r *departmentRepository) FindByCode(ctx context.Context, code string) (*model.Department, error) {
	var dept model.Department
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&dept).Error; err != nil {
		return nil, err
	}
	return &dept, nil
}

func (r *departmentRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.Department, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var depts []model.Department
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Order("sort_order ASC, id ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

func (r *departmentRepository) List(ctx context.Context, query dto.DepartmentListQuery) ([]model.Department, error) {
	base := r.db.WithContext(ctx).Model(&model.Department{})
	if kind := strings.TrimSpace(query.Kind); kind != "" {
		base = base.Where("kind = ?", kind)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ?", like, like)
	}
	var depts []model.Department
	if err := base.Order("sort_order ASC, id ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

func (r *departmentRepository) ListByKind(ctx context.Context, kind string) ([]model.Department, error) {
	var depts []model.Department
	if err := r.db.WithContext(ctx).
		Where("kind = ? AND status = ?", kind, "active").
		Order("sort_order ASC, id ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

func (r *departmentRepository) CountActiveAdmins(ctx context.Context, departmentID uint64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("admins").
		Where("department_id = ? AND resigned_at IS NULL", departmentID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *departmentRepository) CountCategoryRefs(ctx context.Context, departmentID uint64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("ticket_categories").
		Where("department_id = ?", departmentID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *departmentRepository) NameMap(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string)
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   uint64 `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := r.db.WithContext(ctx).
		Model(&model.Department{}).
		Select("id, name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}
