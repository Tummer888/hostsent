// Package model 提供提现子域的数据模型。
//
// 目录位置（2026-10-10 菜单整理）：提现管理挂在「推广返现」菜单下，本子域因此从
// finance/withdraw 迁到 finance/referral/walletwithdraw；HTTP 路由、权限码与前端 URL
// 均不变（/api/v1/admin/finance/withdrawals、finance:withdraw、页面 /finance/withdrawals）。
// 注意：这里管的是「用户钱包余额提现」（withdrawals 表）；返现提现审核在 referral 域
// 自己的 referral_withdrawals（referral/service/withdrawal_service.go），两套单子不要混。
package model

import "time"

// 提现单状态
const (
	WithdrawStatusPending  string = "pending"  // 待审核
	WithdrawStatusApproved string = "approved" // 审核通过（资金已冻结，待打款）
	WithdrawStatusPaying   string = "paying"   // 打款中
	WithdrawStatusPaid     string = "paid"     // 已打款（冻结已结算）
	WithdrawStatusRejected string = "rejected" // 已驳回（已解冻）
	WithdrawStatusFailed   string = "failed"   // 打款失败（已解冻退回）
)

// 打款模式
const (
	PayoutModeManual string = "manual" // 人工打款登记
	PayoutModeAPI    string = "api"    // 渠道接口自动打款
)

// Withdraw 提现单。
type Withdraw struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	WithdrawNo  string     `gorm:"column:withdraw_no;size:64;uniqueIndex;not null"`
	UserID      uint64     `gorm:"column:user_id;index;not null"`
	Amount      float64    `gorm:"type:decimal(15,2);not null"`
	Channel     string     `gorm:"size:20"`  // bank/alipay
	Account     string     `gorm:"size:128"` // 收款账户（脱敏展示）
	AccountName string     `gorm:"column:account_name;size:64"`
	BankName    string     `gorm:"column:bank_name;size:100"`
	Status      string     `gorm:"size:20;not null;default:pending;index"`
	AuditBy     uint64     `gorm:"column:audit_by"`
	AuditByName string     `gorm:"column:audit_by_name;size:50"`
	AuditedAt   *time.Time `gorm:"column:audited_at"`
	// 打款闭环字段（F-02）：审核通过后创建打款任务，打款成功才结算冻结资金。
	PayoutNo   string     `gorm:"column:payout_no;size:64"`
	PayoutMode string     `gorm:"column:payout_mode;size:20;not null;default:''"`
	PayoutID   uint64     `gorm:"column:payout_id;not null;default:0"`
	ChannelTx  string     `gorm:"column:channel_tx;size:128"`
	PaidAt     *time.Time `gorm:"column:paid_at"`
	Remark     string     `gorm:"size:255"`
	CreatedAt  time.Time  `gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Withdraw) TableName() string {
	return "withdrawals"
}
