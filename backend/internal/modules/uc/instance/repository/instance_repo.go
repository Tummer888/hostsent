// Package repository 提供用户中心主机模块的数据访问实现。
package repository

import (
	"context"
	"errors"
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
//
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
