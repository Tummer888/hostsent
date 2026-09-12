// Package model 提供积分体系的数据模型。
//
// 边界（docs/实施计划/36 §2.1）：积分账本与资金账本完全独立。
//   - point_accounts / point_transactions 不参与任何 wallet 口径统计；
//   - 积分不可抵扣订单/账单，不可提现，收银台不得读取本模块；
//   - 流水只增不改不删，幂等键 (user_id, biz_type, ref_no) 与资金流水同构。
package model

import "time"

// 积分变动方向
const (
	DirectionEarn  int = 1  // 获得
	DirectionSpend int = -1 // 消耗
)

// 积分流水类型
const (
	TxTypeEarnPayment   string = "earn_payment"   // 订单支付成功发放
	TxTypeEarnRenewal   string = "earn_renewal"   // 续费支付成功发放
	TxTypeEarnActivity  string = "earn_activity"  // 运营活动发放（预埋）
	TxTypeSpendExchange string = "spend_exchange" // 积分兑换消耗（预埋）
	TxTypeAdjust        string = "adjust"         // 人工调整
	TxTypeExpire        string = "expire"         // 到期回收（预埋）
)

// 发放场景（与 point_rules.scene 对应）
const (
	SceneOrderPurchase string = "order_purchase"
	SceneOrderRenewal  string = "order_renewal"
	SceneBillPayment   string = "bill_payment"
	SceneActivity      string = "activity"
)

// 发放模式
const (
	EarnModeRate  string = "rate"  // 按金额比例
	EarnModeFixed string = "fixed" // 固定积分
)

// 规则状态
const (
	RuleStatusEnabled  int = 1
	RuleStatusDisabled int = 0
)

// PointAccount 用户积分账户（与 users 1:1，独立于 wallet_accounts）。
type PointAccount struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `gorm:"column:user_id;uniqueIndex:uk_point_accounts_user;not null"`
	Balance     int64     `gorm:"not null;default:0"` // 可用积分
	Frozen      int64     `gorm:"not null;default:0"` // 冻结积分（预埋）
	TotalEarned int64     `gorm:"column:total_earned;not null;default:0"`
	TotalSpent  int64     `gorm:"column:total_spent;not null;default:0"`
	Version     uint64    `gorm:"not null;default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (PointAccount) TableName() string { return "point_accounts" }

// PointTransaction 积分流水。只增不改不删，落库后不允许更新积分值。
// 以 (user_id, biz_type, ref_no) 唯一约束保证同一来源只发放一次。
type PointTransaction struct {
	ID            uint64 `gorm:"primaryKey;autoIncrement"`
	TxNo          string `gorm:"column:tx_no;size:64;uniqueIndex:uk_point_tx_no;not null"`
	UserID        uint64 `gorm:"column:user_id;uniqueIndex:uk_point_biz,priority:1;index;not null"`
	Type          string `gorm:"size:32;not null;index"`
	Direction     int    `gorm:"not null"`
	Points        int64  `gorm:"not null"` // 变动积分（正数）
	BalanceBefore int64  `gorm:"column:balance_before;not null;default:0"`
	BalanceAfter  int64  `gorm:"column:balance_after;not null;default:0"`
	BizType       string `gorm:"column:biz_type;uniqueIndex:uk_point_biz,priority:2;size:32;not null"`
	RefNo         string `gorm:"column:ref_no;uniqueIndex:uk_point_biz,priority:3;size:64;not null"`
	Remark        string `gorm:"size:255;not null;default:''"`
	OperatorID    uint64 `gorm:"column:operator_id;not null;default:0"`
	// ExpireAt 预埋：按规则 valid_days 计算的积分有效期，供过期回收任务使用（doc36 P-01）。
	ExpireAt  *time.Time `gorm:"column:expire_at"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (PointTransaction) TableName() string { return "point_transactions" }

// PointRule 积分发放规则：规则可配，避免在代码里写死比例。
type PointRule struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement"`
	Code              string    `gorm:"size:50;not null;uniqueIndex:uk_point_rules_code"`
	Name              string    `gorm:"size:100;not null"`
	Scene             string    `gorm:"size:32;not null"` // order_purchase/order_renewal/bill_payment/activity
	EarnMode          string    `gorm:"column:earn_mode;size:20;not null;default:rate"`
	FixedPoints       int64     `gorm:"column:fixed_points;not null;default:0"`                       // earn_mode=fixed 时生效
	PointsPerYuan     float64   `gorm:"column:points_per_yuan;type:numeric(12,4);not null;default:0"` // earn_mode=rate 时生效
	MinAmount         float64   `gorm:"column:min_amount;type:decimal(15,2);not null;default:0"`      // 触发门槛（0=不限）
	MaxPointsPerOrder int64     `gorm:"column:max_points_per_order;not null;default:0"`               // 单笔上限（0=不限）
	ValidDays         int       `gorm:"column:valid_days;not null;default:0"`                         // 有效期天数（0=永久）
	Status            int       `gorm:"not null;default:1"`
	SortOrder         int       `gorm:"column:sort_order;not null;default:0"`
	Remark            string    `gorm:"size:255;not null;default:''"`
	CreatedAt         time.Time `gorm:"autoCreateTime"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (PointRule) TableName() string { return "point_rules" }

// PointAccountRow 积分账户列表行：附带用户名，便于管理端检索。
type PointAccountRow struct {
	PointAccount
	Username string `gorm:"column:username"`
	Email    string `gorm:"column:email"`
}

// PointTransactionRow 积分流水列表行：附带用户名。
type PointTransactionRow struct {
	PointTransaction
	Username string `gorm:"column:username"`
}
