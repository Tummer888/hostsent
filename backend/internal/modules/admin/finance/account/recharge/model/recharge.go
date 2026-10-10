// Package model 提供充值子域的数据模型。
//
// 目录位置（2026-10-10 菜单整理）：充值管理挂在「资金管理」菜单下，本子域因此从
// finance/recharge 迁到 finance/account/recharge；HTTP 路由、权限码与前端 URL 均不变
// （/api/v1/admin/finance/recharges、finance:recharge、页面 /finance/recharges）。
package model

import "time"

// 充值单状态
const (
	RechargeStatusPending string = "pending" // 待支付
	RechargeStatusSuccess string = "success" // 已到账
	RechargeStatusFailed  string = "failed"  // 失败
)

// 充值方式。manual 为线下/人工（管理端代充值、财务登记）。
const (
	RechargeMethodAlipay string = "alipay"
	RechargeMethodWechat string = "wechat"
	RechargeMethodManual string = "manual"
)

// Recharge 充值单。
type Recharge struct {
	ID         uint64  `gorm:"primaryKey;autoIncrement"`
	RechargeNo string  `gorm:"column:recharge_no;size:64;uniqueIndex;not null"`
	UserID     uint64  `gorm:"column:user_id;index;not null"`
	Amount     float64 `gorm:"type:decimal(15,2);not null"`
	Method     string  `gorm:"size:20;not null"` // alipay/wechat/manual
	Status     string  `gorm:"size:20;not null;default:pending;index"`
	ChannelTx  string  `gorm:"column:channel_tx;size:64"` // 渠道交易号
	// 支付通道关联（doc35）：充值改走支付中心后记录支付单与渠道实例编码，
	// 到账由支付单 paid 事件驱动（幂等键仍是 recharge_no，见 ApproveByNo）。
	PaymentOrderID uint64     `gorm:"column:payment_order_id;not null;default:0;index"`
	ChannelCode    string     `gorm:"column:channel_code;size:64;not null;default:''"`
	PaidAt         *time.Time `gorm:"column:paid_at"`
	Remark         string     `gorm:"size:255"`
	CreatedAt      time.Time  `gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Recharge) TableName() string {
	return "recharges"
}
