// Package repository 提供第三方登录的数据访问实现（doc104 §6）。
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/oauth/dto"
	"hostsent/backend/internal/modules/uc/oauth/model"
)

// ErrOpenIDTaken 该 (provider, openid) 已被其他用户绑定。
var ErrOpenIDTaken = errors.New("该第三方账号已绑定其他用户")

// Repository 第三方登录仓储。
type Repository interface {
	// —— 渠道配置 ——
	ListProviders(ctx context.Context) ([]model.OAuthProvider, error)
	ListEnabledProviders(ctx context.Context) ([]model.OAuthProvider, error)
	FindProvider(ctx context.Context, provider string) (*model.OAuthProvider, error)
	CreateProvider(ctx context.Context, row *model.OAuthProvider) error
	UpdateProvider(ctx context.Context, row *model.OAuthProvider) error

	// —— 绑定关系 ——
	FindBinding(ctx context.Context, provider, openid string) (*model.UserOAuthBinding, error)
	ListBindingsByUser(ctx context.Context, userID uint64) ([]model.UserOAuthBinding, error)
	// CreateBinding 写入绑定；命中唯一索引时返回 ErrOpenIDTaken（并发抢绑的最后一道闸）。
	CreateBinding(ctx context.Context, binding *model.UserOAuthBinding) error
	DeleteBinding(ctx context.Context, userID uint64, provider string) error
	UpdateBindingLogin(ctx context.Context, id uint64, at time.Time) error
	// UpdateBindingProfile 刷新绑定时同步外部昵称/头像/unionid（用户在三方改过资料）。
	UpdateBindingProfile(ctx context.Context, id uint64, nickname, avatar, unionid string) error
	// UpdateProviderBinding 整行保存绑定（绑定幂等重入时刷新资料与状态）。
	UpdateProviderBinding(ctx context.Context, binding *model.UserOAuthBinding) error
	// CountActiveBindings 该用户的有效绑定数（解绑守卫用）。
	CountActiveBindings(ctx context.Context, userID uint64) (int64, error)
	// SyncPrimarySnapshot 把主绑定快照写回 users.oauth_provider / oauth_openid。
	SyncPrimarySnapshot(ctx context.Context, userID uint64, provider, openid string) error

	// —— 管理端绑定排查 ——
	ListBindings(ctx context.Context, query dto.BindingListQuery) ([]dto.BindingInfo, int64, error)
}

type repository struct{ db *gorm.DB }

// NewRepository 创建第三方登录仓储。
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ListProviders(ctx context.Context) ([]model.OAuthProvider, error) {
	var rows []model.OAuthProvider
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) ListEnabledProviders(ctx context.Context) ([]model.OAuthProvider, error) {
	var rows []model.OAuthProvider
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL AND enabled = true").
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) FindProvider(ctx context.Context, provider string) (*model.OAuthProvider, error) {
	var row model.OAuthProvider
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND deleted_at IS NULL", provider).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *repository) CreateProvider(ctx context.Context, row *model.OAuthProvider) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *repository) UpdateProvider(ctx context.Context, row *model.OAuthProvider) error {
	return r.db.WithContext(ctx).Save(row).Error
}

func (r *repository) FindBinding(ctx context.Context, provider, openid string) (*model.UserOAuthBinding, error) {
	var row model.UserOAuthBinding
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND openid = ?", provider, openid).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *repository) ListBindingsByUser(ctx context.Context, userID uint64) ([]model.UserOAuthBinding, error) {
	var rows []model.UserOAuthBinding
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("bound_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CreateBinding 写入绑定。
//
// 依赖 uk_user_oauth_bindings_provider_openid 唯一索引而不是「先查后写」：
// 两个请求同时用同一个微信 openid 登录时，先查后写会双写成功（两个用户都绑上），
// 唯一索引是唯一可靠的闸门。命中时翻译成 ErrOpenIDTaken。
func (r *repository) CreateBinding(ctx context.Context, binding *model.UserOAuthBinding) error {
	err := r.db.WithContext(ctx).Create(binding).Error
	if err == nil {
		return nil
	}
	if isUniqueViolation(err) {
		return ErrOpenIDTaken
	}
	return err
}

func (r *repository) DeleteBinding(ctx context.Context, userID uint64, provider string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		Delete(&model.UserOAuthBinding{}).Error
}

func (r *repository) UpdateBindingLogin(ctx context.Context, id uint64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.UserOAuthBinding{}).
		Where("id = ?", id).
		Update("last_login_at", at).Error
}

// UpdateBindingProfile 刷新外部资料。用 Updates(map) 而非 Save：绑定行的
// user_id/provider/openid 是不可变身份标识，整行保存有被内存态覆盖的风险。
func (r *repository) UpdateBindingProfile(ctx context.Context, id uint64, nickname, avatar, unionid string) error {
	return r.db.WithContext(ctx).Model(&model.UserOAuthBinding{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"nickname": nickname,
			"avatar":   avatar,
			"unionid":  unionid,
		}).Error
}

// UpdateProviderBinding 整行保存（幂等重入路径：状态可能从 disabled 回到 active）。
func (r *repository) UpdateProviderBinding(ctx context.Context, binding *model.UserOAuthBinding) error {
	return r.db.WithContext(ctx).Model(&model.UserOAuthBinding{}).
		Where("id = ?", binding.ID).
		Updates(map[string]any{
			"nickname": binding.Nickname,
			"avatar":   binding.Avatar,
			"unionid":  binding.UnionID,
			"status":   binding.Status,
		}).Error
}

func (r *repository) CountActiveBindings(ctx context.Context, userID uint64) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.UserOAuthBinding{}).
		Where("user_id = ? AND status = ?", userID, model.StatusActive).
		Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// SyncPrimarySnapshot 把主绑定快照写回 users。
//
// provider 为空表示「已无任何绑定」，此时清空两列（管理端列表的第三方图标列
// 与详情页的「主绑定」都读这两列）。
func (r *repository) SyncPrimarySnapshot(ctx context.Context, userID uint64, provider, openid string) error {
	return r.db.WithContext(ctx).Model(&userSnapshotModel{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"oauth_provider": provider,
			"oauth_openid":   openid,
		}).Error
}

// userSnapshotModel 只用于定位 users 表，不承载字段语义。
type userSnapshotModel struct{}

func (userSnapshotModel) TableName() string { return "users" }

// ListBindings 管理端绑定排查列表（按渠道 / 用户 / 关键词筛选）。
//
// 走 JOIN users 带出用户名：绑定行本身只有 user_id，运营排查时看 ID 无意义。
// users 已注销的行仍要能查到（排查「某账号注销前绑过什么」），故不额外过滤 deleted_at。
func (r *repository) ListBindings(ctx context.Context, query dto.BindingListQuery) ([]dto.BindingInfo, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Table("user_oauth_bindings AS b").
		Joins("LEFT JOIN users AS u ON u.id = b.user_id")

	if provider := strings.TrimSpace(query.Provider); provider != "" {
		base = base.Where("b.provider = ?", provider)
	}
	if query.UserID > 0 {
		base = base.Where("b.user_id = ?", query.UserID)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("b.status = ?", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("u.username ILIKE ? OR b.nickname ILIKE ? OR b.openid ILIKE ?", like, like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []dto.BindingInfo
	if err := base.
		Select("b.id, b.user_id, COALESCE(u.username, '') AS username, b.provider, b.openid, b.unionid, b.nickname, b.avatar, b.status, b.bound_at, b.last_login_at").
		Order("b.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
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

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key value")
}
