// Package model 定义「限时活动折扣」的数据模型（doc108 §8J）。
//
// 与代理折扣的边界（这是本模块存在的理由）：
//   - 代理折扣（agent_levels + agent_level_discounts）面向**代理**，回答「这代理拿货几折」，
//     是长期有效的商务关系，写在逐格矩阵里；
//   - 活动折扣（本模块）面向**普通用户**，回答「这期限时几折」，是到点开始、到点结束的营销活动。
//
// 两者主体不同，因此**不共用表、不互相叠加**：代理用户只走代理折扣，不参与活动
// （见 service 层的命中逻辑），避免出现「大促时代理反而比普通用户贵」这种说不清的价格。
package model

import "time"

// 折扣类型。
const (
	DiscountTypeRate   string = "rate"   // 折扣率：0.85 = 八五折
	DiscountTypeAmount string = "amount" // 直减：减固定金额
)

// 作用范围（无定向条目时按此范围生效）。
const (
	ScopeAll      string = "all"      // 全站商品
	ScopeCategory string = "category" // 指定分类
	ScopeProduct  string = "product"  // 指定商品
)

// 定向条目目标类型（doc108 §8J：分类与单个商品两种粒度，可多选）。
const (
	TargetCategory string = "category"
	TargetProduct  string = "product"
)

// 活动状态。
const (
	StatusActive   string = "active"   // 启用：在生效窗口内即参与算价
	StatusDisabled string = "disabled" // 停用：立即退出算价，不必等窗口结束
)

// FlashDiscount 限时活动折扣。
//
// 生效窗口用两个可空时间表示：start_at 为空 = 立即开始，end_at 为空 = 不限结束。
// 窗口是**算价时实时判断**的，不写回状态字段 —— 这样运营改系统时间/补录活动
// 都不会留下「状态说进行中、实际已过期」的脏数据。
type FlashDiscount struct {
	ID   uint64 `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"size:64;not null"`
	// Code 唯一编码，用于活动之间区分与审计（订单快照里记录它）。
	Code        string `gorm:"size:64;not null;uniqueIndex:uk_flash_discounts_code"`
	Description string `gorm:"size:255"`
	// DiscountType rate / amount；DiscountValue 折扣率（0.85）或直减金额。
	DiscountType  string  `gorm:"column:discount_type;size:16;not null;default:rate"`
	DiscountValue float64 `gorm:"column:discount_value;type:decimal(10,4);not null;default:0"`
	// Scope 无定向条目时的作用范围：all / category / product。
	Scope string `gorm:"size:16;not null;default:all"`
	// StartAt / EndAt 生效窗口；nil 分别表示「立即开始」「不限结束」。
	StartAt *time.Time `gorm:"column:start_at"`
	EndAt   *time.Time `gorm:"column:end_at"`
	// Status active / disabled；停用后立刻退出算价。
	Status string `gorm:"size:32;not null;default:active"`
	Remark string `gorm:"size:255"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// ItemCount 定向条目数（联表统计，非持久化）。
	ItemCount int64 `gorm:"->;-:migration"`
}

// TableName 指定表名。
func (FlashDiscount) TableName() string { return "flash_discounts" }

// FlashDiscountItem 活动定向条目：一个分类或一个商品。
//
// 有定向条目时按条目命中（商品优先于分类）；没有条目时按 Scope 生效。
// 这样「全站打折」只需一条 scope=all 的记录，不必往条目表里塞所有商品。
type FlashDiscountItem struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	DiscountID uint64 `gorm:"column:discount_id;not null;uniqueIndex:uk_flash_discount_items"`
	// TargetType category / product。
	TargetType string `gorm:"column:target_type;size:16;not null;uniqueIndex:uk_flash_discount_items"`
	TargetID   uint64 `gorm:"column:target_id;not null;uniqueIndex:uk_flash_discount_items"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (FlashDiscountItem) TableName() string { return "flash_discount_items" }

// RuntimeStatus 按当前时间推导活动的运行态，供界面显示。
//
// 不落库：状态字段只表达运营的启停意图（active/disabled），
// 「未开始 / 进行中 / 已结束」是时间的函数，存下来必然要有人定时去改，
// 一旦调度漏跑就会出现界面与实际不一致。
func (d FlashDiscount) RuntimeStatus(now time.Time) string {
	if d.Status != StatusActive {
		return "disabled"
	}
	if d.StartAt != nil && now.Before(*d.StartAt) {
		return "pending"
	}
	if d.EndAt != nil && now.After(*d.EndAt) {
		return "expired"
	}
	return "running"
}
