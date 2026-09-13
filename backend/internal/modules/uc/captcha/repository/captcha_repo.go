// Package repository 提供验证码与二次验证体系的数据访问（doc91 §1）。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/captcha/model"
)

// Repository 验证码/策略/服务商/用户安全设置的数据访问。
type Repository interface {
	// 场景策略
	FindPolicyByScene(ctx context.Context, scene string) (*model.CaptchaPolicy, error)
	ListPolicies(ctx context.Context) ([]model.CaptchaPolicy, error)
	SavePolicy(ctx context.Context, policy *model.CaptchaPolicy) error

	// 服务商
	ListProviders(ctx context.Context) ([]model.CaptchaProvider, error)
	FindProviderByID(ctx context.Context, id uint64) (*model.CaptchaProvider, error)
	FindDefaultProvider(ctx context.Context) (*model.CaptchaProvider, error)
	CreateProvider(ctx context.Context, p *model.CaptchaProvider) error
	UpdateProvider(ctx context.Context, p *model.CaptchaProvider) error
	DeleteProvider(ctx context.Context, id uint64) error
	ClearDefaultProviders(ctx context.Context, exceptID uint64) error

	// 验证码记录
	CreateCode(ctx context.Context, c *model.VerificationCode) error
	// FindLatestPending 取某场景某目标最新一条未使用且未过期的记录（Redis 降级读）。
	FindLatestPending(ctx context.Context, scene, targetHash string) (*model.VerificationCode, error)
	MarkCodeUsed(ctx context.Context, id uint64) error
	IncrCodeAttempts(ctx context.Context, id uint64) error
	// CountSentSince 统计某场景某目标在窗口内的下发次数（频控降级）。
	CountSentSince(ctx context.Context, scene, targetHash string, since time.Time) (int64, error)

	// 用户安全设置
	FindSettings(ctx context.Context, userID uint64) (*model.UserSecuritySettings, error)
	SaveSettings(ctx context.Context, s *model.UserSecuritySettings) error

	// 管理端 MFA 设置（admins 表扩展列）
	FindAdminMFA(ctx context.Context, adminID uint64) (*AdminMFA, error)
	SaveAdminMFA(ctx context.Context, m *AdminMFA) error

	// 统计
	StatsByScene(ctx context.Context, from, to time.Time) ([]SceneStat, error)
}

// AdminMFA 管理端二次验证设置（admins 表的 mfa_* 三列）。
type AdminMFA struct {
	AdminID       uint64
	MFAEnabled    bool
	MFAChannel    string
	SceneOverride string // jsonb 原文
}

// SceneStat 按场景聚合的验证码统计。
type SceneStat struct {
	Scene   string
	Total   int64
	Used    int64
	Failed  int64
	Expired int64
	CostFen int64
}

type repository struct {
	db *gorm.DB
}

// NewRepository 创建仓储实现。
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindPolicyByScene(ctx context.Context, scene string) (*model.CaptchaPolicy, error) {
	var p model.CaptchaPolicy
	if err := r.db.WithContext(ctx).Where("scene = ?", scene).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *repository) ListPolicies(ctx context.Context) ([]model.CaptchaPolicy, error) {
	var items []model.CaptchaPolicy
	if err := r.db.WithContext(ctx).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *repository) SavePolicy(ctx context.Context, policy *model.CaptchaPolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

func (r *repository) ListProviders(ctx context.Context) ([]model.CaptchaProvider, error) {
	var items []model.CaptchaProvider
	if err := r.db.WithContext(ctx).Order("priority desc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *repository) FindProviderByID(ctx context.Context, id uint64) (*model.CaptchaProvider, error) {
	var p model.CaptchaProvider
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *repository) FindDefaultProvider(ctx context.Context) (*model.CaptchaProvider, error) {
	var p model.CaptchaProvider
	if err := r.db.WithContext(ctx).Where("is_default = true AND status = 1").First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *repository) CreateProvider(ctx context.Context, p *model.CaptchaProvider) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *repository) UpdateProvider(ctx context.Context, p *model.CaptchaProvider) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *repository) DeleteProvider(ctx context.Context, id uint64) error {
	// 软删除：保留历史记录中的 provider_id 可追溯。
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.CaptchaProvider{}).
		Where("id = ?", id).Updates(map[string]any{"deleted_at": now, "is_default": false}).Error
}

