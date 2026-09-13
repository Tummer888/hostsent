// Package repository 提供工单域的数据访问实现。
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/model"
)

// TicketRepository 定义工单数据访问能力。
type TicketRepository interface {
	List(ctx context.Context, query dto.TicketListQuery) ([]model.Ticket, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Ticket, error)
	// FindByIDs 批量取工单（复核队列回填工单信息用，S3）。
	FindByIDs(ctx context.Context, ids []uint64) (map[uint64]model.Ticket, error)
	Create(ctx context.Context, item *model.Ticket) error
	Update(ctx context.Context, item *model.Ticket) error
	// CountByTicketIDs 批量统计工单回复数（列表展示）
	CountByTicketIDs(ctx context.Context, ids []uint64) (map[uint64]int64, error)
	// UserNamesByIDs 批量查询用户账号（工单列表展示提交人）
	UserNamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// AdminNameByID 查询管理员名称（分配时快照处理人）
	AdminNameByID(ctx context.Context, id uint64) (string, error)
	// —— 操作日志（P2-02）——
	// CreateLog 写入一条工单操作日志。
	CreateLog(ctx context.Context, log *model.TicketLog) error
	// ListLogs 按时间正序查询工单操作日志。
	ListLogs(ctx context.Context, ticketID uint64) ([]model.TicketLog, error)
	// —— 自动派单（P2-03/S2）——
	// PickAssignee 按部门挑当前在手工单最少的启用在职员工；departmentID=0 时回落到
	// 按 roleCode 挑人（兼容未配置部门的存量分类）；两者皆空返回 id=0（进未分配池）。
	PickAssignee(ctx context.Context, roleCode string, departmentID uint64) (uint64, string, error)
	// —— S2 数据范围与展示 ——
	// ListCategoriesByDepartment 取某部门下的启用分类 code（主管/客服数据范围）。
	ListCategoriesByDepartment(ctx context.Context, departmentID uint64) ([]string, error)
	// DepartmentNameMap 批量取部门 id → 名称。
	DepartmentNameMap(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// AdminNameMap 批量取管理员 id → 展示名（real_name 优先，回落 username）。
	AdminNameMap(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// ReviewerIDs 取某部门的在岗复核人 ID（support_lead 角色，S3）。
	// departmentID=0 时返回全部 support_lead（无部门归属的存量数据不至于收不到复核通知）。
	ReviewerIDs(ctx context.Context, departmentID uint64) ([]uint64, error)
	// —— 子账号归属（P4-05）——
	// AccountUserIDs 返回账号及其全部子账号 ID（主账号在前）；无子账号时仅返回自身。
	AccountUserIDs(ctx context.Context, accountID uint64) ([]uint64, error)
	// —— 统计 ——
	// StatsTotal 工单总量
	StatsTotal(ctx context.Context) (int64, error)
	// CountByStatus 工单状态分布
	CountByStatus(ctx context.Context) ([]model.TicketStatusCount, error)
	// AvgFirstReplySeconds 平均首次响应时长（秒）
	AvgFirstReplySeconds(ctx context.Context) (int64, error)
	// StatsTrend 近 N 日工单量趋势
	StatsTrend(ctx context.Context, days int) ([]model.TicketTrendPoint, error)
	// CountByCategory 分类工单分布
	CountByCategory(ctx context.Context) ([]model.TicketCategoryStat, error)
}

type ticketRepository struct {
	db *gorm.DB
}

// NewTicketRepository 创建工单仓储实现。
func NewTicketRepository(db *gorm.DB) TicketRepository {
	return &ticketRepository{db: db}
}

func (r *ticketRepository) List(ctx context.Context, query dto.TicketListQuery) ([]model.Ticket, int64, error) {
	page := normalizePage(query.Page)
	pageSize := normalizePageSize(query.PageSize)

	base := r.db.WithContext(ctx).Model(&model.Ticket{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("ticket_no ILIKE ? OR title ILIKE ?", like, like)
	}
	if userKeyword := strings.TrimSpace(query.UserKeyword); userKeyword != "" {
		like := "%" + userKeyword + "%"
		base = base.Joins("JOIN users ON users.id = tickets.user_id").
			Where("users.username ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ?", like, like, like)
	}
	if query.UserID > 0 {
		base = base.Where("tickets.user_id = ?", query.UserID)
	}
	// 账号家族范围（P4-05）：主账号能看到自己与全部子账号提交的工单。
	if len(query.UserIDs) > 0 {
		base = base.Where("tickets.user_id IN ?", query.UserIDs)
	}
	if category := strings.TrimSpace(query.Category); category != "" {
		base = base.Where("tickets.category = ?", category)
	}
	if query.Priority != "" {
		base = base.Where("tickets.priority = ?", query.Priority)
	}
	if query.Status != "" {
		base = base.Where("tickets.status = ?", query.Status)
	}
	if query.ReviewStatus != "" {
		base = base.Where("tickets.review_status = ?", query.ReviewStatus)
	}
	if query.DepartmentID > 0 {
		base = base.Where("tickets.department_id = ?", query.DepartmentID)
	}
	if query.AssignedTo > 0 {
		base = base.Where("tickets.assigned_to = ?", query.AssignedTo)
	}
	// 数据范围隔离（S2）：主管按本部门分类可见，客服再并上「指派给我的」。
	// OrAssignee>0 时用 OR 括起来，保证不放大其它筛选条件的约束。
	if len(query.VisibleCategories) > 0 {
		if query.OrAssignee > 0 {
			base = base.Where("(tickets.category IN ? OR tickets.assigned_to = ?)", query.VisibleCategories, query.OrAssignee)
		} else {
			base = base.Where("tickets.category IN ?", query.VisibleCategories)
		}
	} else if query.OrAssignee > 0 {
		base = base.Where("tickets.assigned_to = ?", query.OrAssignee)
	}
	// 工单工作台视图（P2-07）：my_todo / unassigned / involved / sla_breached
	switch query.View {
	case "my_todo":
		if query.AdminID > 0 {
			base = base.Where("tickets.assigned_to = ?", query.AdminID).
				Where("tickets.status IN ?", []string{model.TicketStatusOpen, model.TicketStatusInProgress, model.TicketStatusWaitingUser})
		}
	case "unassigned":
		base = base.Where("tickets.assigned_to = 0").
			Where("tickets.status NOT IN ?", []string{model.TicketStatusClosed, model.TicketStatusCancelled})
	case "involved":
		if query.AdminID > 0 {
			base = base.Where(
				"tickets.assigned_to = ? OR EXISTS (SELECT 1 FROM ticket_replies tr WHERE tr.ticket_id = tickets.id AND tr.sender_type = ? AND tr.sender_id = ?)",
				query.AdminID, model.SenderTypeAdmin, query.AdminID,
			)
		}
	case "sla_breached":
		// 未首次响应且已超分类 SLA 时限
		base = base.Joins("JOIN ticket_categories c ON c.code = tickets.category").
			Select("tickets.*").
			Where("c.sla_hours > 0 AND tickets.first_reply_at IS NULL").
			Where("tickets.created_at < NOW() - (c.sla_hours * INTERVAL '1 hour')")
	case "review_pending":
		// 有回复待复核（S3 复核中心入口）
		base = base.Where("tickets.review_status = ?", model.ReviewStatusPending)
	}
	if start := normalizeTime(query.StartTime); start != nil {
		base = base.Where("tickets.created_at >= ?", *start)
	}
	if end := normalizeTime(query.EndTime); end != nil {
		base = base.Where("tickets.created_at <= ?", *end)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Ticket
	if err := base.Order("tickets.created_at desc, tickets.id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *ticketRepository) FindByID(ctx context.Context, id uint64) (*model.Ticket, error) {
	var item model.Ticket
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByIDs 批量取工单，返回 id → 工单映射（复核队列回填用）。
func (r *ticketRepository) FindByIDs(ctx context.Context, ids []uint64) (map[uint64]model.Ticket, error) {
	out := make(map[uint64]model.Ticket, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var items []model.Ticket
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		out[item.ID] = item
	}
	return out, nil
}

// AccountUserIDs 返回账号及其全部子账号 ID（主账号在前），供用户端按归属查询（P4-05）。
func (r *ticketRepository) AccountUserIDs(ctx context.Context, accountID uint64) ([]uint64, error) {
	if accountID == 0 {
		return nil, nil
	}
	var subIDs []uint64
	if err := r.db.WithContext(ctx).
		Table("users").
		Where("owner_user_id = ?", accountID).
		Pluck("id", &subIDs).Error; err != nil {
		return nil, err
	}
	return append([]uint64{accountID}, subIDs...), nil
}

func (r *ticketRepository) Create(ctx context.Context, item *model.Ticket) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ticketRepository) Update(ctx context.Context, item *model.Ticket) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ticketRepository) CountByTicketIDs(ctx context.Context, ids []uint64) (map[uint64]int64, error) {
	result := make(map[uint64]int64, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		TicketID uint64 `gorm:"column:ticket_id"`
		Count    int64  `gorm:"column:count"`
	}
	if err := r.db.WithContext(ctx).Model(&model.TicketReply{}).
		Select("ticket_id, count(*) as count").
		Where("ticket_id IN ?", ids).
		Group("ticket_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.TicketID] = row.Count
	}
	return result, nil
}

func (r *ticketRepository) UserNamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	result := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Table("users").
		Select("id, username").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Username
	}
	return result, nil
}

func (r *ticketRepository) AdminNameByID(ctx context.Context, id uint64) (string, error) {
	var row struct {
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Table("admins").
		Select("username").
		Where("id = ?", id).
		Scan(&row).Error; err != nil {
		return "", err
	}
	return row.Username, nil
}

// CreateLog 写入一条工单操作日志。
func (r *ticketRepository) CreateLog(ctx context.Context, log *model.TicketLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ListLogs 按时间正序查询工单操作日志。
func (r *ticketRepository) ListLogs(ctx context.Context, ticketID uint64) ([]model.TicketLog, error) {
	var items []model.TicketLog
	if err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at asc, id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// PickAssignee 挑当前在手工单最少的启用在职员工（S2：按部门派单）。
// 优先 departmentID（分类挂部门后为主路径）；departmentID=0 时回落到 roleCode
// （兼容未配置部门的存量分类，保持旧行为可用）；两者皆空返回 0（进未分配池）。
func (r *ticketRepository) PickAssignee(ctx context.Context, roleCode string, departmentID uint64) (uint64, string, error) {
	if roleCode == "" && departmentID == 0 {
		return 0, "", nil
	}
	type row struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
		Load     int64  `gorm:"column:load"`
	}
	var candidates []row
	query := r.db.WithContext(ctx).Table("admins a").
		Select("a.id AS id, a.username AS username, COUNT(t.id) AS load").
		Joins("LEFT JOIN tickets t ON t.assigned_to = a.id AND t.status IN ?", []string{model.TicketStatusOpen, model.TicketStatusInProgress, model.TicketStatusWaitingUser}).
		// 离职员工不再接单（resigned_at 非空），避免派给已离场的人。
		Where("a.status = ? AND a.resigned_at IS NULL", "active")
	if departmentID > 0 {
		query = query.Where("a.department_id = ?", departmentID)
	}
	if roleCode != "" {
		query = query.Joins("JOIN admin_roles ar ON ar.admin_id = a.id").
			Joins("JOIN roles r ON r.id = ar.role_id").
			Where("r.code = ?", roleCode)
	}
	if err := query.Group("a.id, a.username").Order("load asc, a.id asc").Limit(1).Scan(&candidates).Error; err != nil {
		return 0, "", err
	}
	if len(candidates) == 0 {
		return 0, "", nil
	}
	return candidates[0].ID, candidates[0].Username, nil
}

// ListCategoriesByDepartment 取某部门下的启用分类 code（数据范围隔离用，S2）。
func (r *ticketRepository) ListCategoriesByDepartment(ctx context.Context, departmentID uint64) ([]string, error) {
	if departmentID == 0 {
		return nil, nil
	}
	var codes []string
	if err := r.db.WithContext(ctx).Model(&model.TicketCategory{}).
		Where("department_id = ? AND status = ?", departmentID, model.CategoryStatusActive).
		Pluck("code", &codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// DepartmentNameMap 批量取部门 id → 名称（列表展示，S2）。
func (r *ticketRepository) DepartmentNameMap(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   uint64 `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := r.db.WithContext(ctx).Table("departments").
		Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

// AdminNameMap 批量取管理员 id → 展示名（优先 real_name，回落 username）。
func (r *ticketRepository) AdminNameMap(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
		RealName string `gorm:"column:real_name"`
	}
	if err := r.db.WithContext(ctx).Table("admins").
		Select("id, username, real_name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.RealName != "" {
			out[row.ID] = row.RealName
			continue
		}
		out[row.ID] = row.Username
	}
	return out, nil
}

// ReviewerIDs 取某部门的在岗复核人（support_lead）。离职/禁用员工不接收通知。
func (r *ticketRepository) ReviewerIDs(ctx context.Context, departmentID uint64) ([]uint64, error) {
	var ids []uint64
	query := r.db.WithContext(ctx).Table("admins a").
		Joins("JOIN admin_roles ar ON ar.admin_id = a.id").
		Joins("JOIN roles r ON r.id = ar.role_id AND r.status = 'active'").
		Where("r.code = ? AND a.status = ? AND a.resigned_at IS NULL", "support_lead", "active")
	if departmentID > 0 {
		query = query.Where("a.department_id = ?", departmentID)
	}
	if err := query.Distinct().Pluck("a.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *ticketRepository) StatsTotal(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Ticket{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *ticketRepository) CountByStatus(ctx context.Context) ([]model.TicketStatusCount, error) {
	var counts []model.TicketStatusCount
	if err := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	return counts, nil
}

func (r *ticketRepository) AvgFirstReplySeconds(ctx context.Context) (int64, error) {
	var avg float64
	err := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (first_reply_at - created_at))), 0)").
		Where("first_reply_at IS NOT NULL").
		Scan(&avg).Error
	if err != nil {
		return 0, err
	}
	return int64(avg + 0.5), nil
}

func (r *ticketRepository) StatsTrend(ctx context.Context, days int) ([]model.TicketTrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	start := dayStart(time.Now()).AddDate(0, 0, -(days - 1))
	var points []model.TicketTrendPoint
	if err := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Select("to_char(created_at, 'YYYY-MM-DD') as date, COUNT(*) as count").
		Where("created_at >= ?", start).
		Group("date").
		Order("date asc").
		Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

func (r *ticketRepository) CountByCategory(ctx context.Context) ([]model.TicketCategoryStat, error) {
	var stats []model.TicketCategoryStat
	if err := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Select("category, count(*) as count").
		Group("category").
		Order("count desc").
		Scan(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}
