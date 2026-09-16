package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/model"
)

// recentLimit 聚合视图每个业务域最多带回的行数。
// 详情页各 Tab 的完整列表走各自的分页接口（/orders、/tickets、/instances …），
// 这里只负责「概览一眼看得到」的近期若干条，因此必须封顶，
// 否则单用户的流水/订单会随业务增长无上限地拖慢整个详情页。
const recentLimit = 8

type UserDetailRepository interface {
	// ListCustomerPermissions 客户侧权限码（子账号权限，固定枚举，落 sub_account_permissions）。
	ListCustomerPermissions(ctx context.Context, userID uint64) ([]string, error)
	// ListRbacRolesByUserID 用户绑定的后台角色（user_roles ⋈ roles），只读展示用。
	ListRbacRolesByUserID(ctx context.Context, userID uint64) ([]model.UserRoleBrief, error)
	// ListInstancesByUserID 实例权威表 instances 的近期摘要。
	ListInstancesByUserID(ctx context.Context, userID uint64) ([]model.UserInstanceBrief, error)
	// ListOrdersByUserID 订单权威表 orders 的近期摘要。
	ListOrdersByUserID(ctx context.Context, userID uint64) ([]model.UserOrderBrief, error)
	// ListBillsByUserID 账单权威表 bills 的近期摘要。
	ListBillsByUserID(ctx context.Context, userID uint64) ([]model.UserBillBrief, error)
	// ListTransactionsByUserID 资金流水权威表 wallet_transactions 的近期摘要。
	ListTransactionsByUserID(ctx context.Context, userID uint64) ([]model.UserTransactionBrief, error)
	// ListTicketsByUserID 工单权威表 tickets 的近期摘要。
	ListTicketsByUserID(ctx context.Context, userID uint64) ([]model.UserTicketBrief, error)
	// Counts 各域计数，单条 SQL 取回，供 Tab 徽标与统计卡使用。
	Counts(ctx context.Context, userID uint64) (*model.UserDetailCounts, error)
}

type userDetailRepository struct {
	db *gorm.DB
}

func NewUserDetailRepository(db *gorm.DB) UserDetailRepository {
	return &userDetailRepository{db: db}
}

// ListCustomerPermissions 读客户侧权限码。
//
// 语义修正（本轮）：客户侧权限是固定枚举（internal/pkg/auth/user_permission.go），
// 落表 sub_account_permissions，与后台 RBAC（admin_roles → role_permissions →
// permissions）完全无关。原实现走 user_roles ⋈ role_permissions ⋈ permissions，
// 导致客户详情页渲染出「用户列表 / 用户管理 / 查看用户详情」这类**后台权限节点**。
func (r *userDetailRepository) ListCustomerPermissions(ctx context.Context, userID uint64) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).
		Table("sub_account_permissions").
		Where("user_id = ?", userID).
		Order("permission_code ASC").
		Pluck("permission_code", &codes).Error
	if err != nil {
		return nil, err
	}
	if codes == nil {
		codes = []string{}
	}
	return codes, nil
}

// ListRbacRolesByUserID 读用户绑定的后台角色。仅作只读展示，说明"这个账号在后台
// 被授予了什么角色"，不作为客户侧权限解释。
func (r *userDetailRepository) ListRbacRolesByUserID(ctx context.Context, userID uint64) ([]model.UserRoleBrief, error) {
	var items []model.UserRoleBrief
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Select("roles.id AS id, roles.code AS code, roles.name AS name, roles.scope AS scope").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.id ASC").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.UserRoleBrief{}
	}
	return items, nil
}

// ListInstancesByUserID 读实例权威表 instances。
// 显式列出真实列：原实现只 Select 了 6 列并把 cpu/memory/disk 拼成 specs 字符串、
// 用 COALESCE 把 NULL 到期时间伪造成 1970-01-01，操作员会把「未设置」误读成真实值。
func (r *userDetailRepository) ListInstancesByUserID(ctx context.Context, userID uint64) ([]model.UserInstanceBrief, error) {
	var items []model.UserInstanceBrief
	err := r.db.WithContext(ctx).
		Table("instances").
		Select(`id, instance_id, name, COALESCE(region, '') AS region, COALESCE(zone, '') AS zone,
			cpu, memory, disk, COALESCE(os, '') AS os, COALESCE(public_ip, '') AS public_ip,
			status, COALESCE(billing_mode, '') AS billing_mode,
			COALESCE(lifecycle_stage, '') AS lifecycle_stage, order_id,
			COALESCE(source_mode, '') AS source_mode, expire_at, created_at`).
		Where("user_id = ?", userID).
		Order("id DESC").
		Limit(recentLimit).
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.UserInstanceBrief{}
	}
	return items, nil
}

