// Package model 提供资金流水子域的数据模型。
package model

import "time"

// 资金流水类型
const (
	TxTypeRecharge   string = "recharge"   // 充值
	TxTypeConsume    string = "consume"    // 消费（订单支付扣余额）
	TxTypeRefund     string = "refund"     // 退款（订单退款回余额）
	TxTypeCommission string = "commission" // 佣金（分销入账）
	TxTypeSettlement string = "settlement" // 结算（分销/提现出账）
	TxTypeAdjust     string = "adjust"     // 调账（赠送/扣减）
	// TxTypeAdminOrder 后台代客下单扣款（管理端余额支付路径）。
	// 历史实现直接写字面量 "order"，库中已存在该类型，但模型无常量、前端也无标签。
	TxTypeAdminOrder string = "order" // 后台代下单扣款
	// TxTypeReferralTransfer 推广返现转入现金余额（收入）。
	TxTypeReferralTransfer string = "referral_transfer"
	// TxTypeFreeze 冻结（可用余额→冻结余额；提现申请/预授权）。
	TxTypeFreeze string = "freeze"
	// TxTypeUnfreeze 解冻（冻结余额→可用余额；提现驳回/打款失败退余额）。
	TxTypeUnfreeze string = "unfreeze"
)

// 资金方向
const (
	DirectionIncome  int = 1  // 收入
	DirectionExpense int = -1 // 支出
)

// ConsumeTypes 计入「消费」的流水类型。
//
// 之所以要有这张表：消费额、账单、收入统计三处各自写过一遍类型白名单，写法还
// 不一样 —— 有的只认 consume，漏了后台代客下单的 order，于是同一笔代客下单在
// 资金流水里有、在累计消费里没有（实测用户 2730：买了 18 元代客单，总消费额
// 停在 1 元）。把「什么算消费」收敛到一处，新增扣款类型时只改这里。
//
// 判定同时要求 direction=expense：refund 等类型可能是收入方向，不该被算进来。
// settle（冻结结算）不在内：它是提现打款出账，不是买货消费。
var ConsumeTypes = []string{TxTypeConsume, TxTypeAdminOrder}

// InternalTransferTypes 内部划转：可用余额 ↔ 冻结余额，钱没离开账户，不算真实收付。
// 对账与统计口径都要剔除，否则提现申请会把「支出」凭空抬高一次。
var InternalTransferTypes = []string{TxTypeFreeze, TxTypeUnfreeze}

// IsConsumeType 判断某类型是否计入消费口径。
func IsConsumeType(txType string) bool {
	for _, t := range ConsumeTypes {
		if t == txType {
			return true
		}
	}
	return false
}

// WalletTransaction 资金流水台账。只增不改不删，落库后不允许更新金额。
// 以 (user_id, biz_type, ref_no) 唯一约束保证同一来源只记一次账（幂等）。
type WalletTransaction struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	TxNo          string    `gorm:"column:tx_no;size:64;uniqueIndex;not null"` // 流水号，如 20260830W00001
	UserID        uint64    `gorm:"column:user_id;uniqueIndex:uk_biz,priority:1;index;not null"`
	Type          string    `gorm:"size:32;not null;index"`                                         // recharge/consume/refund/commission/settlement/adjust
	Direction     int       `gorm:"not null"`                                                       // 1=收入 -1=支出
	Amount        float64   `gorm:"type:decimal(15,2);not null"`                                    // 变动金额
	BalanceBefore float64   `gorm:"column:balance_before;type:decimal(15,2);not null"`              // 变动前余额
	BalanceAfter  float64   `gorm:"column:balance_after;type:decimal(15,2);not null"`               // 变动后余额
	OrderID       uint64    `gorm:"column:order_id;index"`                                          // 关联订单（可为空）
	OrderNo       string    `gorm:"column:order_no;size:64"`                                        // 订单号快照
	RefNo         string    `gorm:"column:ref_no;uniqueIndex:uk_biz,priority:3;size:64"`            // 关联单号（退款/结算/充值单号）
	BizType       string    `gorm:"column:biz_type;uniqueIndex:uk_biz,priority:2;size:32;not null"` // 业务标识，用于幂等
	Remark        string    `gorm:"size:255"`
	OperatorID    uint64    `gorm:"column:operator_id"` // 操作人（0=系统）
	CreatedAt     time.Time `gorm:"autoCreateTime;index"`
}

// TableName 指定表名
func (WalletTransaction) TableName() string {
	return "wallet_transactions"
}
