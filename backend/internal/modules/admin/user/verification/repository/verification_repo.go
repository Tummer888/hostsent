// Package repository 提供实名认证模块的数据访问实现。
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/verification/dto"
	"hostsent/backend/internal/modules/admin/user/verification/model"
)

// ErrConfigNotFound 配置项不存在。
var ErrConfigNotFound = errors.New("实名配置项不存在")

// ErrProviderNotFound 服务商不存在。
var ErrProviderNotFound = errors.New("实名核验服务商不存在")

// VerificationRepository 定义实名认证记录查询能力。
type VerificationRepository interface {
	ListByStatus(ctx context.Context, status string, query dto.VerificationListQuery) ([]model.VerificationApplication, int64, error)
	// FindByID 取单条申请（doc104 §5.7 详情）。
	FindByID(ctx context.Context, id uint64) (*model.VerificationApplication, error)
	// LatestByUser 取某用户最近一次申请（用户端状态卡与冷却期判定用）。
	LatestByUser(ctx context.Context, userID uint64) (*model.VerificationApplication, error)
	// ListByUser 取某用户的申请历史（分页）。
	ListByUser(ctx context.Context, userID uint64, page, pageSize int) ([]model.VerificationApplication, int64, error)
	// Create 新建申请。
	Create(ctx context.Context, app *model.VerificationApplication) error
	// UpdateReview 整单审核落库（状态 + 审核人 + 理由），并在同一事务里写审核日志。
	UpdateReview(ctx context.Context, app *model.VerificationApplication, log *model.VerificationReviewLog) error
	// UpdateProviderResult 写回三方核验结果。
	UpdateProviderResult(ctx context.Context, app *model.VerificationApplication) error
	// WriteReviewLog 单独写一条审核日志（submit / provider_* 等不改状态的动作用）。
	WriteReviewLog(ctx context.Context, log *model.VerificationReviewLog) error
	// ListReviewLogs 取某申请的审核轨迹。
	ListReviewLogs(ctx context.Context, applicationID uint64) ([]model.VerificationReviewLog, error)
	// NextReviewRound 该用户下一次提交的轮次（历史最大轮次 + 1）。
	NextReviewRound(ctx context.Context, userID uint64) (int, error)
	// ListDocuments 取申请的附件列表。
	ListDocuments(ctx context.Context, applicationID uint64) ([]model.VerificationDocument, error)
	// SaveDocument 写入附件（同类型覆盖，doc104 §5.7 材料上传）。
	SaveDocument(ctx context.Context, row *model.VerificationDocument) error
	// FindDocument 取单条附件（下载鉴权用）。
	FindDocument(ctx context.Context, id uint64) (*model.VerificationDocument, error)
	// SaveEnterprise 写入企业认证扩展信息（同一申请重复提交时覆盖）。
	SaveEnterprise(ctx context.Context, row *model.VerificationEnterprise) error
	// FindEnterprise 取申请的企业扩展信息（个人认证返回 ErrRecordNotFound）。
	FindEnterprise(ctx context.Context, applicationID uint64) (*model.VerificationEnterprise, error)
	// CountPending 待审核数量（总览卡片用）。
	CountPending(ctx context.Context) (int64, error)

	// —— 配置（verification_configs）——
	ListConfigs(ctx context.Context, query dto.VerificationConfigListQuery) ([]model.VerificationConfig, error)
	FindConfigByKey(ctx context.Context, key string) (*model.VerificationConfig, error)
	SaveConfig(ctx context.Context, row *model.VerificationConfig) error
	DeleteConfig(ctx context.Context, id uint64) error

	// —— 服务商（realname_providers）——
	ListProviders(ctx context.Context) ([]model.RealnameProvider, error)
	FindProviderByID(ctx context.Context, id uint64) (*model.RealnameProvider, error)
	FindDefaultProvider(ctx context.Context, providerType string) (*model.RealnameProvider, error)
	CreateProvider(ctx context.Context, row *model.RealnameProvider) error
	UpdateProvider(ctx context.Context, row *model.RealnameProvider) error
	DeleteProvider(ctx context.Context, id uint64) error
	ClearDefaultProviders(ctx context.Context, keepID uint64) error

	// —— 用户实名信任信号 ——
	// MarkUserVerified 写 users.real_name_verified_at（实名状态的唯一信任来源）。
	MarkUserVerified(ctx context.Context, userID uint64, at time.Time, source string) error
	// ClearUserVerified 撤销实名（清空信任信号）。
	ClearUserVerified(ctx context.Context, userID uint64) error
	// IsUserVerified 实名判定的唯一口径：只认 real_name_verified_at 非空。
	IsUserVerified(ctx context.Context, userID uint64) (bool, error)
}

