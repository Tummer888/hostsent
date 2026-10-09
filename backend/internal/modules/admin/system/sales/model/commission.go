package model

import "time"

// 提成全局配置：落在 system_configs 的 sales 分组，运行时读取（与 referral 同模式）。
// 键名沿用 S1 已写入 db.go 的 sales.commission.* / sales.relation.* 命名，避免改动线上配置键。
const (
	ConfigGroup                string = "sales"
	ConfigKeyEnabled           string = "sales.enabled"
	ConfigKeyFirstOrder        string = "sales.commission.first_order"
	ConfigKeySubsequent        string = "sales.commission.subsequent"
	ConfigKeyRenewal           string = "sales.commission.renewal"
	ConfigKeyReleaseDays       string = "sales.commission.release_days"
	ConfigKeyMinWithdraw       string = "sales.commission.min_withdraw"
	ConfigKeyProtectDays       string = "sales.relation.protect_days"
	ConfigKeyRenewalCommission string = "sales.renewal_commission_mode"
	ConfigKeyAllowNegative     string = "sales.allow_negative"
)

// 续费提成归属模式。
const (
	// RenewalModeFollowOrder 跟单：续费提成随「源订单」的销售快照（默认，鼓励持续服务）。
	RenewalModeFollowOrder string = "follow_order"
	// RenewalModeFollowOwner 跟现役归属：续费提成归客户当前归属销售。
	RenewalModeFollowOwner string = "follow_owner"
)

// Rates 提成配置。比率为小数（0.08 表示 8%）。
type Rates struct {
	Enabled       bool    `json:"enabled"`
	FirstOrder    float64 `json:"first_order_rate"` // 客户首单
	Subsequent    float64 `json:"subsequent_rate"`  // 客户后续下单
	Renewal       float64 `json:"renewal_rate"`     // 续费
	ReleaseDays   int     `json:"release_days"`     // 解冻期（天）
	MinWithdraw   float64 `json:"min_withdraw"`     // 最低提现额
	ProtectDays   int     `json:"protect_days"`     // 归属保护期（天）
	RenewalMode   string  `json:"renewal_mode"`     // follow_order/follow_owner
	AllowNegative bool    `json:"allow_negative"`   // 已打款后退款允许负余额
}

// DefaultRates 配置缺失时的兜底值（与 db.go seedSystemConfigs 写入的默认值一致）。
func DefaultRates() Rates {
	return Rates{
		Enabled:       true,
		FirstOrder:    0.08,
		Subsequent:    0.05,
		Renewal:       0.03,
		ReleaseDays:   30,
		MinWithdraw:   100,
		ProtectDays:   90,
		RenewalMode:   RenewalModeFollowOrder,
		AllowNegative: true,
	}
}

// 提成台账类型。台账只增不改，落库后不允许更新金额。
const (
	TxTypeAccrueFirst      string = "accrue_first"      // 客户首单计提（待解冻）
	TxTypeAccrueSubsequent string = "accrue_subsequent" // 客户后续单计提（待解冻）
	TxTypeAccrueRenewal    string = "accrue_renewal"    // 续费计提（待解冻）
	TxTypeRelease          string = "release"           // 解冻转可用（pending_release → balance）
	TxTypeRefundClawback   string = "refund_clawback"   // 退款冲减（先扣解冻中，不足再扣可用，可为负）
	TxTypeWithdrawFreeze   string = "withdraw_freeze"   // 提现申请冻结（balance → frozen）
	TxTypeWithdrawReturn   string = "withdraw_return"   // 提现驳回/打款失败解冻（frozen → balance）
	TxTypeWithdrawPaid     string = "withdraw_paid"     // 打款成功结算（frozen 出账）
)

// IsAccrualType 判断是否为计提类流水（解冻期的清理对象）。
func IsAccrualType(t string) bool {
	switch t {
	case TxTypeAccrueFirst, TxTypeAccrueSubsequent, TxTypeAccrueRenewal:
		return true
	}
	return false
}

// TxTypeLabels 台账类型中文名（前端展示与备注共用）。
func TxTypeLabels() map[string]string {
	return map[string]string{
		TxTypeAccrueFirst:      "首单提成",
		TxTypeAccrueSubsequent: "后续单提成",
		TxTypeAccrueRenewal:    "续费提成",
		TxTypeRelease:          "解冻转可用",
		TxTypeRefundClawback:   "退款冲减",
		TxTypeWithdrawFreeze:   "提现冻结",
		TxTypeWithdrawReturn:   "提现退回",
		TxTypeWithdrawPaid:     "提现已打款",
	}
}

// 台账方向。
const (
	DirectionIncome  int = 1  // 入账
	DirectionExpense int = -1 // 出账
)

// 提现单状态。approved 后进入打款流程（paying），settler 回调落 paid。
const (
	WithdrawStatusPending  string = "pending"  // 待审核
	WithdrawStatusApproved string = "approved" // 审核通过（待打款）
	WithdrawStatusPaying   string = "paying"   // 已登记打款，等结算回调
	WithdrawStatusPaid     string = "paid"     // 已打款（frozen 出账完成）
	WithdrawStatusRejected string = "rejected" // 已驳回（解冻回可用）
	WithdrawStatusFailed   string = "failed"   // 打款失败（解冻回可用）
)

