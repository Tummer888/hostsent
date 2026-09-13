// Package repository 提供销售归属子域的数据访问实现。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/sales/model"
)

// RelationFilter 归属关系列表查询条件。
type RelationFilter struct {
	// AdminID 限定归属销售；0 表示不限（主管/超管视角）。
	AdminID uint64
	// DepartmentID 限定该销售所属部门；0 表示不限。由服务端按鉴权范围注入。
	DepartmentID uint64
	Keyword      string // 用户名/邮箱/手机模糊
	Status       string // active/released，空表示 active
	OnlyAssigned int    // 1=仅已归属；0=全部；-1 未显式指定（按 Status 处理）
	Page         int
	PageSize     int
}

// RelationRow 归属关系展示行：带出客户与销售信息。
type RelationRow struct {
	ID            uint64     `gorm:"column:id"`
	AdminID       uint64     `gorm:"column:admin_id"`
	AdminName     string     `gorm:"column:admin_name"`
	AdminRealName string     `gorm:"column:admin_real_name"`
	Department    string     `gorm:"column:department_name"`
	UserID        uint64     `gorm:"column:user_id"`
	Username      string     `gorm:"column:username"`
	Email         string     `gorm:"column:email"`
	Phone         string     `gorm:"column:phone"`
	Status        string     `gorm:"column:status"`
	Reason        string     `gorm:"column:reason"`
	OperatorID    uint64     `gorm:"column:operator_id"`
	ProtectAt     *time.Time `gorm:"column:protect_until"`
	EffectiveAt   *time.Time `gorm:"column:effective_at"`
	ReleasedAt    *time.Time `gorm:"column:released_at"`
	TotalConsume  float64    `gorm:"column:total_consume_amount"`
	UserCreated   *time.Time `gorm:"column:user_created_at"`
	CreatedAt     *time.Time `gorm:"column:created_at"`
}

// SalesCandidateRow 可被分配客户的销售（sales_enabled 且在职）。
type SalesCandidateRow struct {
	AdminID        uint64 `gorm:"column:admin_id"`
	Username       string `gorm:"column:username"`
	RealName       string `gorm:"column:real_name"`
	DepartmentID   uint64 `gorm:"column:department_id"`
	DepartmentName string `gorm:"column:department_name"`
	// ActiveCustomers 只读：当前名下现役客户数，供分配时判断负载。
	ActiveCustomers int64 `gorm:"column:active_customers"`
}

// UnassignedRow 未归属客户池行。
type UnassignedRow struct {
	UserID       uint64     `gorm:"column:user_id"`
	Username     string     `gorm:"column:username"`
	Email        string     `gorm:"column:email"`
	Phone        string     `gorm:"column:phone"`
	TotalConsume float64    `gorm:"column:total_consume_amount"`
	CreatedAt    *time.Time `gorm:"column:created_at"`
}

