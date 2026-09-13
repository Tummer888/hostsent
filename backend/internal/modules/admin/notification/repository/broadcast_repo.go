package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// BroadcastUser 群发目标用户（最小字段集）。
type BroadcastUser struct {
	ID       uint64
	Username string
	Email    string
	Phone    string
}

// BroadcastRepository 群发目标检索与批量投递写入。
//
// 本接口只读 users 表、只写投递队列表，不复用 /admin/users 的权限，
// 避免「客服要群发就必须有全站用户列表权限」的权限放大。
type BroadcastRepository interface {
	// CountTargets 命中人数（预览与上限校验）。
	CountTargets(ctx context.Context, target notifydto.BroadcastTarget) (int64, error)
	// StreamTargets 分批遍历命中用户（每批回调一次），避免一次性载入全量。
	StreamTargets(ctx context.Context, target notifydto.BroadcastTarget, batch int, fn func([]BroadcastUser) error) error
	// SampleTargets 取前 N 条用于预览（联系方式打码由服务层做）。
	SampleTargets(ctx context.Context, target notifydto.BroadcastTarget, limit int) ([]BroadcastUser, error)
	// ListGroups 用户组选项（含成员数）。
	ListGroups(ctx context.Context) ([]notifydto.BroadcastGroupItem, error)
	// SearchUsers 按关键词检索用户（分页）。
	SearchUsers(ctx context.Context, keyword string, page, size int) ([]BroadcastUser, int64, error)
	// CountBatch 统计批次投递状态分布。
	CountBatch(ctx context.Context, batchID string) (map[string]int64, error)
}

type broadcastRepository struct {
	db *gorm.DB
}

// NewBroadcastRepository 创建群发仓储。
func NewBroadcastRepository(db *gorm.DB) BroadcastRepository {
	return &broadcastRepository{db: db}
}

// buildUserScope 把目标定义编译成 users 表查询。
//
// 状态默认只发 active 用户：给已禁用/已注销的账号群发短信既浪费配额也无意义，
// 除非调用方在 filter 里显式指定 status。
func (r *broadcastRepository) buildUserScope(ctx context.Context, target notifydto.BroadcastTarget) *gorm.DB {
	q := r.db.WithContext(ctx).Table("users")
	switch target.Mode {
	case "group":
		q = q.Where("user_group_id = ?", target.UserGroupID)
	case "users":
		if len(target.UserIDs) == 0 {
			return q.Where("1 = 0")
		}
		q = q.Where("id IN ?", target.UserIDs)
	case "filter":
		q = applyBroadcastFilter(q, target.Filter)
	case "all":
		// 全部用户；仍受「非子账号 + 状态」默认约束。
		q = q.Where("COALESCE(is_sub_account, false) = false")
	}
	if target.Mode != "filter" {
		q = q.Where("status = ?", "active")
		q = q.Where("COALESCE(is_sub_account, false) = false")
	}
	return q
}

func applyBroadcastFilter(q *gorm.DB, f *notifydto.BroadcastFilter) *gorm.DB {
	if f == nil {
		return q.Where("status = ?", "active")
	}
	if f.RegisteredAfter != "" {
		q = q.Where("created_at >= ?", f.RegisteredAfter)
	}
	if f.RegisteredBefore != "" {
		q = q.Where("created_at <= ?", f.RegisteredBefore)
	}
	if f.Tier != "" {
		q = q.Where("tier = ?", f.Tier)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	} else {
		q = q.Where("status = ?", "active")
	}
	if f.HasInstance != nil {
		sub := r_subqueryInstances(*f.HasInstance)
		q = q.Where(sub)
	}
	if f.MinBalance != "" {
		q = q.Where("COALESCE(total_consume_amount, 0) >= ?", f.MinBalance)
	}
	if f.MaxBalance != "" {
		q = q.Where("COALESCE(total_consume_amount, 0) <= ?", f.MaxBalance)
	}
	return q
}

