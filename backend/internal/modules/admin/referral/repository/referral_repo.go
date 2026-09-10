// Package repository 提供推广邀请返现子域的数据访问实现。
package repository

import (
	"context"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/referral/model"
)

// TxFilter 返现台账查询条件。
type TxFilter struct {
	UserID   uint64 // 返现归属人（邀请人）
	Type     string // 台账类型，空表示全部
	Page     int
	PageSize int
}

// WithdrawFilter 提现单查询条件。
type WithdrawFilter struct {
	UserID   uint64
	Status   string
	Page     int
	PageSize int
}

// InviteeRow 我的邀请列表行（邀请人视角）。
type InviteeRow struct {
	UserID        uint64  `json:"user_id"`
	Username      string  `json:"username"`
	Email         string  `json:"email"`
	InvitedAt     string  `json:"invited_at"`
	CashbackTotal float64 `json:"cashback_total"`
	OrderCount    int64   `json:"order_count"`
}

// TxRow 返现台账展示行：附带被邀请人用户名与邀请人用户名。
type TxRow struct {
	model.ReferralTransaction
	InviteeUsername string `json:"invitee_username" gorm:"column:invitee_username"`
	InviterUsername string `json:"inviter_username" gorm:"column:inviter_username"`
}

// WithdrawalRow 提现单展示行：附带申请人用户名。
type WithdrawalRow struct {
	model.ReferralWithdrawal
	Username string `json:"username" gorm:"column:username"`
}

// ReferralRepository 推广返现数据访问。
// 传入的 db 可为普通连接或事务连接，以支持在事务内完成加锁与变更。
type ReferralRepository interface {
	// —— 返现账户 ——
	LockAccount(db *gorm.DB, userID uint64) (*model.ReferralAccount, error)
	// EnsureAccount 幂等建户（并发安全）并返回加锁后的账户。
	EnsureAccount(db *gorm.DB, userID uint64) (*model.ReferralAccount, error)
	FindAccount(ctx context.Context, userID uint64) (*model.ReferralAccount, error)
	CreateAccount(db *gorm.DB, acc *model.ReferralAccount) error
	UpdateAccount(db *gorm.DB, acc *model.ReferralAccount) error

	// —— 返现台账 ——
	FindTxByBiz(db *gorm.DB, userID uint64, bizType, refNo string) (*model.ReferralTransaction, error)
	CreateTx(db *gorm.DB, tx *model.ReferralTransaction) error
	ListTxRows(ctx context.Context, f TxFilter) ([]TxRow, int64, error)
	SumTxAmount(ctx context.Context, userID, orderID uint64, types ...string) (float64, error)
	ExistsIncomeForInvitee(ctx context.Context, inviteeUserID uint64) (bool, error)

	// —— 邀请关系（单级，落在 users.inviter_user_id）——
	FindInviterID(ctx context.Context, inviteeUserID uint64) (uint64, error)
	FindIDByInviteCode(ctx context.Context, code string) (uint64, error)
	SetInviteCode(ctx context.Context, userID uint64, code string) error
	EnsureInviteCode(ctx context.Context, userID uint64, code string) (string, error)
	BindInviter(ctx context.Context, inviteeID, inviterID uint64) error
	ListInvitees(ctx context.Context, inviterID uint64, page, pageSize int) ([]InviteeRow, int64, error)
	CountInvitees(ctx context.Context, inviterID uint64) (int64, error)
	InviteCodeOf(ctx context.Context, userID uint64) (string, error)

	// —— 提现单 ——
	CreateWithdrawal(db *gorm.DB, w *model.ReferralWithdrawal) error
	FindWithdrawalByID(db *gorm.DB, id uint64) (*model.ReferralWithdrawal, error)
	FindWithdrawalByNo(db *gorm.DB, no string) (*model.ReferralWithdrawal, error)
	UpdateWithdrawal(db *gorm.DB, w *model.ReferralWithdrawal) error
	ListWithdrawalRows(ctx context.Context, f WithdrawFilter) ([]WithdrawalRow, int64, error)
	// SumPendingWithdrawals 汇总待审核提现金额（用于可用余额展示）。
	SumPendingWithdrawals(ctx context.Context, userID uint64) (float64, error)

	// —— 全局配置 ——
	Rates(ctx context.Context) (model.Rates, error)
}

type referralRepository struct {
	db *gorm.DB
}

// NewReferralRepository 创建推广返现仓储。
func NewReferralRepository(db *gorm.DB) ReferralRepository {
	return &referralRepository{db: db}
}

// —— 返现账户 ——