// ClearDefaultProviders 取消其他行的默认标记（每 type 至多一个默认，由部分唯一索引兜底）。
func (r *repository) ClearDefaultProviders(ctx context.Context, exceptID uint64) error {
	return r.db.WithContext(ctx).Model(&model.CaptchaProvider{}).
		Where("id <> ? AND is_default = true", exceptID).
		Update("is_default", false).Error
}

func (r *repository) CreateCode(ctx context.Context, c *model.VerificationCode) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *repository) FindLatestPending(ctx context.Context, scene, targetHash string) (*model.VerificationCode, error) {
	var c model.VerificationCode
	err := r.db.WithContext(ctx).
		Where("scene = ? AND target_hash = ? AND status = ? AND expire_at > ?",
			scene, targetHash, model.CodeStatusPending, time.Now()).
		Order("id desc").First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *repository) MarkCodeUsed(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.VerificationCode{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": model.CodeStatusUsed, "used_at": now}).Error
}

func (r *repository) IncrCodeAttempts(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.VerificationCode{}).
		Where("id = ?", id).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
}

func (r *repository) CountSentSince(ctx context.Context, scene, targetHash string, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.VerificationCode{}).
		Where("scene = ? AND target_hash = ? AND created_at > ?", scene, targetHash, since).
		Count(&n).Error
	return n, err
}

func (r *repository) FindSettings(ctx context.Context, userID uint64) (*model.UserSecuritySettings, error) {
	var s model.UserSecuritySettings
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *repository) SaveSettings(ctx context.Context, s *model.UserSecuritySettings) error {
	var existing model.UserSecuritySettings
	err := r.db.WithContext(ctx).Where("user_id = ?", s.UserID).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return r.db.WithContext(ctx).Create(s).Error
	case err != nil:
		return err
	default:
		s.ID = existing.ID
		return r.db.WithContext(ctx).Save(s).Error
	}
}

func (r *repository) FindAdminMFA(ctx context.Context, adminID uint64) (*AdminMFA, error) {
	var out AdminMFA
	err := r.db.WithContext(ctx).Table("admins").
		Select("id AS admin_id, mfa_enabled, mfa_channel, COALESCE(mfa_scene_overrides::text, '') AS scene_override").
		Where("id = ?", adminID).Scan(&out).Error
	if err != nil {
		return nil, err
	}
	if out.AdminID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &out, nil
}

func (r *repository) SaveAdminMFA(ctx context.Context, m *AdminMFA) error {
	updates := map[string]any{
		"mfa_enabled": m.MFAEnabled,
		"mfa_channel": m.MFAChannel,
	}
	if m.SceneOverride == "" {
		updates["mfa_scene_overrides"] = nil
	} else {
		updates["mfa_scene_overrides"] = gorm.Expr("?::jsonb", m.SceneOverride)
	}
	return r.db.WithContext(ctx).Table("admins").Where("id = ?", m.AdminID).Updates(updates).Error
}

func (r *repository) StatsByScene(ctx context.Context, from, to time.Time) ([]SceneStat, error) {
	var out []SceneStat
	err := r.db.WithContext(ctx).Model(&model.VerificationCode{}).
		Select(`scene,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'used') AS used,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			COUNT(*) FILTER (WHERE status = 'expired') AS expired,
			COALESCE(SUM(cost_fen), 0) AS cost_fen`).
		Where("created_at >= ? AND created_at < ?", from, to).
		Group("scene").
		Order("total DESC").
		Scan(&out).Error
	return out, err
}