// RelationRepository 归属关系数据访问。
type RelationRepository interface {
	// FindActiveByUser 返回客户现役归属；无归属返回 (nil, nil)。
	FindActiveByUser(ctx context.Context, userID uint64) (*model.StaffSalesRelation, error)
	// LockActiveByUser 行锁读取现役归属（改派事务内使用）。
	LockActiveByUser(db *gorm.DB, userID uint64) (*model.StaffSalesRelation, error)
	Create(db *gorm.DB, rel *model.StaffSalesRelation) error
	// Release 把指定归属置为已释放（release 事务内使用）。
	Release(db *gorm.DB, id uint64, operatorID uint64, reason string, at time.Time) error
	// SetUserOwner 回写 users.sales_admin_id 快照。
	SetUserOwner(db *gorm.DB, userID, adminID uint64) error
	// ListRows 归属客户列表（已归属视图）。
	ListRows(ctx context.Context, f RelationFilter) ([]RelationRow, int64, error)
	// ListUnassigned 未归属客户池（users.sales_admin_id = 0）。
	ListUnassigned(ctx context.Context, keyword string, page, pageSize int) ([]UnassignedRow, int64, error)
	// ListByUser 客户归属变更历史（时间倒序）。
	ListByUser(ctx context.Context, userID uint64, limit int) ([]RelationRow, error)
	// CountActiveByAdmin 某销售的现役客户数（离职转派与业绩概览用）。
	CountActiveByAdmin(ctx context.Context, adminID uint64) (int64, error)
	// CountActiveByDepartment 某部门现役归属客户数（部门概览用）。
	CountActiveByDepartment(ctx context.Context, departmentID uint64) (int64, error)
	// CountNewByAdminInPeriod 某销售某月新增归属客户数（业绩概览用），period 为 YYYY-MM。
	CountNewByAdminInPeriod(ctx context.Context, adminID uint64, period string) (int64, error)
	// CountSalesInDepartment 某部门在职销售人数（排行分母）。
	CountSalesInDepartment(ctx context.Context, departmentID uint64) (int64, error)
	// DepartmentOfAdmin 读员工所属部门（0 表示未分配）。
	DepartmentOfAdmin(ctx context.Context, adminID uint64) (uint64, error)
	// ListActiveUserIDsByAdmin 某销售名下现役客户 ID 列表（离职转派用）。
	ListActiveUserIDsByAdmin(ctx context.Context, adminID uint64) ([]uint64, error)
	// FindSalesCandidate 按 ID 校验目标销售是否可用（在职且开启销售能力）。
	FindSalesCandidate(ctx context.Context, adminID uint64) (*SalesCandidateRow, error)
	// AdminBrief 读员工展示信息（不校验销售能力，用于概览/台账展示）。
	AdminBrief(ctx context.Context, adminID uint64) (*SalesCandidateRow, error)
	// ListSalesCandidates 销售下拉：在职且开启销售能力，可按部门过滤。
	ListSalesCandidates(ctx context.Context, departmentID uint64) ([]SalesCandidateRow, error)
	// FindUserOwner 读 users.sales_admin_id 快照（0 表示未归属）。
	FindUserOwner(ctx context.Context, userID uint64) (uint64, error)
	// UserExists 判断客户是否存在且为主账号。
	UserExists(ctx context.Context, userID uint64) (bool, error)
}

type relationRepository struct {
	db *gorm.DB
}

// NewRelationRepository 创建归属关系仓储。
func NewRelationRepository(db *gorm.DB) RelationRepository {
	return &relationRepository{db: db}
}

