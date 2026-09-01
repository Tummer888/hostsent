package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/finance/model"
	usermodel "hostsent/backend/internal/modules/admin/user/account/model"
)

// WalletRepository 钱包账户数据访问。
// 传入的 db 可为普通连接或事务连接，以支持账务核心在事务内完成加锁与变更。
type WalletRepository interface {
	FindByUser(db *gorm.DB, userID uint64) (*model.WalletAccount, error)
	LockByUser(db *gorm.DB, userID uint64) (*model.WalletAccount, error)
	Create(db *gorm.DB, acc *model.WalletAccount) error
	Update(db *gorm.DB, acc *model.WalletAccount) error
	SyncUsersBalance(db *gorm.DB, userID uint64, balance float64) error
	SumBalance(ctx context.Context) (float64, error)
}

type walletRepository struct {
	db *gorm.DB
}

// NewWalletRepository 创建钱包账户仓储。
func NewWalletRepository(db *gorm.DB) WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) FindByUser(db *gorm.DB, userID uint64) (*model.WalletAccount, error) {
	var acc model.WalletAccount
	if err := db.Where("user_id = ?", userID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

// LockByUser 以行锁（SELECT ... FOR UPDATE）读取账户，防止并发超扣。
func (r *walletRepository) LockByUser(db *gorm.DB, userID uint64) (*model.WalletAccount, error) {
	var acc model.WalletAccount
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

func (r *walletRepository) Create(db *gorm.DB, acc *model.WalletAccount) error {
	return db.Create(acc).Error
}

func (r *walletRepository) Update(db *gorm.DB, acc *model.WalletAccount) error {
	return db.Save(acc).Error
}

// SyncUsersBalance 在同一事务内同步 users.balance，避免两处余额漂移。
func (r *walletRepository) SyncUsersBalance(db *gorm.DB, userID uint64, balance float64) error {
	return db.Model(&usermodel.User{}).Where("id = ?", userID).Update("balance", balance).Error
}

// SumBalance 汇总全部钱包账户可用余额（用于对账）。
func (r *walletRepository) SumBalance(ctx context.Context) (float64, error) {
	var sum float64
	err := r.db.WithContext(ctx).Model(&model.WalletAccount{}).Select("COALESCE(SUM(balance), 0)").Scan(&sum).Error
	return sum, err
}