type verificationRepository struct {
	db *gorm.DB
}

// NewVerificationRepository 创建实名认证记录仓储。
func NewVerificationRepository(db *gorm.DB) VerificationRepository {
	return &verificationRepository{db: db}
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

// ListByStatus 查询指定状态的实名认证记录列表。
func (r *verificationRepository) ListByStatus(ctx context.Context, status string, query dto.VerificationListQuery) ([]model.VerificationApplication, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.VerificationApplication{}).Where("status = ?", status)
	base = applyListFilters(base, query)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.VerificationApplication
	if err := base.Order("submitted_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func applyListFilters(base *gorm.DB, query dto.VerificationListQuery) *gorm.DB {
	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}
	if username := strings.TrimSpace(query.Username); username != "" {
		base = base.Where("username ILIKE ?", "%"+username+"%")
	}
	if verificationType := strings.TrimSpace(query.VerificationType); verificationType != "" {
		base = base.Where("verification_type = ?", verificationType)
	}
	if reviewerName := strings.TrimSpace(query.ReviewerName); reviewerName != "" {
		base = base.Where("reviewer_name ILIKE ?", "%"+reviewerName+"%")
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("username ILIKE ? OR subject_name ILIKE ? OR real_name ILIKE ? OR id_number_masked ILIKE ?", like, like, like, like)
	}
	if startTime := strings.TrimSpace(query.StartTime); startTime != "" {
		if parsed, err := time.Parse(time.RFC3339, startTime); err == nil {
			base = base.Where("submitted_at >= ?", parsed)
		}
	}
	if endTime := strings.TrimSpace(query.EndTime); endTime != "" {
		if parsed, err := time.Parse(time.RFC3339, endTime); err == nil {
			base = base.Where("submitted_at <= ?", parsed)
		}
	}
	return base
}

func (r *verificationRepository) FindByID(ctx context.Context, id uint64) (*model.VerificationApplication, error) {
	var app model.VerificationApplication
	if err := r.db.WithContext(ctx).First(&app, id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *verificationRepository) LatestByUser(ctx context.Context, userID uint64) (*model.VerificationApplication, error) {
	var app model.VerificationApplication
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("submitted_at DESC, id DESC").
		First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *verificationRepository) ListByUser(ctx context.Context, userID uint64, page, pageSize int) ([]model.VerificationApplication, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	base := r.db.WithContext(ctx).Model(&model.VerificationApplication{}).Where("user_id = ?", userID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.VerificationApplication
	if err := base.Order("submitted_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *verificationRepository) Create(ctx context.Context, app *model.VerificationApplication) error {
	return r.db.WithContext(ctx).Create(app).Error
}

// UpdateReview 整单审核落库。
//
// 状态流转与审核日志写在同一个事务里：只有状态变了却没有轨迹（或反之）都会让
// 「谁在什么时候放行的」无法回答，而这正是合规审计要问的第一个问题。
func (r *verificationRepository) UpdateReview(ctx context.Context, app *model.VerificationApplication, log *model.VerificationReviewLog) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.VerificationApplication{}).Where("id = ?", app.ID).Updates(map[string]any{
			"status":             app.Status,
			"reviewed_at":        app.ReviewedAt,
			"reviewed_by":        app.ReviewedBy,
			"reviewer_name":      app.ReviewerName,
			"reject_reason_code": app.RejectReasonCode,
			"reject_reason":      app.RejectReason,
			"review_note":        app.ReviewNote,
		}).Error; err != nil {
			return err
		}
		if log == nil {
			return nil
		}
		return tx.Create(log).Error
	})
}

func (r *verificationRepository) UpdateProviderResult(ctx context.Context, app *model.VerificationApplication) error {
	return r.db.WithContext(ctx).Model(&model.VerificationApplication{}).Where("id = ?", app.ID).Updates(map[string]any{
		"provider":            app.Provider,
		"provider_txn_no":     app.ProviderTxnNo,
		"provider_result":     app.ProviderResult,
		"provider_message":    app.ProviderMessage,
		"provider_checked_at": app.ProviderCheckedAt,
	}).Error
}

