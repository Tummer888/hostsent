// Package repository 提供用户中心主机模块的数据访问实现。
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/instance/model"
)

// ErrNotFound 主机不存在或不属于当前用户。
var ErrNotFound = errors.New("主机不存在或无权访问")

// OperationEntry 实例操作流水（用户可见的只读视图）。
//
// 只暴露「谁、什么时候、做了什么、成功没有」，不含 Params 与上游返回原文：
// 那是运营侧排障信息，且可能带内部参数，不适合直接透给客户。
type OperationEntry struct {
	ID           uint64
	Action       string
	OperatorType string
	OperatorName string
	Result       string
	ErrorMessage string
	CreatedAt    time.Time
}

// InstanceRepository 用户中心主机仓储接口。
type InstanceRepository interface {
	ListByUser(ctx context.Context, userID uint64) ([]model.Instance, error)
	FindByUser(ctx context.Context, userID, id uint64) (*model.Instance, error)
	// Provision 读来源订单的开通履约状态（provision_tasks）：status + 最近失败原因。
	// 待开通实例的详情据此展示"为什么还没开通"。
	Provision(ctx context.Context, orderID uint64) (status, lastError string, err error)
	// Operations 读某实例最近的操作流水（倒序）；归属由调用方先用 FindByUser 校验。
	Operations(ctx context.Context, instanceID uint64, limit int) ([]OperationEntry, error)
	// UpdateStatus 落库实例服务状态（电源操作下发成功后即时反映到界面）。
	// 只写 status 一列：这是用户中心对实例的唯一写入点，其余字段归同步/开通链路。
	UpdateStatus(ctx context.Context, id uint64, status string) error

	// ---- 过渡态（迁移 112，与运维台共用同一组 instances 列）----
	//
	// 客户侧的电源/维护操作走的是同一张表、同一批上游接口，因此必须在同一处
	// 互斥：客户连点两下、或客户点完运营又点，都只能有一个指令真正下发。
	//
	// ClaimTransition 原子抢占：仅当没有生效中的过渡态时写入并返回 true。
	// 判定与写入压在同一条 UPDATE 的 WHERE 里，由数据库保证并发下只有一个赢。
	ClaimTransition(ctx context.Context, id uint64, state string, startedAt, until time.Time) (bool, error)
	// ClearTransitionIfOwner 仅当过渡态仍等于 state 时清除（下发失败回滚用），
	// 避免慢请求的失败回滚误清掉后来者已抢占的占位。
	ClearTransitionIfOwner(ctx context.Context, id uint64, state string) error
}

type instanceRepository struct {
	db *gorm.DB
}

// NewInstanceRepository 创建用户中心主机仓储。
func NewInstanceRepository(db *gorm.DB) InstanceRepository {
	return &instanceRepository{db: db}
}

// ListByUser 查询某用户的全部主机（附带操作人用户名，用于「操作人」列 P4-09）。
func (r *instanceRepository) ListByUser(ctx context.Context, userID uint64) ([]model.Instance, error) {
	var items []model.Instance
	if err := r.db.WithContext(ctx).
		Select("instances.*, actor.username AS actor_name").
		Joins("LEFT JOIN users AS actor ON actor.id = instances.actor_user_id").
		Where("instances.user_id = ?", userID).
		Order("instances.id desc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Provision 读来源订单的开通任务状态（未创建任务/订单为 0 时返回空串）。
func (r *instanceRepository) Provision(ctx context.Context, orderID uint64) (string, string, error) {
	if orderID == 0 {
		return "", "", nil
	}
	var row struct {
		Status    string
		LastError string
	}
	err := r.db.WithContext(ctx).Table("provision_tasks").
		Select("status, COALESCE(last_error, '') AS last_error").
		Where("order_id = ?", orderID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", "", nil
		}
		return "", "", err
	}
	return row.Status, row.LastError, nil
}

// FindByUser 按 id + userID 查询主机（校验归属）。
func (r *instanceRepository) FindByUser(ctx context.Context, userID, id uint64) (*model.Instance, error) {
	var item model.Instance
	err := r.db.WithContext(ctx).
		Select("instances.*, actor.username AS actor_name").
		Joins("LEFT JOIN users AS actor ON actor.id = instances.actor_user_id").
		Where("instances.user_id = ? AND instances.id = ?", userID, id).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Operations 读实例最近操作流水（倒序，最多 limit 条）。
// 直接查 instance_operations 而不是委派管理端：用户只该看到**自己实例**的记录，
// 用 instance_id 过滤即可，不需要跨用户的审计视图，也不必让 uc 包依赖 admin 包。
func (r *instanceRepository) Operations(ctx context.Context, instanceID uint64, limit int) ([]OperationEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []OperationEntry
	if err := r.db.WithContext(ctx).Table("instance_operations").
		Select("id, action, operator_type, operator_name, result, COALESCE(error_message, '') AS error_message, created_at").
		Where("instance_id = ?", instanceID).
		Order("id DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ClaimTransition 原子抢占过渡态：仅当没有"生效中"的过渡态时写入。
//
// WHERE 的两个条件对应两类可抢占情形：本来空闲（NULL/空串），或上一个动作的
// 窗口已过（结果未知，不该永久锁死实例）。条件与赋值在同一条 UPDATE 里，
// 由数据库行锁保证互斥 —— 这是客户侧"连点两次都下发"的直接修法。
func (r *instanceRepository) ClaimTransition(ctx context.Context, id uint64, state string, startedAt, until time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.Instance{}).
		Where("id = ? AND (transition_state IS NULL OR transition_state = '' OR transition_until IS NULL OR transition_until < ?)",
			id, startedAt).
		Updates(map[string]any{
			"transition_state":      state,
			"transition_started_at": startedAt,
			"transition_until":      until,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ClearTransitionIfOwner 仅当过渡态仍等于 state 时清除（下发失败的回滚）。
func (r *instanceRepository) ClearTransitionIfOwner(ctx context.Context, id uint64, state string) error {
	return r.db.WithContext(ctx).Model(&model.Instance{}).
		Where("id = ? AND transition_state = ?", id, state).
		Updates(map[string]any{
			"transition_state":      nil,
			"transition_started_at": nil,
			"transition_until":      nil,
		}).Error
}

// UpdateStatus 落库实例服务状态。
//
// 用 Updates 只改 status：GORM 的 Updates(struct) 会跳过零值字段（status="stopped"
// 这类非空值没问题，但显式只列一列更能表达"这是唯一写入点"的意图），
// 同时自动带上 updated_at，便于运营看出状态最后变化时间。
func (r *instanceRepository) UpdateStatus(ctx context.Context, id uint64, status string) error {
	if id == 0 || strings.TrimSpace(status) == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.Instance{}).
		Where("id = ?", id).
		Update("status", status).Error
}
