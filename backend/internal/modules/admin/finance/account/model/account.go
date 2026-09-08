// Package model 提供钱包账户子域的数据模型。
package model

import "time"

// WalletAccount 用户钱包账户。可用余额以 users.balance 为准，本表做统计与冻结控制。
type WalletAccount struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	UserID       uint64    `gorm:"column:user_id;uniqueIndex:uk_wallet_user;not null"`         // 与 users.id 1:1
	Balance      float64   `gorm:"type:decimal(15,2);not null;default:0"`                      // 可用余额（与 users.balance 同步）
	Frozen       float64   `gorm:"type:decimal(15,2);not null;default:0"`                      // 冻结金额（交易/提现中）
	TotalIncome  float64   `gorm:"column:total_income;type:decimal(15,2);not null;default:0"`  // 累计收入
	TotalExpense float64   `gorm:"column:total_expense;type:decimal(15,2);not null;default:0"` // 累计支出
	Version      uint64    `gorm:"not null;default:0"`                                         // 乐观锁版本号
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (WalletAccount) TableName() string {
	return "wallet_accounts"
}
