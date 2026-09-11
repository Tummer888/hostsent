// Package repository 提供开放平台的数据访问（P6/T6.1）。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	openmodel "hostsent/backend/internal/modules/open/model"
)

// ErrAppNotFound 应用不存在。
var ErrAppNotFound = errors.New("open: app not found")

// ResolvedApp 网关鉴权所需的完整应用视图：应用 + 能力位 + IP 规则。
type ResolvedApp struct {
	App     openmodel.OpenApp
	Scopes  []openmodel.OpenAppScope
	IPRules []openmodel.OpenAppIPRule

	scopeSet map[string]struct{}
}

// NewResolvedApp 装配应用视图并构建 scope 索引。
func NewResolvedApp(app openmodel.OpenApp, scopes []openmodel.OpenAppScope, ipRules []openmodel.OpenAppIPRule) *ResolvedApp {
	res := &ResolvedApp{App: app, Scopes: scopes, IPRules: ipRules, scopeSet: make(map[string]struct{}, len(scopes))}
	for _, s := range scopes {
		res.scopeSet[s.Scope] = struct{}{}
	}
	return res
}

// HasScope 判断应用是否持有某能力位（只看 scope 码；object 级范围限定由业务层处理）。
func (r *ResolvedApp) HasScope(scope string) bool {
	if r.scopeSet == nil {
		return false
	}
	_, ok := r.scopeSet[scope]
	return ok
}

// ScopeNames 返回去重后的 scope 码列表。
func (r *ResolvedApp) ScopeNames() []string {
	names := make([]string, 0, len(r.Scopes))
	seen := map[string]struct{}{}
	for _, s := range r.Scopes {
		if _, ok := seen[s.Scope]; ok {
			continue
		}
		seen[s.Scope] = struct{}{}
		names = append(names, s.Scope)
	}
	return names
}

// AppRepository 开放应用仓储：网关解析 + CLI 管理两用。
// 同时实现 APILogRepository（审计日志写入）。
type AppRepository interface {
	APILogRepository
	// ResolveByAppID 按 app_id 装载应用及其能力位、IP 规则。
	ResolveByAppID(ctx context.Context, appID string) (*ResolvedApp, error)
	// CreateApp 新建应用（含能力位，事务内）。
	CreateApp(ctx context.Context, app *openmodel.OpenApp, scopes []string) error
	// ListApps 列出全部应用（CLI 管理用）。
	ListApps(ctx context.Context) ([]openmodel.OpenApp, error)
	// GetByAppID 读取单个应用。
	GetByAppID(ctx context.Context, appID string) (*openmodel.OpenApp, error)
	// GetByID 按主键读取单个应用（回调投递时解析 notify_url/notify_secret）。
	GetByID(ctx context.Context, id uint64) (*openmodel.OpenApp, error)
	// ListNotifiableByOwner 归属账号下启用且配置了回调地址的应用（事件发布目标）。
	ListNotifiableByOwner(ctx context.Context, ownerUserID uint64) ([]openmodel.OpenApp, error)
	// UpdateStatus 启用/停用。
	UpdateStatus(ctx context.Context, appID string, status int) error
	// RotateSecret 重置 app_secret（传入加密后的密文）。
	RotateSecret(ctx context.Context, appID, encryptedSecret string) error
	// SetNotify 更新回调地址与 notify_secret（T6.5；secret 为空则保持不变）。
	SetNotify(ctx context.Context, appID, notifyURL, encryptedSecret string) error
	// AddIPRules / DeleteIPRules / ListIPRules CLI 维护白名单。
	AddIPRules(ctx context.Context, appID string, cidrs []string) error
	DeleteIPRules(ctx context.Context, appID string) error
	ListIPRules(ctx context.Context, appID string) ([]openmodel.OpenAppIPRule, error)
}

// APILogRepository 开放 API 审计日志写入（网关 Audit 中间件使用）。
type APILogRepository interface {
	WriteLog(ctx context.Context, log *openmodel.OpenAPILog) error
}

type appRepository struct {
	db *gorm.DB
}

// NewAppRepository 构造开放应用仓储。
func NewAppRepository(db *gorm.DB) AppRepository {
	return &appRepository{db: db}
}

