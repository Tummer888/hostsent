// Package model 提供推广邀请返现子域的数据模型。
//
// 边界（见 docs/实施计划/84-推广邀请返现体系实施计划.md）：
//   - referral_accounts 是返现余额的唯一权威，与现金钱包 wallet_accounts 完全独立；
//   - 返现只能「提现」或「转入现金余额」，两条出口都留独立台账；
//   - 邀请关系为单级，落在 users.inviter_user_id 上，不建关系表。
package model

import "time"

// 返现全局配置：落在 system_configs 的 referral 分组，运行时读取（首个运行时配置消费者）。
const (
	ConfigGroup          string = "referral"
	ConfigKeyEnabled     string = "referral.enabled"
	ConfigKeyFirstOrder  string = "referral.first_order_rate"
	ConfigKeySubsequent  string = "referral.subsequent_rate"
	ConfigKeyRenewal     string = "referral.renewal_rate"
	ConfigKeyMinWithdraw string = "referral.min_withdraw_amount"
)

// Rates 返现比率配置。比率为小数（0.10 表示 10%）。
type Rates struct {
	Enabled     bool    `json:"enabled"`
	FirstOrder  float64 `json:"first_order_rate"` // 被邀请人首单
	Subsequent  float64 `json:"subsequent_rate"`  // 被邀请人后续下单
	Renewal     float64 `json:"renewal_rate"`     // 被邀请人续费
	MinWithdraw float64 `json:"min_withdraw_amount"`
}

// DefaultRates 配置缺失时的兜底值（与 db.Seed 写入的默认值保持一致）。
func DefaultRates() Rates {
	return Rates{Enabled: true, FirstOrder: 0.10, Subsequent: 0.05, Renewal: 0.03, MinWithdraw: 50}
}

// 返现台账类型。台账只增不改，落库后不允许更新金额。
const (
	TxTypeCashback        string = "cashback"         // 首单/后续返现入账（+）
	TxTypeRenewalCashback string = "renewal_cashback" // 续费返现入账（+）
	TxTypeRefundClawback  string = "refund_clawback"  // 退款冲减（-）
	TxTypeWithdrawFreeze  string = "withdraw_freeze"  // 提现申请冻结（可用余额 -）
	TxTypeWithdrawReturn  string = "withdraw_return"  // 提现驳回解冻（可用余额 +）
	TxTypeTransferOut     string = "transfer_out"     // 转入现金余额（可用余额 -）
	TxTypeTransferReturn  string = "transfer_return"  // 转入失败补偿回滚（可用余额 +）
)

// 台账方向。
const (
	DirectionIncome  int = 1  // 收入
	DirectionExpense int = -1 // 支出
)

// 提现单状态。
const (
	WithdrawStatusPending  string = "pending"  // 待审核
	WithdrawStatusApproved string = "approved" // 审核通过（已出账）
	WithdrawStatusRejected string = "rejected" // 已驳回
)

// ReferralAccount 返现账户：与 users 1:1，独立于 wallet_accounts。
// balance 为可用返现余额；提现申请冻结后转入 frozen，审核通过再从 frozen 扣除。
type ReferralAccount struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `gorm:"column:user_id;not null;uniqueIndex:uk_referral_user"`      // 与 users.id 1:1
	Balance     float64   `gorm:"type:decimal(15,2);not null;default:0"`                     // 可用返现余额（允许为负：退款冲减可致欠款）
	Frozen      float64   `gorm:"type:decimal(15,2);not null;default:0"`                     // 提现申请冻结金额
	TotalIncome float64   `gorm:"column:total_income;type:decimal(15,2);not null;default:0"` // 累计返现收入
	TotalOut    float64   `gorm:"column:total_out;type:decimal(15,2);not null;default:0"`    // 累计成功出账（提现通过 + 转入余额 + 退款冲减）
	Version     uint64    `gorm:"not null;default:0"`                                        // 乐观锁版本号
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (ReferralAccount) TableName() string { return "referral_accounts" }

// ReferralTransaction 返现台账。以 (user_id, biz_type, ref_no) 唯一约束保证同一来源只记一次账。
// balance_before/balance_after 跟踪的是「可用返现余额」；提现审核通过只扣 frozen，
// 不产生可用余额变动，因此不落台账行，审核结果记在 referral_withdrawals 上。
type ReferralTransaction struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	TxNo          string    `gorm:"column:tx_no;size:64;not null;uniqueIndex:uk_referral_tx_no"`                               // 台账流水号
	UserID        uint64    `gorm:"column:user_id;not null;uniqueIndex:uk_referral_biz,priority:1;index:idx_referral_tx_user"` // 返现归属人（邀请人）
	Type          string    `gorm:"size:32;not null;index:idx_referral_tx_type"`                                               // cashback/renewal_cashback/refund_clawback/withdraw_freeze/withdraw_return/transfer_out
	Direction     int       `gorm:"not null"`                                                                                  // 1=收入 -1=支出
	Amount        float64   `gorm:"type:decimal(15,2);not null"`                                                               // 变动金额
	BalanceBefore float64   `gorm:"column:balance_before;type:decimal(15,2);not null"`                                         // 变动前可用余额
	BalanceAfter  float64   `gorm:"column:balance_after;type:decimal(15,2);not null"`                                          // 变动后可用余额
	BizType       string    `gorm:"column:biz_type;size:32;not null;uniqueIndex:uk_referral_biz,priority:2"`                   // 幂等业务标识
	RefNo         string    `gorm:"column:ref_no;size:64;uniqueIndex:uk_referral_biz,priority:3"`                              // 关联单号（订单号/退款号/提现号/转账号）
	OrderID       uint64    `gorm:"column:order_id;index:idx_referral_tx_order_id"`                                            // 关联订单
	OrderNo       string    `gorm:"column:order_no;size:64"`                                                                   // 订单号快照
	InviterUserID uint64    `gorm:"column:inviter_user_id"`                                                                    // 邀请人
	InviteeUserID uint64    `gorm:"column:invitee_user_id;index:idx_referral_tx_invitee"`                                      // 被邀请人（下单人）
	Remark        string    `gorm:"size:255"`
	OperatorID    uint64    `gorm:"column:operator_id"`                           // 操作人（0=系统）
	CreatedAt     time.Time `gorm:"autoCreateTime;index:idx_referral_tx_created"` // 创建时间
}

// TableName 指定表名。
func (ReferralTransaction) TableName() string { return "referral_transactions" }

// ReferralWithdrawal 返现提现单：独立于现金钱包提现（withdrawals），走独立审核。
type ReferralWithdrawal struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	WithdrawNo  string     `gorm:"column:withdraw_no;size:64;not null;uniqueIndex:uk_referral_wd_no"`
	UserID      uint64     `gorm:"column:user_id;not null;index:idx_referral_wd_user"`
	Amount      float64    `gorm:"type:decimal(15,2);not null"`
	Channel     string     `gorm:"size:20"`  // bank/alipay
	Account     string     `gorm:"size:128"` // 收款账户
	Status      string     `gorm:"size:20;not null;default:pending;index:idx_referral_wd_status"`
	AuditBy     uint64     `gorm:"column:audit_by"`
	AuditByName string     `gorm:"column:audit_by_name;size:50"`
	AuditedAt   *time.Time `gorm:"column:audited_at"`
	Remark      string     `gorm:"size:255"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (ReferralWithdrawal) TableName() string { return "referral_withdrawals" }
