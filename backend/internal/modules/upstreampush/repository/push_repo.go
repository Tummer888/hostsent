// Package repository 提供上游推送接收端的数据访问。
//
// 这一层刻意用窄方法（按订单号取令牌、按订单回写实例、按上游工单号反查工单）
// 而不是复用各自域的大仓储：推送接收是免登录入口，能碰到的数据面越小越好；
// 每个方法都写清了"为什么只能查这一列"。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ErrNotFound 目标行不存在（令牌查不到订单、上游工单号查不到工单）。
var ErrNotFound = errors.New("upstreampush: target not found")

// InstanceSnapshot 推送命中实例行前的最小快照，用于判定是否真的需要改（幂等）。
type InstanceSnapshot struct {
	ID                 uint64
	ProviderID         uint64
	Status             string
	LifecycleStage     string
	PublicIP           string
	PrivateIP          string
	OS                 string
	RawData            string
	ExpireAt           *time.Time
	ProviderInstanceID string
}

// InstancePatch 要写回实例的字段（零值表示不改）。
type InstancePatch struct {
	ProviderInstanceID string
	Status             string
	PublicIP           string
	PrivateIP          string
	OS                 string
	RawData            string
	ExpireAt           *time.Time
	// TouchSyncedAt 是否刷新 last_synced_at（有实际改动时置 true）。
	TouchSyncedAt bool
}

// Repository 上游推送接收数据访问能力。
type Repository interface {
	// DownstreamTokenByOrderID 取订单的上游对接令牌（本渠道推送的验签密钥）。
	DownstreamTokenByOrderID(ctx context.Context, orderID uint64) (string, error)
	// InstanceByOrderID 取该订单关联的实例行（取最早一行，与开通落库口径一致）。
	InstanceByOrderID(ctx context.Context, orderID uint64) (*InstanceSnapshot, error)
	// ApplyInstancePatch 按主键回写实例字段。
	ApplyInstancePatch(ctx context.Context, instanceID uint64, patch InstancePatch) error
	// OrderStatus 取订单当前状态（判断是否要把 provisioning 推进到 active）。
	OrderStatus(ctx context.Context, orderID uint64) (string, error)
	// AdvanceOrderToActive CAS 推进订单 provisioning → active，返回是否生效。
	AdvanceOrderToActive(ctx context.Context, orderID uint64) (bool, error)

	// TicketIDByUpstreamTID 按上游工单号反查本地工单 id。
	TicketIDByUpstreamTID(ctx context.Context, upstreamTID string) (uint64, error)
	// TicketUserAndNo 取工单的归属用户与工单号（推送回复要通知用户）。
	TicketUserAndNo(ctx context.Context, ticketID uint64) (userID uint64, ticketNo string, err error)
	// InsertUpstreamReply 写入一条来源为上游的回复；已存在（同一上游回复号）返回 applied=false。
	InsertUpstreamReply(ctx context.Context, ticketID uint64, upstreamReplyID, content string) (replyID uint64, applied bool, err error)
	// MarkTicketWaitingUser 上游回复即等用户回话：状态推进为 waiting_user 并刷新首响/更新时间。
	MarkTicketWaitingUser(ctx context.Context, ticketID uint64) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository 创建上游推送接收仓储。
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) DownstreamTokenByOrderID(ctx context.Context, orderID uint64) (string, error) {
	var token string
	err := r.db.WithContext(ctx).
		Table("orders").
		Select("downstream_token").
		Where("id = ? AND deleted_at IS NULL", orderID).
		Scan(&token).Error
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", ErrNotFound
	}
	return token, nil
}

func (r *repository) InstanceByOrderID(ctx context.Context, orderID uint64) (*InstanceSnapshot, error) {
	var row InstanceSnapshot
	err := r.db.WithContext(ctx).
		Table("instances").
		Select("id, provider_id, COALESCE(status, '') AS status, COALESCE(lifecycle_stage, '') AS lifecycle_stage, "+
			"COALESCE(public_ip, '') AS public_ip, COALESCE(private_ip, '') AS private_ip, COALESCE(os, '') AS os, "+
			"COALESCE(raw_data, '') AS raw_data, expire_at, COALESCE(provider_instance_id, '') AS provider_instance_id").
		Where("order_id = ?", orderID).
		Order("id ASC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, ErrNotFound
	}
	return &row, nil
}