func (r *appRepository) ResolveByAppID(ctx context.Context, appID string) (*ResolvedApp, error) {
	var app openmodel.OpenApp
	if err := r.db.WithContext(ctx).Where("app_id = ?", appID).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	var scopes []openmodel.OpenAppScope
	if err := r.db.WithContext(ctx).Where("app_id = ?", app.ID).Find(&scopes).Error; err != nil {
		return nil, err
	}
	var rules []openmodel.OpenAppIPRule
	if err := r.db.WithContext(ctx).Where("app_id = ?", app.ID).Find(&rules).Error; err != nil {
		return nil, err
	}
	res := NewResolvedApp(app, scopes, rules)
	return res, nil
}

func (r *appRepository) CreateApp(ctx context.Context, app *openmodel.OpenApp, scopes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(app).Error; err != nil {
			return err
		}
		if len(scopes) == 0 {
			return nil
		}
		rows := make([]openmodel.OpenAppScope, 0, len(scopes))
		for _, s := range scopes {
			rows = append(rows, openmodel.OpenAppScope{AppID: app.ID, Scope: s})
		}
		return tx.Create(&rows).Error
	})
}

func (r *appRepository) ListApps(ctx context.Context) ([]openmodel.OpenApp, error) {
	var apps []openmodel.OpenApp
	err := r.db.WithContext(ctx).Order("id").Find(&apps).Error
	return apps, err
}

func (r *appRepository) GetByAppID(ctx context.Context, appID string) (*openmodel.OpenApp, error) {
	var app openmodel.OpenApp
	if err := r.db.WithContext(ctx).Where("app_id = ?", appID).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	return &app, nil
}

func (r *appRepository) GetByID(ctx context.Context, id uint64) (*openmodel.OpenApp, error) {
	var app openmodel.OpenApp
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	return &app, nil
}

func (r *appRepository) ListNotifiableByOwner(ctx context.Context, ownerUserID uint64) ([]openmodel.OpenApp, error) {
	var apps []openmodel.OpenApp
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND status = ? AND notify_url <> ''", ownerUserID, openmodel.OpenAppStatusEnabled).
		Find(&apps).Error
	return apps, err
}

func (r *appRepository) UpdateStatus(ctx context.Context, appID string, status int) error {
	return r.db.WithContext(ctx).Model(&openmodel.OpenApp{}).
		Where("app_id = ?", appID).
		Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
}

func (r *appRepository) RotateSecret(ctx context.Context, appID, encryptedSecret string) error {
	return r.db.WithContext(ctx).Model(&openmodel.OpenApp{}).
		Where("app_id = ?", appID).
		Updates(map[string]any{"app_secret": encryptedSecret, "updated_at": time.Now()}).Error
}

func (r *appRepository) SetNotify(ctx context.Context, appID, notifyURL, encryptedSecret string) error {
	updates := map[string]any{"updated_at": time.Now()}
	if notifyURL != "" {
		updates["notify_url"] = notifyURL
	}
	if encryptedSecret != "" {
		updates["notify_secret"] = encryptedSecret
	}
	return r.db.WithContext(ctx).Model(&openmodel.OpenApp{}).
		Where("app_id = ?", appID).Updates(updates).Error
}

func (r *appRepository) AddIPRules(ctx context.Context, appID string, cidrs []string) error {
	app, err := r.GetByAppID(ctx, appID)
	if err != nil {
		return err
	}
	rows := make([]openmodel.OpenAppIPRule, 0, len(cidrs))
	for _, c := range cidrs {
		rows = append(rows, openmodel.OpenAppIPRule{AppID: app.ID, CIDR: c})
	}
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}

func (r *appRepository) DeleteIPRules(ctx context.Context, appID string) error {
	return r.db.WithContext(ctx).Where("app_id = (SELECT id FROM open_apps WHERE app_id = ?)", appID).
		Delete(&openmodel.OpenAppIPRule{}).Error
}

func (r *appRepository) ListIPRules(ctx context.Context, appID string) ([]openmodel.OpenAppIPRule, error) {
	var rules []openmodel.OpenAppIPRule
	err := r.db.WithContext(ctx).
		Where("app_id = (SELECT id FROM open_apps WHERE app_id = ?)", appID).
		Order("id").Find(&rules).Error
	return rules, err
}

// WriteLog 落一条 API 审计日志（网关异步调用，失败由调用方记日志）。
func (r *appRepository) WriteLog(ctx context.Context, log *openmodel.OpenAPILog) error {
	return r.db.WithContext(ctx).Create(log).Error
}
