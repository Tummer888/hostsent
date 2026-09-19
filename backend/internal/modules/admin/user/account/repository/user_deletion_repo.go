package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/model"
)

// purgeTables 硬删除时需要按依赖顺序清理的表。
//
// 顺序原则：先叶子后主表。表名均为字面量（不接受外部输入），因此可安全拼进 SQL。
//
// 不含 admin_audit_logs / upstream_api_logs / job_run_logs / log_cleanup_jobs：
// 它们是平台合规日志，不属于用户个人数据，由日志中心的保留策略独立管理
// （doc104 §4.7）。硬删除用户后这些行仍保留 admin_id/操作留痕，这是有意为之。
//
// 同样不含 retired_* 系归档快照表（retired_user_orders_20260915、
// retired_user_bills_20260909 等）：它们是被迁移脚本冻结的历史证据副本，
// 只用于对账与审计，不是活跃个人数据，且已经脱离业务读写路径。
//
// column 除了真实列名，还支持两种「经父表反查」的伪列：
// order_of_user（按 orders.id）与 callback_of_user（按 payment_orders.payment_no）。
var purgeTables = []struct {
	table  string
	column string
}{
	// 订单与退款
	{"order_refunds", "user_id"},
	{"payment_refunds", "user_id"},
	{"provision_tasks", "user_id"},
	// order_items 无 user_id，只能按 order_id 反查；必须排在 orders 之前，
	// 否则 orders 一删就再也找不到这些明细行，留下永久孤儿。
	{"order_items", "order_of_user"},
	{"orders", "user_id"},
	// 账单与发票
	{"invoice_requests", "user_id"},
	{"bills", "user_id"},
	// 钱包与资金流水
	{"wallet_transactions", "user_id"},
	{"withdrawals", "user_id"},
	{"recharges", "user_id"},
	{"wallet_accounts", "user_id"},
	// 实例与生命周期
	{"instance_operations", "user_id"},
	{"instance_renewals", "user_id"},
	{"instance_auto_renewals", "user_id"},
	// instances.user_id 是全库唯一指向 users 的外键，必须最后删（上面已先查在管实例）。
	{"instances", "user_id"},
	// 工单
	{"ticket_logs", "ticket_id"},
	{"ticket_attachments", "ticket_id"},
	{"ticket_replies", "ticket_id"},
	{"tickets", "user_id"},
	// 积分
	{"point_transactions", "user_id"},
	{"point_accounts", "user_id"},
	// 返现
	{"referral_transactions", "invitee_user_id"},
	{"referral_transactions", "inviter_user_id"},
	{"referral_transactions", "user_id"},
	{"referral_withdrawals", "user_id"},
	{"referral_accounts", "user_id"},
	// 支付中心
	// payment_callback_logs 无 user_id，按 payment_no 反查；raw_body 含下单人信息，
	// 属个人数据，随支付单一起清。同样必须排在 payment_orders 之前。
	{"payment_callback_logs", "callback_of_user"},
	{"payment_payouts", "user_id"},
	{"payment_orders", "user_id"},
	{"user_payout_accounts", "user_id"},
	{"user_payment_preferences", "user_id"},
	// 实名认证
	{"verification_documents", "application_id"},
	{"verification_enterprises", "application_id"},
	{"verification_review_logs", "application_id"},
	{"verification_applications", "user_id"},
	// 通知（notifications.user_id = 0 表示广播，不会被本行命中，符合预期）
	{"notification_reads", "user_id"},
	{"notification_preferences", "user_id"},
	{"notifications", "user_id"},
	// 营销与权限
	{"coupon_grants", "user_id"},
	{"user_roles", "user_id"},
	{"sub_account_permissions", "user_id"},
	{"user_security_settings", "user_id"},
	{"staff_sales_relations", "user_id"},
	{"user_level_change_logs", "user_id"},
	{"user_operation_logs", "account_user_id"},
	{"user_oauth_bindings", "user_id"},
	// 安全与日志（属于用户个人数据，随用户删除）
	{"login_logs", "user_id"},
	{"user_sessions", "user_id"},
	{"risk_events", "user_id"},
	// 开放平台
	{"open_apps", "owner_user_id"},
}

// anonymizeOnPurge 硬删除时「置零引用」而非删除的表。
//
// sales_commission_transactions 是业务员（admin_id）的佣金台账，不是用户自己的
// 资产：它记录的是「某业务员从某客户身上拿了多少佣金」，删掉就等于销毁他人的
// 财务凭证，与 §4.7「合规留痕不清理」同一条原则。这里只把 customer_user_id
// 归零做去标识化，保留台账本身的完整性与对账能力。
var anonymizeOnPurge = []struct {
	table  string
	column string
}{
	{"sales_commission_transactions", "customer_user_id"},
}

