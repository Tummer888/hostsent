package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/model"
)

// PreferenceRepository 用户支付方式偏好与收款账户数据访问。
type PreferenceRepository interface {
	ListPreferences(ctx context.Context, userID uint64) ([]model.UserPaymentPreference, error)
	ReplacePreferences(ctx context.Context, userID uint64, items []model.UserPaymentPreference) error

	CreateAccount(ctx context.Context, a *model.UserPayoutAccount) error
	UpdateAccount(ctx context.Context, a *model.UserPayoutAccount) error
	FindAccount(ctx context.Context, id uint64) (*model.UserPayoutAccount, error)
	ListAccounts(ctx context.Context, userID uint64) ([]model.UserPayoutAccount, error)
	ClearDefaultAccount(ctx context.Context, userID, exceptID uint64) error
}

type preferenceRepository struct {
	db *gorm.DB
}

// NewPreferenceRepository 创建偏好/收款账户仓储。
func NewPreferenceRepository(db *gorm.DB) PreferenceRepository {
	return &preferenceRepository{db: db}
}

func (r *preferenceRepository) ListPreferences(ctx context.Context, userID uint64) ([]model.UserPaymentPreference, error) {
	var items []model.UserPaymentPreference
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("scene asc, priority desc, id asc").Find(&items).Error
	return items, err
}

// ReplacePreferences 全量替换用户偏好（先删后插，单事务）。
func (r *preferenceRepository) ReplacePreferences(ctx context.Context, userID uint64, items []model.UserPaymentPreference) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserPaymentPreference{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

func (r *preferenceRepository) CreateAccount(ctx context.Context, a *model.UserPayoutAccount) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *preferenceRepository) UpdateAccount(ctx context.Context, a *model.UserPayoutAccount) error {
	return r.db.WithContext(ctx).Save(a).Error
}

func (r *preferenceRepository) FindAccount(ctx context.Context, id uint64) (*model.UserPayoutAccount, error) {
	var a model.UserPayoutAccount
	if err := r.db.WithContext(ctx).First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *preferenceRepository) ListAccounts(ctx context.Context, userID uint64) ([]model.UserPayoutAccount, error) {
	var items []model.UserPayoutAccount
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("is_default desc, id asc").Find(&items).Error
	return items, err
}

func (r *preferenceRepository) ClearDefaultAccount(ctx context.Context, userID, exceptID uint64) error {
	return r.db.WithContext(ctx).Model(&model.UserPayoutAccount{}).
		Where("user_id = ? AND id <> ?", userID, exceptID).
		Update("is_default", false).Error
}