// 业绩目标范围。
const (
	TargetScopeAdmin      string = "admin"      // 个人目标
	TargetScopeDepartment string = "department" // 部门目标
)

// CommissionAccount 提成账户：与 admins 1:1，独立于返现账户与现金钱包。
//   - balance 可用（已过解冻期）；
//   - pending_release 解冻中（计提后未到 release_at）；
//   - frozen 提现冻结（申请后、打款结算前）。
type CommissionAccount struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	AdminID        uint64    `gorm:"column:admin_id;not null;uniqueIndex:uk_sca_admin"`
	Balance        float64   `gorm:"type:decimal(15,2);not null;default:0"` // 可用提成（允许为负：已打款后退款形成欠款）
	Frozen         float64   `gorm:"type:decimal(15,2);not null;default:0"`
	PendingRelease float64   `gorm:"column:pending_release;type:decimal(15,2);not null;default:0"`
	TotalIncome    float64   `gorm:"column:total_income;type:decimal(15,2);not null;default:0"`
	TotalOut       float64   `gorm:"column:total_out;type:decimal(15,2);not null;default:0"`
	Version        uint64    `gorm:"not null;default:0"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (CommissionAccount) TableName() string { return "sales_commission_accounts" }

// CommissionTransaction 提成台账。以 (admin_id, biz_type, ref_no) 唯一约束保证同一来源只记一次账。
// release_at 非空表示「该计提仍在解冻中」；解冻后置 NULL 作为已解冻标记（幂等）。
type CommissionTransaction struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	TxNo          string     `gorm:"column:tx_no;size:64;not null;uniqueIndex:uk_sct_no"`
	AdminID       uint64     `gorm:"column:admin_id;not null;index:idx_sct_admin_created"`
	Type          string     `gorm:"size:32;not null"`
	Direction     int        `gorm:"not null"`
	Amount        float64    `gorm:"type:decimal(15,2);not null"`
	BalanceBefore float64    `gorm:"column:balance_before;type:decimal(15,2);not null;default:0"`
	BalanceAfter  float64    `gorm:"column:balance_after;type:decimal(15,2);not null;default:0"`
	BizType       string     `gorm:"column:biz_type;size:32;not null"`
	RefNo         string     `gorm:"column:ref_no;size:64;not null"`
	OrderID       uint64     `gorm:"column:order_id;not null;default:0;index:idx_sct_order"`
	OrderNo       string     `gorm:"column:order_no;size:64;not null;default:''"`
	CustomerID    uint64     `gorm:"column:customer_user_id;not null;default:0"` // 产生提成的客户（users.id）
	ReleaseAt     *time.Time `gorm:"column:release_at;index:idx_sct_release"`
	Remark        string     `gorm:"size:255;not null;default:''"`
	OperatorID    uint64     `gorm:"column:operator_id;not null;default:0"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (CommissionTransaction) TableName() string { return "sales_commission_transactions" }

// SalesWithdrawal 提成提现单：主体是 admins，出口复用 payment_payouts（biz_type=sales_withdraw）。
type SalesWithdrawal struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	WithdrawNo  string     `gorm:"column:withdraw_no;size:64;not null;uniqueIndex:uk_sw_no"`
	AdminID     uint64     `gorm:"column:admin_id;not null;index:idx_sw_admin"`
	Amount      float64    `gorm:"type:decimal(15,2);not null"`
	Channel     string     `gorm:"size:20;not null;default:''"`
	Account     string     `gorm:"size:128;not null;default:''"`
	AccountName string     `gorm:"column:account_name;size:64;not null;default:''"`
	BankName    string     `gorm:"column:bank_name;size:100;not null;default:''"`
	PayoutMode  string     `gorm:"column:payout_mode;size:20;not null;default:manual"`
	PayoutNo    string     `gorm:"column:payout_no;size:64;not null;default:''"`
	PayoutID    uint64     `gorm:"column:payout_id;not null;default:0"`
	Status      string     `gorm:"size:20;not null;default:pending;index:idx_sw_status"`
	AuditBy     uint64     `gorm:"column:audit_by;not null;default:0"`
	AuditByName string     `gorm:"column:audit_by_name;size:64;not null;default:''"`
	AuditedAt   *time.Time `gorm:"column:audited_at"`
	PaidAt      *time.Time `gorm:"column:paid_at"`
	Remark      string     `gorm:"size:255;not null;default:''"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (SalesWithdrawal) TableName() string { return "sales_withdrawals" }

// SalesTarget 业绩目标：按「周期 + 范围」唯一（个人目标 admin_id>0，部门目标 department_id>0）。
type SalesTarget struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	Period       string    `gorm:"size:7;not null"` // YYYY-MM
	Scope        string    `gorm:"size:20;not null;default:admin"`
	AdminID      uint64    `gorm:"column:admin_id;not null;default:0"`
	DepartmentID uint64    `gorm:"column:department_id;not null;default:0"`
	TargetAmount float64   `gorm:"column:target_amount;type:decimal(15,2);not null;default:0"`
	TargetOrders int       `gorm:"column:target_orders;not null;default:0"`
	CreatedBy    uint64    `gorm:"column:created_by;not null;default:0"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (SalesTarget) TableName() string { return "sales_targets" }