func (r *referralRepository) FindAccount(ctx context.Context, userID uint64) (*model.ReferralAccount, error) {
	var acc model.ReferralAccount
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

// LockAccount 以行锁（SELECT ... FOR UPDATE）读取返现账户，防止并发超额。
func (r *referralRepository) LockAccount(db *gorm.DB, userID uint64) (*model.ReferralAccount, error) {
	var acc model.ReferralAccount
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

func (r *referralRepository) CreateAccount(db *gorm.DB, acc *model.ReferralAccount) error {
	return db.Create(acc).Error
}

// EnsureAccount 先按 user_id 幂等插入（并发下唯一键冲突静默跳过），再行锁读取。
func (r *referralRepository) EnsureAccount(db *gorm.DB, userID uint64) (*model.ReferralAccount, error) {
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(&model.ReferralAccount{UserID: userID}).Error; err != nil {
		return nil, err
	}
	return r.LockAccount(db, userID)
}

func (r *referralRepository) UpdateAccount(db *gorm.DB, acc *model.ReferralAccount) error {
	return db.Save(acc).Error
}

// —— 返现台账 ——

func (r *referralRepository) FindTxByBiz(db *gorm.DB, userID uint64, bizType, refNo string) (*model.ReferralTransaction, error) {
	var tx model.ReferralTransaction
	if err := db.Where("user_id = ? AND biz_type = ? AND ref_no = ?", userID, bizType, refNo).First(&tx).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *referralRepository) CreateTx(db *gorm.DB, tx *model.ReferralTransaction) error {
	return db.Create(tx).Error
}

// ListTxRows 分页查询返现台账（关联出邀请人/被邀请人用户名）。
func (r *referralRepository) ListTxRows(ctx context.Context, f TxFilter) ([]TxRow, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	base := r.db.WithContext(ctx).Table("referral_transactions rt")
	if f.UserID > 0 {
		base = base.Where("rt.user_id = ?", f.UserID)
	}
	if f.Type != "" {
		base = base.Where("rt.type = ?", f.Type)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []TxRow
	err := r.db.WithContext(ctx).Table("referral_transactions rt").
		Select(`rt.*,
			COALESCE(inv.username, '') AS inviter_username,
			COALESCE(ie.username, '') AS invitee_username`).
		Joins("LEFT JOIN users inv ON inv.id = rt.inviter_user_id").
		Joins("LEFT JOIN users ie ON ie.id = rt.invitee_user_id").
		Where("(? = 0 OR rt.user_id = ?)", f.UserID, f.UserID).
		Where("(? = '' OR rt.type = ?)", f.Type, f.Type).
		Order("rt.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// SumTxAmount 汇总指定订单下若干类型的台账金额（绝对值）。
func (r *referralRepository) SumTxAmount(ctx context.Context, userID, orderID uint64, types ...string) (float64, error) {
	if len(types) == 0 {
		return 0, nil
	}
	var sum float64
	err := r.db.WithContext(ctx).Model(&model.ReferralTransaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND order_id = ? AND type IN ?", userID, orderID, types).
		Scan(&sum).Error
	return sum, err
}

// ExistsIncomeForInvitee 判断某被邀请人是否已有返现入账（用于首单判定）。
func (r *referralRepository) ExistsIncomeForInvitee(ctx context.Context, inviteeUserID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ReferralTransaction{}).
		Where("invitee_user_id = ? AND direction = ?", inviteeUserID, model.DirectionIncome).
		Count(&count).Error
	return count > 0, err
}

// —— 邀请关系 ——

// FindInviterID 返回被邀请人的邀请人 ID；未绑定返回 0。
func (r *referralRepository) FindInviterID(ctx context.Context, inviteeUserID uint64) (uint64, error) {
	var inviter *uint64
	err := r.db.WithContext(ctx).Table("users").
		Select("inviter_user_id").Where("id = ?", inviteeUserID).Scan(&inviter).Error
	if err != nil {
		return 0, err
	}
	if inviter == nil {
		return 0, nil
	}
	return *inviter, nil
}

// FindIDByInviteCode 按邀请码反查用户 ID；不存在返回 gorm.ErrRecordNotFound。
func (r *referralRepository) FindIDByInviteCode(ctx context.Context, code string) (uint64, error) {
	var id uint64
	err := r.db.WithContext(ctx).Table("users").
		Select("id").Where("invite_code = ?", code).Limit(1).Scan(&id).Error
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return id, nil
}

func (r *referralRepository) SetInviteCode(ctx context.Context, userID uint64, code string) error {
	return r.db.WithContext(ctx).Table("users").
		Where("id = ?", userID).Update("invite_code", code).Error
}

// EnsureInviteCode 用户邀请码为空时写入候选码，返回最终生效的邀请码。
func (r *referralRepository) EnsureInviteCode(ctx context.Context, userID uint64, code string) (string, error) {
	current, err := r.InviteCodeOf(ctx, userID)
	if err != nil {
		return "", err
	}
	if current != "" {
		return current, nil
	}
	if err := r.SetInviteCode(ctx, userID, code); err != nil {
		return "", err
	}
	return code, nil
}

func (r *referralRepository) InviteCodeOf(ctx context.Context, userID uint64) (string, error) {
	var code *string
	if err := r.db.WithContext(ctx).Table("users").
		Select("invite_code").Where("id = ?", userID).Scan(&code).Error; err != nil {
		return "", err
	}
	if code == nil {
		return "", nil
	}
	return *code, nil
}

// BindInviter 绑定邀请关系；一次性写入，已有邀请人时不覆盖。
func (r *referralRepository) BindInviter(ctx context.Context, inviteeID, inviterID uint64) error {
	return r.db.WithContext(ctx).Table("users").
		Where("id = ? AND inviter_user_id IS NULL", inviteeID).
		Updates(map[string]any{
			"inviter_user_id": inviterID,
			"invited_at":      gorm.Expr("now()"),
		}).Error
}

func (r *referralRepository) ListInvitees(ctx context.Context, inviterID uint64, page, pageSize int) ([]InviteeRow, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	base := r.db.WithContext(ctx).Table("users u").Where("u.inviter_user_id = ?", inviterID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []InviteeRow
	err := r.db.WithContext(ctx).Table("users u").
		Select(`u.id AS user_id, u.username, u.email,
			COALESCE(to_char(u.invited_at, 'YYYY-MM-DD HH24:MI:SS'), '') AS invited_at,
			COALESCE(SUM(rt.amount), 0) AS cashback_total,
			COUNT(DISTINCT rt.order_id) AS order_count`).
		Joins("LEFT JOIN referral_transactions rt ON rt.invitee_user_id = u.id AND rt.direction = 1").
		Where("u.inviter_user_id = ?", inviterID).
		Group("u.id, u.username, u.email, u.invited_at").
		Order("u.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *referralRepository) CountInvitees(ctx context.Context, inviterID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("users").Where("inviter_user_id = ?", inviterID).Count(&count).Error
	return count, err
}

// —— 提现单 ——

func (r *referralRepository) CreateWithdrawal(db *gorm.DB, w *model.ReferralWithdrawal) error {
	return db.Create(w).Error
}

func (r *referralRepository) FindWithdrawalByID(db *gorm.DB, id uint64) (*model.ReferralWithdrawal, error) {
	var w model.ReferralWithdrawal
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&w, id).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *referralRepository) FindWithdrawalByNo(db *gorm.DB, no string) (*model.ReferralWithdrawal, error) {
	var w model.ReferralWithdrawal
	if err := db.Where("withdraw_no = ?", no).First(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *referralRepository) UpdateWithdrawal(db *gorm.DB, w *model.ReferralWithdrawal) error {
	return db.Save(w).Error
}

// ListWithdrawalRows 分页查询提现单（关联出申请人用户名）。
func (r *referralRepository) ListWithdrawalRows(ctx context.Context, f WithdrawFilter) ([]WithdrawalRow, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	base := r.db.WithContext(ctx).Table("referral_withdrawals rw")
	if f.UserID > 0 {
		base = base.Where("rw.user_id = ?", f.UserID)
	}
	if f.Status != "" {
		base = base.Where("rw.status = ?", f.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []WithdrawalRow
	err := r.db.WithContext(ctx).Table("referral_withdrawals rw").
		Select("rw.*, COALESCE(u.username, '') AS username").
		Joins("LEFT JOIN users u ON u.id = rw.user_id").
		Where("(? = 0 OR rw.user_id = ?)", f.UserID, f.UserID).
		Where("(? = '' OR rw.status = ?)", f.Status, f.Status).
		Order("rw.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *referralRepository) SumPendingWithdrawals(ctx context.Context, userID uint64) (float64, error) {
	var sum float64
	err := r.db.WithContext(ctx).Model(&model.ReferralWithdrawal{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND status = ?", userID, model.WithdrawStatusPending).
		Scan(&sum).Error
	return sum, err
}

// —— 全局配置 ——

// Rates 读取 referral 分组配置；缺失项按默认值兜底，非法值忽略。
func (r *referralRepository) Rates(ctx context.Context) (model.Rates, error) {
	rates := model.DefaultRates()
	var rows []struct {
		Key   string `gorm:"column:config_key"`
		Value string `gorm:"column:config_value"`
	}
	err := r.db.WithContext(ctx).Table("system_configs").
		Select("config_key, config_value").
		Where("config_group = ? AND status = ?", model.ConfigGroup, "active").
		Scan(&rows).Error
	if err != nil {
		return rates, err
	}
	for _, row := range rows {
		switch row.Key {
		case model.ConfigKeyEnabled:
			rates.Enabled = row.Value == "true" || row.Value == "1"
		case model.ConfigKeyFirstOrder:
			rates.FirstOrder = parseRate(row.Value, rates.FirstOrder)
		case model.ConfigKeySubsequent:
			rates.Subsequent = parseRate(row.Value, rates.Subsequent)
		case model.ConfigKeyRenewal:
			rates.Renewal = parseRate(row.Value, rates.Renewal)
		case model.ConfigKeyMinWithdraw:
			if v, perr := strconv.ParseFloat(row.Value, 64); perr == nil && v >= 0 {
				rates.MinWithdraw = v
			}
		}
	}
	return rates, nil
}

// —— 内部工具 ——

func parseRate(value string, fallback float64) float64 {
	v, err := strconv.ParseFloat(value, 64)
	if err != nil || v < 0 || v > 1 {
		return fallback
	}
	return v
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