func (r *repository) ApplyInstancePatch(ctx context.Context, instanceID uint64, patch InstancePatch) error {
	updates := map[string]any{
		"status":     patch.Status,
		"public_ip":  patch.PublicIP,
		"private_ip": patch.PrivateIP,
		"os":         patch.OS,
		"raw_data":   patch.RawData,
		"updated_at": time.Now(),
	}
	if patch.ProviderInstanceID != "" {
		updates["provider_instance_id"] = patch.ProviderInstanceID
	}
	if patch.ExpireAt != nil {
		updates["expire_at"] = *patch.ExpireAt
	}
	if patch.TouchSyncedAt {
		updates["last_synced_at"] = time.Now()
	}
	return r.db.WithContext(ctx).Table("instances").Where("id = ?", instanceID).Updates(updates).Error
}

func (r *repository) OrderStatus(ctx context.Context, orderID uint64) (string, error) {
	var status string
	if err := r.db.WithContext(ctx).Table("orders").Select("status").Where("id = ?", orderID).Scan(&status).Error; err != nil {
		return "", err
	}
	if status == "" {
		return "", ErrNotFound
	}
	return status, nil
}

// AdvanceOrderToActive 只认 provisioning → active 这一条迁移（CAS）。
// 已 refuned/cancelled 的订单不因上游一句推送被复活——那是状态机的事，不是推送的事。
func (r *repository) AdvanceOrderToActive(ctx context.Context, orderID uint64) (bool, error) {
	res := r.db.WithContext(ctx).Table("orders").
		Where("id = ? AND status = ?", orderID, "provisioning").
		Updates(map[string]any{"status": "active", "updated_at": time.Now()})
	return res.RowsAffected > 0, res.Error
}

func (r *repository) TicketIDByUpstreamTID(ctx context.Context, upstreamTID string) (uint64, error) {
	var id uint64
	if err := r.db.WithContext(ctx).Table("tickets").
		Select("id").Where("upstream_tid = ?", upstreamTID).Order("id ASC").Limit(1).Scan(&id).Error; err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, ErrNotFound
	}
	return id, nil
}

func (r *repository) TicketUserAndNo(ctx context.Context, ticketID uint64) (uint64, string, error) {
	var row struct {
		UserID   uint64
		TicketNo string
	}
	if err := r.db.WithContext(ctx).Table("tickets").
		Select("user_id, ticket_no").Where("id = ?", ticketID).Scan(&row).Error; err != nil {
		return 0, "", err
	}
	return row.UserID, row.TicketNo, nil
}

// InsertUpstreamReply 幂等写入上游回复。
//
// 幂等键用 (ticket_id, upstream_reply_id) 部分唯一索引，而不是先查后插：
// 上游重试是并发/连续到达的，先查后插会两侧都查不到、两侧都插入。
// ON CONFLICT DO NOTHING + RowsAffected 判定是否真的写进去了。
func (r *repository) InsertUpstreamReply(ctx context.Context, ticketID uint64, upstreamReplyID, content string) (uint64, bool, error) {
	var insertedID uint64
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO ticket_replies (ticket_id, sender_type, sender_id, sender_name, content, is_internal, upstream_reply_id, created_at)
		VALUES (?, 'admin', 0, '上游客服', ?, false, ?, now())
		ON CONFLICT (ticket_id, upstream_reply_id) WHERE upstream_reply_id <> '' DO NOTHING
		RETURNING id`, ticketID, content, upstreamReplyID).Scan(&insertedID).Error
	if err != nil {
		return 0, false, err
	}
	if insertedID == 0 {
		// 冲突未插入：取回既有行 id，让调用方能返回给上游。
		var existing uint64
		if err := r.db.WithContext(ctx).Table("ticket_replies").
			Select("id").Where("ticket_id = ? AND upstream_reply_id = ?", ticketID, upstreamReplyID).
			Scan(&existing).Error; err != nil {
			return 0, false, err
		}
		return existing, false, nil
	}
	return insertedID, true, nil
}

func (r *repository) MarkTicketWaitingUser(ctx context.Context, ticketID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Table("tickets").
		Where("id = ?", ticketID).
		Where("status NOT IN ?", []string{"closed", "cancelled", "resolved"}).
		Updates(map[string]any{
			"status":         "waiting_user",
			"updated_at":     now,
			"first_reply_at": gorm.Expr("COALESCE(first_reply_at, ?)", now),
		}).Error
}