// r_subqueryInstances 生成「是否有实例」的子查询条件。
func r_subqueryInstances(has bool) string {
	if has {
		return "EXISTS (SELECT 1 FROM instances i WHERE i.user_id = users.id)"
	}
	return "NOT EXISTS (SELECT 1 FROM instances i WHERE i.user_id = users.id)"
}

func (r *broadcastRepository) CountTargets(ctx context.Context, target notifydto.BroadcastTarget) (int64, error) {
	var count int64
	if err := r.buildUserScope(ctx, target).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *broadcastRepository) SampleTargets(ctx context.Context, target notifydto.BroadcastTarget, limit int) ([]BroadcastUser, error) {
	if limit <= 0 {
		limit = 10
	}
	var out []BroadcastUser
	err := r.buildUserScope(ctx, target).
		Select("id, username, COALESCE(email, '') AS email, COALESCE(phone, '') AS phone").
		Order("id ASC").Limit(limit).Scan(&out).Error
	return out, err
}

// StreamTargets 按 id 游标分批遍历，避免 OFFSET 在大表上的性能塌陷。
func (r *broadcastRepository) StreamTargets(ctx context.Context, target notifydto.BroadcastTarget, batch int, fn func([]BroadcastUser) error) error {
	if batch <= 0 {
		batch = 1000
	}
	lastID := uint64(0)
	for {
		var rows []BroadcastUser
		q := r.buildUserScope(ctx, target).Where("id > ?", lastID)
		if err := q.Select("id, username, COALESCE(email, '') AS email, COALESCE(phone, '') AS phone").
			Order("id ASC").Limit(batch).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		lastID = rows[len(rows)-1].ID
		if err := fn(rows); err != nil {
			return err
		}
		if len(rows) < batch {
			return nil
		}
	}
}

func (r *broadcastRepository) ListGroups(ctx context.Context) ([]notifydto.BroadcastGroupItem, error) {
	type row struct {
		ID          uint64
		Name        string
		MemberCount int64
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("user_groups g").
		Select("g.id, g.name, COUNT(u.id) AS member_count").
		Joins("LEFT JOIN users u ON u.user_group_id = g.id AND u.status = 'active' AND COALESCE(u.is_sub_account, false) = false").
		Where("g.status = ?", "active").
		Group("g.id, g.name").
		Order("g.sort_order ASC, g.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]notifydto.BroadcastGroupItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, notifydto.BroadcastGroupItem{ID: row.ID, Name: row.Name, MemberCount: row.MemberCount})
	}
	return out, nil
}

func (r *broadcastRepository) SearchUsers(ctx context.Context, keyword string, page, size int) ([]BroadcastUser, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	base := r.db.WithContext(ctx).Table("users").
		Where("status = ?", "active").
		Where("COALESCE(is_sub_account, false) = false")
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("username LIKE ? OR email LIKE ? OR phone LIKE ?", like, like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []BroadcastUser
	err := base.Select("id, username, COALESCE(email, '') AS email, COALESCE(phone, '') AS phone").
		Order("id DESC").Offset((page - 1) * size).Limit(size).Scan(&out).Error
	return out, total, err
}

func (r *broadcastRepository) CountBatch(ctx context.Context, batchID string) (map[string]int64, error) {
	var rows []struct {
		SendStatus string
		Count      int64
	}
	if err := r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{}).
		Select("send_status, count(*) AS count").
		Where("batch_id = ?", batchID).
		Group("send_status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.SendStatus] = row.Count
	}
	return out, nil
}

// BatchIDPrefix 群发批次号前缀。
const BatchIDPrefix = "BC"

// NewBatchID 生成群发批次号：BC + 时间戳 + 4 位随机 hex。
func NewBatchID(now string, suffix string) string {
	return fmt.Sprintf("%s%s-%s", BatchIDPrefix, now, suffix)
}