// DeletionCheckRow 注销前置校验的原始计数。
type DeletionCheckRow struct {
	Instances      int64 `gorm:"column:instances"`
	BalanceNonZero int64 `gorm:"column:balance_nonzero"`
	UnpaidBills    int64 `gorm:"column:unpaid_bills"`
	OpenTickets    int64 `gorm:"column:open_tickets"`
	OpenOrders     int64 `gorm:"column:open_orders"`
}

// DeletionCheck 汇总注销前置校验所需的全部计数（单条 SQL，避免 5 次往返）。
//
// 阻断与警告的分级见 doc104 §4.4：在管实例硬阻断（有外键，且硬删除前必须释放），
// 其余四项可被 force 绕过。
func (r *userRepository) DeletionCheck(ctx context.Context, id uint64) (*DeletionCheckRow, error) {
	var out DeletionCheckRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			(SELECT COUNT(*) FROM instances WHERE user_id = ? AND status NOT IN ('destroyed','terminated','released')) AS instances,
			(SELECT COUNT(*) FROM wallet_accounts WHERE user_id = ? AND balance <> 0)                                     AS balance_nonzero,
			(SELECT COUNT(*) FROM bills WHERE user_id = ? AND status = 'unpaid')                                          AS unpaid_bills,
			(SELECT COUNT(*) FROM tickets WHERE user_id = ? AND status IN ('open','in_progress','waiting_user'))           AS open_tickets,
			(SELECT COUNT(*) FROM orders WHERE user_id = ? AND deleted_at IS NULL AND status IN ('pending','paid'))        AS open_orders
	`, id, id, id, id, id).Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SoftDelete 软删除（注销）用户：写删除标记并把状态收敛为 cancelled。
//
// statusBefore 由服务层传入（注销前的真实状态），恢复时精确还原。
// 用 Updates(map) 而非 Save，避免把内存里的联表别名（user_group_name 等只读字段）
// 一起写回。
func (r *userRepository) SoftDelete(ctx context.Context, id uint64, operatorID uint64, reason, statusBefore string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"deleted_at":           now,
		"deleted_by":           operatorID,
		"delete_reason":        reason,
		"status_before_delete": statusBefore,
		"status":               model.StatusCancelled,
	}).Error
}

// Restore 恢复已注销用户。
//
// restoreStatus 为空时回落 disabled（而不是 active）：注销前若本就是 disabled，
// 恢复成 active 等于顺手解封，属于越权。
func (r *userRepository) Restore(ctx context.Context, id uint64, restoreStatus string) error {
	if restoreStatus == "" {
		restoreStatus = model.StatusDisabled
	}
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"deleted_at":           nil,
		"deleted_by":           0,
		"delete_reason":        "",
		"status_before_delete": "",
		"status":               restoreStatus,
	}).Error
}

// ListPurgeCandidates 返回留存期已过、可硬删除的用户。
//
// 只取「已注销 且 deleted_at 早于 before」的行。多取 1 条用于判断是否还有剩余，
// 便于调度器在日志里给出「本轮已满，仍有积压」的提示。
func (r *userRepository) ListPurgeCandidates(ctx context.Context, before time.Time, limit int) ([]model.User, error) {
	if limit <= 0 {
		limit = 50
	}
	var users []model.User
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", before).
		Order("deleted_at ASC, id ASC").
		Limit(limit).
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// PurgeUser 硬删除一个用户及其全部个人数据（单事务）。
//
// 前置条件：调用方必须先确认该用户没有在管实例——instances.user_id 是全库唯一
// 指向 users 的外键，残留实例会让最后一步的 DELETE 直接撞 23503。
// 本方法仍会再查一次并返回 ErrUserHasInstances，把「谁忘了检查」这件事暴露出来。
func (r *userRepository) PurgeUser(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var running int64
		if err := tx.Table("instances").
			Where("user_id = ? AND status NOT IN ('destroyed','terminated','released')", id).
			Count(&running).Error; err != nil {
			return err
		}
		if running > 0 {
			return ErrUserHasInstances
		}

		// 工单子表按 ticket_id 关联，需先取出工单 ID 集合。
		var ticketIDs []uint64
		if err := tx.Table("tickets").Where("user_id = ?", id).Pluck("id", &ticketIDs).Error; err != nil {
			return err
		}
		// 实名申请子表同理。
		var applicationIDs []uint64
		if err := tx.Table("verification_applications").Where("user_id = ?", id).Pluck("id", &applicationIDs).Error; err != nil {
			return err
		}
		// order_items / payment_callback_logs 没有 user_id，只能经父表反查。
		// 必须在父表被删之前取出来。
		var orderIDs []uint64
		if err := tx.Table("orders").Where("user_id = ?", id).Pluck("id", &orderIDs).Error; err != nil {
			return err
		}
		var paymentNos []string
		if err := tx.Table("payment_orders").Where("user_id = ?", id).Pluck("payment_no", &paymentNos).Error; err != nil {
			return err
		}

		for _, t := range purgeTables {
			switch t.column {
			case "ticket_id":
				if len(ticketIDs) == 0 {
					continue
				}
				if err := tx.Exec("DELETE FROM "+t.table+" WHERE ticket_id IN ?", ticketIDs).Error; err != nil {
					return err
				}
			case "application_id":
				if len(applicationIDs) == 0 {
					continue
				}
				if err := tx.Exec("DELETE FROM "+t.table+" WHERE application_id IN ?", applicationIDs).Error; err != nil {
					return err
				}
			case "order_of_user":
				if len(orderIDs) == 0 {
					continue
				}
				if err := tx.Exec("DELETE FROM "+t.table+" WHERE order_id IN ?", orderIDs).Error; err != nil {
					return err
				}
			case "callback_of_user":
				if len(paymentNos) == 0 {
					continue
				}
				if err := tx.Exec("DELETE FROM "+t.table+" WHERE payment_no IN ?", paymentNos).Error; err != nil {
					return err
				}
			default:
				if err := tx.Exec("DELETE FROM "+t.table+" WHERE "+t.column+" = ?", id).Error; err != nil {
					return err
				}
			}
		}

		// 去标识化：把指向被删用户的引用置零，但保留行本身（见 anonymizeOnPurge 注释）。
		for _, t := range anonymizeOnPurge {
			if err := tx.Exec("UPDATE "+t.table+" SET "+t.column+" = 0 WHERE "+t.column+" = ?", id).Error; err != nil {
				return err
			}
		}

		// 自引用解绑：其他用户的邀请人/主账号指向被删用户时置空，
		// 否则会留下指向不存在 ID 的悬空引用（这两列没有外键，不会报错，只会静默脏）。
		if err := tx.Exec("UPDATE users SET inviter_user_id = NULL WHERE inviter_user_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE users SET owner_user_id = NULL, is_sub_account = false WHERE owner_user_id = ?", id).Error; err != nil {
			return err
		}

		return tx.Exec("DELETE FROM users WHERE id = ?", id).Error
	})
}

// CountDeleted 已注销用户数（总览卡片与回收站入口）。
func (r *userRepository) CountDeleted(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("deleted_at IS NOT NULL").Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// OAuthProvidersByUserIDs 批量查询用户已绑定的第三方渠道（列表图标列）。
//
// 这是修掉「前端读 oauth_providers 而后端从不生产」的那一侧：返回 userID → provider 列表。
func (r *userRepository) OAuthProvidersByUserIDs(ctx context.Context, ids []uint64) (map[uint64][]string, error) {
	result := make(map[uint64][]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		UserID   uint64 `gorm:"column:user_id"`
		Provider string `gorm:"column:provider"`
	}
	if err := r.db.WithContext(ctx).
		Table("user_oauth_bindings").
		Select("user_id", "provider").
		Where("user_id IN ? AND status = ?", ids, "active").
		Order("user_id ASC, provider ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.Provider)
	}
	return result, nil
}

// ErrUserHasInstances 该用户仍有在管实例，不可硬删除。
//
// 定义在仓储包而非服务包：它是数据完整性约束的直接映射（instances.user_id 是
// 全库唯一指向 users 的外键），服务层只是转述，不该另起一个同义错误。
var ErrUserHasInstances = errors.New("用户仍存在未释放的实例，无法删除")

// AdminNamesByIDs 批量查询管理员 ID → 显示名（real_name 优先，回退 username）。
//
// 用途：回收站与用户详情的「注销人」列。deleted_by = 0 表示系统或用户自助，
// 调用方自行渲染成「系统」，本方法不对 0 做特判（保持纯查询语义）。
func (r *userRepository) AdminNamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	result := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ID   uint64 `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := r.db.WithContext(ctx).
		Table("admins").
		Select("id", "COALESCE(NULLIF(real_name, ''), username, '') AS name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result, nil
}
