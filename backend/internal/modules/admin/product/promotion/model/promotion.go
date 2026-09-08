// Package model 提供促销管理（promotion）子域的数据模型。
package model

import "time"

// 优惠券类型
const (
	CouponTypeFull     string = "full"     // 满减券
	CouponTypeDiscount string = "discount" // 折扣券
	CouponTypeVoucher  string = "voucher"  // 代金券
)

// 优惠券状态
const (
	CouponStatusOffline int = 0 // 停用/未启用
	CouponStatusActive  int = 1 // 生效中
	CouponStatusExpired int = 2 // 已过期
)

// CouponGrant 状态
const (
	CouponGrantIssued   string = "issued"   // 已发放
	CouponGrantUsed     string = "used"     // 已使用
	CouponGrantExpired  string = "expired"  // 已过期
)

// 促销活动类型
const (
	PromotionTypeDiscount string = "discount" // 折扣活动/限时折扣
	PromotionTypeNewUser  string = "newuser"  // 新用户专享
	PromotionTypeBundle   string = "bundle"   // 套餐组合
)

// 促销活动状态
const (
	PromotionStatusDraft  int = 0 // 草稿
	PromotionStatusActive int = 1 // 进行中
	PromotionStatusEnded  int = 2 // 已结束
)

// Coupon 优惠券。
type Coupon struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	CouponCode   string     `gorm:"column:coupon_code;size:64;uniqueIndex;not null"` // 优惠券编码
	Name         string     `gorm:"size:100;not null"`                               // 优惠券名称
	CouponType   string     `gorm:"column:coupon_type;size:20;not null"`             // 类型
	Amount       float64    `gorm:"column:amount;type:decimal(10,2)"`                // 满减/代金金额
	MinAmount    float64    `gorm:"column:min_amount;type:decimal(10,2)"`            // 满额门槛
	Discount     float64    `gorm:"column:discount;type:decimal(10,2);default:0"`    // 折扣（折扣券，0.8=8折）
	TotalStock   int        `gorm:"column:total_stock;default:0"`                    // 发放总量，0 表示不限
	ClaimedCount int        `gorm:"column:claimed_count;default:0"`                  // 已领取数
	PerUserLimit int        `gorm:"column:per_user_limit;default:1"`                 // 每人限领
	Scope        string     `gorm:"type:text"`                                       // 适用范围 JSON
	ValidFrom    *time.Time `gorm:"column:valid_from;index"`                         // 有效期开始
	ValidTo      *time.Time `gorm:"column:valid_to;index"`                           // 有效期结束
	Status       int        `gorm:"default:1"`                                       // 状态
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Coupon) TableName() string {
	return "coupons"
}

// CouponGrant 优惠券发放记录。
type CouponGrant struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement"`
	CouponID  uint64     `gorm:"column:coupon_id;not null;index"`                   // 优惠券 ID
	UserID    uint64     `gorm:"column:user_id;index"`                              // 发放对象用户 ID
	UserName  string     `gorm:"column:user_name;size:100"`                         // 用户名快照
	Status    string     `gorm:"column:status;size:20;default:'issued';index"`      // 状态
	UsedAt    *time.Time `gorm:"column:used_at"`                                    // 使用时间
	ValidFrom *time.Time `gorm:"column:valid_from"`                                 // 生效时间
	ValidTo   *time.Time `gorm:"column:valid_to"`                                   // 失效时间
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (CouponGrant) TableName() string {
	return "coupon_grants"
}

// Promotion 促销活动（折扣/新用户专享/套餐组合）。
type Promotion struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	Name          string     `gorm:"size:100;not null"`                       // 活动名称
	PromotionType string     `gorm:"column:promotion_type;size:20;not null"`  // 类型
	Rule          string     `gorm:"type:text"`                               // 规则 JSON
	StartTime     *time.Time `gorm:"column:start_time"`                       // 开始时间
	EndTime       *time.Time `gorm:"column:end_time"`                         // 结束时间
	SortOrder     int        `gorm:"column:sort_order;default:0"`              // 排序
	Status        int        `gorm:"default:0;index"`                         // 状态
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Promotion) TableName() string {
	return "promotions"
}