func (r *verificationRepository) WriteReviewLog(ctx context.Context, log *model.VerificationReviewLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *verificationRepository) ListReviewLogs(ctx context.Context, applicationID uint64) ([]model.VerificationReviewLog, error) {
	var rows []model.VerificationReviewLog
	if err := r.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *verificationRepository) NextReviewRound(ctx context.Context, userID uint64) (int, error) {
	var maxRound int
	if err := r.db.WithContext(ctx).Model(&model.VerificationApplication{}).
		Where("user_id = ?", userID).
		Select("COALESCE(MAX(review_round), 0)").
		Scan(&maxRound).Error; err != nil {
		return 0, err
	}
	return maxRound + 1, nil
}

func (r *verificationRepository) ListDocuments(ctx context.Context, applicationID uint64) ([]model.VerificationDocument, error) {
	var rows []model.VerificationDocument
	if err := r.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("sort ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// SaveDocument 写入一条申请附件。
//
// 用「先查同类型再决定插入/更新」而不是无条件插入：同一张单重复提交同一类型材料
// （用户重传证件正面）应覆盖旧文件，否则详情页会出现两张证件正面图，
// 审核员无法判断哪张才是最新的。sort 在更新时保持不变，维持既有展示顺序。
func (r *verificationRepository) SaveDocument(ctx context.Context, row *model.VerificationDocument) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.VerificationDocument
		err := tx.Where("application_id = ? AND document_type = ?", row.ApplicationID, row.DocumentType).
			First(&existing).Error
		switch {
		case err == nil:
			row.ID = existing.ID
			row.Sort = existing.Sort
			return tx.Model(&existing).Updates(map[string]any{"file_url": row.FileURL}).Error
		case errors.Is(err, gorm.ErrRecordNotFound):
			return tx.Create(row).Error
		default:
			return err
		}
	})
}