func (r *relationRepository) FindActiveByUser(ctx context.Context, userID uint64) (*model.StaffSalesRelation, error) {
	var rel model.StaffSalesRelation
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, model.RelationStatusActive).
		Order("id desc").First(&rel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (r *relationRepository) LockActiveByUser(db *gorm.DB, userID uint64) (*model.StaffSalesRelation, error) {
	var rel model.StaffSalesRelation
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND status = ?", userID, model.RelationStatusActive).
		Order("id desc").First(&rel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (r *relationRepository) Create(db *gorm.DB, rel *model.StaffSalesRelation) error {
	return db.Create(rel).Error
}

func (r *relationRepository) Release(db *gorm.DB, id uint64, operatorID uint64, reason string, at time.Time) error {
	return db.Model(&model.StaffSalesRelation{}).Where("id = ?", id).Updates(map[string]any{
		"status":      model.RelationStatusReleased,
		"released_at": at,
		"operator_id": operatorID,
		"reason":      reason,
	}).Error
}

func (r *relationRepository) SetUserOwner(db *gorm.DB, userID, adminID uint64) error {
	return db.Table("users").Where("id = ?", userID).Update("sales_admin_id", adminID).Error
}

// ListRows 已归属客户列表：以 users 为基表左连现役归属，保证「已归属但关系表缺行」的脏数据也能被发现。
func (r *relationRepository) ListRows(ctx context.Context, f RelationFilter) ([]RelationRow, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	base := r.db.WithContext(ctx).Table("users u").
		Joins("JOIN staff_sales_relations ssr ON ssr.user_id = u.id AND ssr.status = ?", model.RelationStatusActive).
		Joins("LEFT JOIN admins a ON a.id = ssr.admin_id").
		Joins("LEFT JOIN departments d ON d.id = a.department_id")
	if f.AdminID > 0 {
		base = base.Where("ssr.admin_id = ?", f.AdminID)
	}
	if f.DepartmentID > 0 {
		base = base.Where("a.department_id = ?", f.DepartmentID)
	}
	if kw := likeKeyword(f.Keyword); kw != "" {
		base = base.Where("(u.username ILIKE ? OR u.email ILIKE ? OR u.phone ILIKE ?)", kw, kw, kw)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []RelationRow
	err := base.Select(`ssr.id, ssr.admin_id, ssr.user_id, ssr.status, ssr.reason, ssr.operator_id,
			ssr.protect_until, ssr.effective_at, ssr.released_at, ssr.created_at,
			COALESCE(a.username, '') AS admin_name,
			COALESCE(a.real_name, '') AS admin_real_name,
			COALESCE(d.name, '') AS department_name,
			u.username, u.email, COALESCE(u.phone, '') AS phone,
			u.total_consume_amount, u.created_at AS user_created_at`).
		Order("ssr.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *relationRepository) ListUnassigned(ctx context.Context, keyword string, page, pageSize int) ([]UnassignedRow, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	// 排除子账号：归属只落在主账号上，子账号随主账号消费。
	base := r.db.WithContext(ctx).Table("users u").
		Where("COALESCE(u.sales_admin_id, 0) = 0").
		Where("u.is_sub_account = ?", false)
	if kw := likeKeyword(keyword); kw != "" {
		base = base.Where("(u.username ILIKE ? OR u.email ILIKE ? OR u.phone ILIKE ?)", kw, kw, kw)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []UnassignedRow
	err := base.Select(`u.id AS user_id, u.username, u.email, COALESCE(u.phone, '') AS phone,
			u.total_consume_amount, u.created_at`).
		Order("u.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListByUser 归属变更历史：含已释放记录，按生效时间倒序。
func (r *relationRepository) ListByUser(ctx context.Context, userID uint64, limit int) ([]RelationRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []RelationRow
	err := r.db.WithContext(ctx).Table("staff_sales_relations ssr").
		Joins("LEFT JOIN admins a ON a.id = ssr.admin_id").
		Joins("LEFT JOIN departments d ON d.id = a.department_id").
		Select(`ssr.id, ssr.admin_id, ssr.user_id, ssr.status, ssr.reason, ssr.operator_id,
			ssr.protect_until, ssr.effective_at, ssr.released_at, ssr.created_at,
			COALESCE(a.username, '') AS admin_name,
			COALESCE(a.real_name, '') AS admin_real_name,
			COALESCE(d.name, '') AS department_name`).
		Where("ssr.user_id = ?", userID).
		Order("ssr.id desc").Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *relationRepository) CountActiveByAdmin(ctx context.Context, adminID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.StaffSalesRelation{}).
		Where("admin_id = ? AND status = ?", adminID, model.RelationStatusActive).Count(&n).Error
	return n, err
}

func (r *relationRepository) CountActiveByDepartment(ctx context.Context, departmentID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("staff_sales_relations ssr").
		Joins("JOIN admins a ON a.id = ssr.admin_id").
		Where("ssr.status = ? AND a.department_id = ?", model.RelationStatusActive, departmentID).
		Count(&n).Error
	return n, err
}

// CountNewByAdminInPeriod 某销售某月新增归属客户数（按关系生效时间）。
func (r *relationRepository) CountNewByAdminInPeriod(ctx context.Context, adminID uint64, period string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.StaffSalesRelation{}).
		Where("admin_id = ?", adminID).
		Where("to_char(effective_at, 'YYYY-MM') = ?", period).
		Count(&n).Error
	return n, err
}

func (r *relationRepository) CountSalesInDepartment(ctx context.Context, departmentID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("admins").
		Where("department_id = ? AND sales_enabled = ? AND resigned_at IS NULL AND status = ?",
			departmentID, true, "active").
		Count(&n).Error
	return n, err
}

func (r *relationRepository) DepartmentOfAdmin(ctx context.Context, adminID uint64) (uint64, error) {
	var dept *uint64
	err := r.db.WithContext(ctx).Table("admins").Select("department_id").
		Where("id = ?", adminID).Scan(&dept).Error
	if err != nil || dept == nil {
		return 0, err
	}
	return *dept, nil
}

func (r *relationRepository) ListActiveUserIDsByAdmin(ctx context.Context, adminID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.StaffSalesRelation{}).
		Where("admin_id = ? AND status = ?", adminID, model.RelationStatusActive).
		Pluck("user_id", &ids).Error
	return ids, err
}

// FindSalesCandidate 校验目标销售可用性：在职（resigned_at 为空）且开启销售能力。
// 直接查 admins 表而非引入 manager 仓储，避免 sales 模块反向依赖员工模块（同工单模块做法）。
func (r *relationRepository) FindSalesCandidate(ctx context.Context, adminID uint64) (*SalesCandidateRow, error) {
	var row SalesCandidateRow
	err := r.db.WithContext(ctx).Table("admins a").
		Joins("LEFT JOIN departments d ON d.id = a.department_id").
		Select(`a.id AS admin_id, a.username, COALESCE(a.real_name, '') AS real_name,
			a.department_id, COALESCE(d.name, '') AS department_name`).
		Where("a.id = ? AND a.sales_enabled = ? AND a.resigned_at IS NULL AND a.status = ?",
			adminID, true, "active").
		Limit(1).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.AdminID == 0 {
		return nil, nil
	}
	return &row, nil
}

// AdminBrief 读员工展示信息（不做销售能力校验）。
func (r *relationRepository) AdminBrief(ctx context.Context, adminID uint64) (*SalesCandidateRow, error) {
	if adminID == 0 {
		return nil, nil
	}
	var row SalesCandidateRow
	err := r.db.WithContext(ctx).Table("admins a").
		Joins("LEFT JOIN departments d ON d.id = a.department_id").
		Select(`a.id AS admin_id, a.username, COALESCE(a.real_name, '') AS real_name,
			a.department_id, COALESCE(d.name, '') AS department_name`).
		Where("a.id = ?", adminID).Limit(1).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.AdminID == 0 {
		return nil, nil
	}
	return &row, nil
}

// ListSalesCandidates 销售下拉：在职且开启销售能力，可选按部门过滤。
func (r *relationRepository) ListSalesCandidates(ctx context.Context, departmentID uint64) ([]SalesCandidateRow, error) {
	var rows []SalesCandidateRow
	q := r.db.WithContext(ctx).Table("admins a").
		Joins("LEFT JOIN departments d ON d.id = a.department_id").
		Select(`a.id AS admin_id, a.username, COALESCE(a.real_name, '') AS real_name,
			a.department_id, COALESCE(d.name, '') AS department_name,
			(SELECT COUNT(*) FROM staff_sales_relations s WHERE s.admin_id = a.id AND s.status = 'active') AS active_customers`).
		Where("a.sales_enabled = ? AND a.resigned_at IS NULL AND a.status = ?", true, "active")
	if departmentID > 0 {
		q = q.Where("a.department_id = ?", departmentID)
	}
	err := q.Order("a.id asc").Scan(&rows).Error
	return rows, err
}

func (r *relationRepository) FindUserOwner(ctx context.Context, userID uint64) (uint64, error) {
	var owner *uint64
	err := r.db.WithContext(ctx).Table("users").Select("sales_admin_id").
		Where("id = ?", userID).Scan(&owner).Error
	if err != nil || owner == nil {
		return 0, err
	}
	return *owner, nil
}

// UserExists 判断客户是否存在且为主账号（归属只挂在主账号上）。
func (r *relationRepository) UserExists(ctx context.Context, userID uint64) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("users").
		Where("id = ? AND is_sub_account = ?", userID, false).Count(&n).Error
	return n > 0, err
}

func likeKeyword(kw string) string {
	if kw == "" {
		return ""
	}
	return "%" + kw + "%"
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