// ListOrdersByUserID 读订单权威表 orders（user_orders 已于 Phase 2 退役，见 migrations/013、049）。
func (r *userDetailRepository) ListOrdersByUserID(ctx context.Context, userID uint64) ([]model.UserOrderBrief, error) {
	var items []model.UserOrderBrief
	err := r.db.WithContext(ctx).
		Table("orders").
		Select(`id, order_no, COALESCE(product_name, '') AS product_name,
			final_amount, status, COALESCE(pay_method, '') AS pay_method,
			renewal_id, created_at, pay_time AS paid_at`).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC, id DESC").
		Limit(recentLimit).
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.UserOrderBrief{}
	}
	return items, nil
}

// ListBillsByUserID 读账单权威表 bills（user_bills 已于 Phase 2 退役，见 migrations/014）。
func (r *userDetailRepository) ListBillsByUserID(ctx context.Context, userID uint64) ([]model.UserBillBrief, error) {
	var items []model.UserBillBrief
	err := r.db.WithContext(ctx).
		Table("bills").
		Select("id, bill_no, period, total_amount, bill_type, status, created_at").
		Where("user_id = ?", userID).
		Order("period DESC, id DESC").
		Limit(recentLimit).
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.UserBillBrief{}
	}
	return items, nil
}

// ListTransactionsByUserID 读资金流水权威表 wallet_transactions
//（user_transactions 已于 Phase 2 退役，见 migrations/014）。
func (r *userDetailRepository) ListTransactionsByUserID(ctx context.Context, userID uint64) ([]model.UserTransactionBrief, error) {
	var items []model.UserTransactionBrief
	err := r.db.WithContext(ctx).
		Table("wallet_transactions").
		Select("id, tx_no, type, direction, amount, balance_after, COALESCE(remark, '') AS remark, created_at").
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Limit(recentLimit).
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.UserTransactionBrief{}
	}
	return items, nil
}

// ListTicketsByUserID 读工单权威表 tickets（user_tickets 已于 Phase 2 退役）。
func (r *userDetailRepository) ListTicketsByUserID(ctx context.Context, userID uint64) ([]model.UserTicketBrief, error) {
	var items []model.UserTicketBrief
	err := r.db.WithContext(ctx).
		Table("tickets").
		Select("id, ticket_no, title, category, priority, status, updated_at, created_at").
		Where("user_id = ?", userID).
		Order("updated_at DESC, id DESC").
		Limit(recentLimit).
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.UserTicketBrief{}
	}
	return items, nil
}

// Counts 单条 SQL 取回 14 个计数。全部用标量子查询，任何一个域缺数据都返回 0，
// 不因某张表为空而让整个详情页失败。
func (r *userDetailRepository) Counts(ctx context.Context, userID uint64) (*model.UserDetailCounts, error) {
	var out model.UserDetailCounts
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			(SELECT COUNT(*) FROM instances WHERE user_id = ?)                                            AS instance_count,
			(SELECT COUNT(*) FROM instances WHERE user_id = ? AND status = 'running')                     AS running_instance_count,
			(SELECT COUNT(*) FROM orders WHERE user_id = ? AND deleted_at IS NULL)                        AS order_count,
			(SELECT COALESCE(SUM(final_amount), 0) FROM orders WHERE user_id = ? AND deleted_at IS NULL
				AND status IN ('paid','active','completed','refunding','refunded'))                       AS order_total_amount,
			(SELECT COUNT(*) FROM bills WHERE user_id = ?)                                                AS bill_count,
			(SELECT COUNT(*) FROM bills WHERE user_id = ? AND status = 'unpaid')                          AS unpaid_bill_count,
			(SELECT COUNT(*) FROM wallet_transactions WHERE user_id = ?)                                  AS transaction_count,
			(SELECT COUNT(*) FROM tickets WHERE user_id = ?)                                              AS ticket_count,
			(SELECT COUNT(*) FROM tickets WHERE user_id = ? AND status IN ('open','in_progress','waiting_user')) AS open_ticket_count,
			(SELECT COUNT(*) FROM login_logs WHERE user_id = ?)                                           AS login_count,
			(SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND status = 'active')                  AS active_session_count,
			(SELECT COUNT(*) FROM risk_events WHERE user_id = ?)                                          AS risk_event_count,
			(SELECT COUNT(*) FROM user_operation_logs WHERE account_user_id = ?)                          AS operation_log_count,
			(SELECT COUNT(*) FROM verification_applications WHERE user_id = ?)                            AS verification_count
	`, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID).
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return &out, nil
}