// FindDocument 取单条附件（下载前做归属校验用）。
func (r *verificationRepository) FindDocument(ctx context.Context, id uint64) (*model.VerificationDocument, error) {
	var row model.VerificationDocument
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *verificationRepository) FindEnterprise(ctx context.Context, applicationID uint64) (*model.VerificationEnterprise, error) {
	var row model.VerificationEnterprise
	if err := r.db.WithContext(ctx).Where("application_id = ?", applicationID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// SaveEnterprise 写入企业扩展信息。
//
// 用 ON CONFLICT (application_id) 而不是「先查再写」：application_id 上有唯一索引，
// 并发提交下「查不到→插入」会撞唯一冲突；这里让数据库一次定胜负。
func (r *verificationRepository) SaveEnterprise(ctx context.Context, row *model.VerificationEnterprise) error {
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO verification_enterprises
			(application_id, company_name, credit_code_masked, legal_person_name, contact_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, now(), now())
		ON CONFLICT (application_id) DO UPDATE SET
			company_name = EXCLUDED.company_name,
			credit_code_masked = EXCLUDED.credit_code_masked,
			legal_person_name = EXCLUDED.legal_person_name,
			contact_name = EXCLUDED.contact_name,
			updated_at = now()`,
		row.ApplicationID, row.CompanyName, row.CreditCodeMasked,
		row.LegalPersonName, row.ContactName).Error
}

func (r *verificationRepository) CountPending(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.VerificationApplication{}).
		Where("status = ?", model.StatusPending).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// ---------- 配置 ----------

func (r *verificationRepository) ListConfigs(ctx context.Context, query dto.VerificationConfigListQuery) ([]model.VerificationConfig, error) {
	base := r.db.WithContext(ctx).Model(&model.VerificationConfig{})
	if group := strings.TrimSpace(query.ConfigGroup); group != "" {
		base = base.Where("config_group = ?", group)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	var rows []model.VerificationConfig
	if err := base.Order("config_group ASC, config_key ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *verificationRepository) FindConfigByKey(ctx context.Context, key string) (*model.VerificationConfig, error) {
	var row model.VerificationConfig
	if err := r.db.WithContext(ctx).Where("config_key = ?", key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *verificationRepository) SaveConfig(ctx context.Context, row *model.VerificationConfig) error {
	if row.ID == 0 {
		return r.db.WithContext(ctx).Create(row).Error
	}
	return r.db.WithContext(ctx).Model(&model.VerificationConfig{}).Where("id = ?", row.ID).Updates(map[string]any{
		"config_value": row.ConfigValue,
		"value_type":   row.ValueType,
		"description":  row.Description,
		"status":       row.Status,
		"updated_by":   row.UpdatedBy,
	}).Error
}

func (r *verificationRepository) DeleteConfig(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.VerificationConfig{}, id).Error
}

// ---------- 服务商 ----------

func (r *verificationRepository) ListProviders(ctx context.Context) ([]model.RealnameProvider, error) {
	var rows []model.RealnameProvider
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("priority DESC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *verificationRepository) FindProviderByID(ctx context.Context, id uint64) (*model.RealnameProvider, error) {
	var row model.RealnameProvider
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return &row, nil
}

// FindDefaultProvider 取某类型下的默认 provider。
//
// 优先 is_default=true；没有默认标记时回落该类型下优先级最高的一条 ——
// 「配了凭证但忘了勾默认」不该让核验直接不可用。
func (r *verificationRepository) FindDefaultProvider(ctx context.Context, providerType string) (*model.RealnameProvider, error) {
	var row model.RealnameProvider
	err := r.db.WithContext(ctx).
		Where("provider_type = ? AND status = 1 AND deleted_at IS NULL", providerType).
		Order("is_default DESC, priority DESC, id ASC").
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *verificationRepository) CreateProvider(ctx context.Context, row *model.RealnameProvider) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *verificationRepository) UpdateProvider(ctx context.Context, row *model.RealnameProvider) error {
	return r.db.WithContext(ctx).Model(&model.RealnameProvider{}).Where("id = ?", row.ID).Updates(map[string]any{
		"name":          row.Name,
		"mode":          row.Mode,
		"descriptor":    row.Descriptor,
		"credentials":   row.Credentials,
		"endpoint":      row.Endpoint,
		"priority":      row.Priority,
		"health_status": row.HealthStatus,
		"last_error":    row.LastError,
		"last_check_at": row.LastCheckAt,
		"status":        row.Status,
		"is_default":    row.IsDefault,
		"remark":        row.Remark,
	}).Error
}

func (r *verificationRepository) DeleteProvider(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.RealnameProvider{}).Where("id = ?", id).
		Updates(map[string]any{"deleted_at": now, "is_default": false}).Error
}

// ClearDefaultProviders 同类型只保留一个默认（部分唯一索引 uk_realname_providers_default_per_type）。
func (r *verificationRepository) ClearDefaultProviders(ctx context.Context, keepID uint64) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE realname_providers SET is_default = false
		WHERE is_default = true AND deleted_at IS NULL AND id <> ?
		  AND provider_type = (SELECT provider_type FROM realname_providers WHERE id = ?)`,
		keepID, keepID).Error
}

// ---------- 用户实名信任信号 ----------

// MarkUserVerified 写实名信任信号。
//
// 只写 real_name_verified_at / real_name_verified_source 两列：users.real_name
// 是展示名（用户可自助改），认证状态必须与它解耦（doc104 §5.3）。
func (r *verificationRepository) MarkUserVerified(ctx context.Context, userID uint64, at time.Time, source string) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE users SET real_name_verified_at = ?, real_name_verified_source = ? WHERE id = ?`,
		at, source, userID).Error
}

// ClearUserVerified 撤销实名。
func (r *verificationRepository) ClearUserVerified(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE users SET real_name_verified_at = NULL, real_name_verified_source = '' WHERE id = ?`,
		userID).Error
}

// IsUserVerified 实名判定的唯一口径（doc104 §5.3 F13）。
//
// 刻意只查 real_name_verified_at：users.real_name 是展示名，用户在资料页
// 就能改，用它判断实名等于给了一条自助「已实名」的后门。
//
// 用 COUNT 而不是 `Scan(&*time.Time)`：把 SQL NULL 扫进指针类型的行为依赖驱动，
// database/sql 会直接报 "unsupported Scan, storing driver.Value type <nil> into
// type *time.Time"（实测），于是「未实名」这条最常见的路径变成查询失败。
// 判定「有没有」用计数最直白，也不依赖驱动实现。
func (r *verificationRepository) IsUserVerified(ctx context.Context, userID uint64) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Table("users").
		Where("id = ? AND real_name_verified_at IS NOT NULL", userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
