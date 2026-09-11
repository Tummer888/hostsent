// Package model 提供订单域的数据模型。
package model

import "time"

// 订单状态
const (
	OrderStatusPending      string = "pending"      // 待支付
	OrderStatusPaid         string = "paid"         // 已支付
	OrderStatusProvisioning string = "provisioning" // 开通中
	OrderStatusActive       string = "active"       // 服务中
	OrderStatusRefunding    string = "refunding"    // 退款中
	OrderStatusRefunded     string = "refunded"     // 已退款
	OrderStatusCancelled    string = "cancelled"    // 已取消
	OrderStatusClosed       string = "closed"       // 已关闭（过期/作废）
	OrderStatusCompleted    string = "completed"    // 已完成
)

// 支付方式
const (
	PayMethodBalance string = "balance" // 余额支付
	PayMethodAlipay  string = "alipay"  // 支付宝
	PayMethodWeChat  string = "wechat"  // 微信支付
	PayMethodManual  string = "manual"  // 线下/人工
)

// Order 订单
type Order struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	OrderNo     string `gorm:"column:order_no;size:64;uniqueIndex;not null"` // 订单号
	UserID      uint64 `gorm:"column:user_id;index;not null"`                // 下单用户
	ProductID   uint64 `gorm:"column:product_id;index"`                      // 主产品ID(简化快照，明细见 order_items)
	ProductName string `gorm:"column:product_name;size:128"`                 // 产品名称快照
	Specs       string `gorm:"type:text"`                                    // 规格快照 JSON
	// SpecCode 所选 SKU 编码（T4.2）：履约时据此回查 spec_bindings 的平台参数；
	// 空表示按商品级配置（存量订单语义）。
	SpecCode    string     `gorm:"column:spec_code;size:64;index"`
	Quantity    int        `gorm:"default:1"`                                                 // 数量
	PriceModel  string     `gorm:"column:price_model;size:20;default:'fixed'"`                // 价格模型
	TotalAmount float64    `gorm:"column:total_amount;type:decimal(15,2);not null;default:0"` // 应付总额
	PaidAmount  float64    `gorm:"column:paid_amount;type:decimal(15,2);not null;default:0"`  // 实付金额
	Status      string     `gorm:"size:32;not null;default:pending;index"`                    // 订单状态
	PayMethod   string     `gorm:"column:pay_method;size:20"`                                 // 支付方式
	PayTime     *time.Time `gorm:"column:pay_time"`                                           // 支付时间
	ExpireTime  *time.Time `gorm:"column:expire_time"`                                        // 过期/计费到期时间
	Remark      string     `gorm:"size:255"`                                                  // 备注
	OperatorID  uint64     `gorm:"column:operator_id"`                                        // 最近操作人
	RenewalID   uint64     `gorm:"column:renewal_id;index;default:0"`                         // 关联续费记录 ID（续费订单，doc60）
	// 算价快照（P5-01/P5-04）：原价、优惠、实付与命中的折扣来源，便于对账与展示。
	OriginalAmount float64    `gorm:"column:original_amount;type:decimal(15,2);not null;default:0"` // 优惠前金额
	DiscountAmount float64    `gorm:"column:discount_amount;type:decimal(15,2);not null;default:0"` // 优惠金额
	FinalAmount    float64    `gorm:"column:final_amount;type:decimal(15,2);not null;default:0"`    // 实付金额
	PricePolicyID  *uint64    `gorm:"column:price_policy_id"`                                       // 命中的折扣策略 ID
	DiscountSource string     `gorm:"column:discount_source;size:32"`                               // agent / group / promotion / manual
	PriceSnapshot  string     `gorm:"column:price_snapshot;type:jsonb"`                             // 命中规则明细 JSON
	CreatedAt      time.Time  `gorm:"autoCreateTime;index"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`
	DeletedAt      *time.Time `gorm:"index"`
}

// TableName 指定表名
func (Order) TableName() string {
	return "orders"
}

// OrderStatusCount 订单状态计数（统计聚合结果）
type OrderStatusCount struct {
	Status string `gorm:"column:status"`
	Count  int64  `gorm:"column:count"`
}

// OrderTrendPoint 订单趋势点（按日聚合：销售额 + 订单量）
type OrderTrendPoint struct {
	Date   string  `gorm:"column:date"`
	Sales  float64 `gorm:"column:sales"`
	Orders int64   `gorm:"column:orders"`
}
