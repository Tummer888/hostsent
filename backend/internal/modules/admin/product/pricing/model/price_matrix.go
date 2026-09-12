package model

import "time"

// 价格来源（product_prices.source）：区分"同步推导"与"人工填写"。
const (
	// PriceSourceUpstream 上游按周期给的成本价直接采用（未配置加价规则）。
	PriceSourceUpstream = "upstream"
	// PriceSourceMarkup 上游成本价 + 商品加价规则重算所得。
	PriceSourceMarkup = "markup"
	// PriceSourceManual 运营人工填写（自营商品，或人工覆盖）。
	PriceSourceManual = "manual"
)

// ProductPrice 周期价格矩阵（doc25 §3.1）：商品 × 规格 × 周期 一个价。
//
// 与 ProductPricing（product_pricing 表，计费模板/基础价）的区别：
//   - ProductPricing 回答"这商品基础单价多少"，单值；
//   - ProductPrice 回答"这商品按月/季/年各卖多少"，是矩阵。
//
// Price 存**折后价**（上游本身就是按周期给折后价，如年付 = 月付 × 10），
// 折扣策略（price_policies）在下单算价时于该基数之上再做二次叠加。
type ProductPrice struct {
	ID uint64 `gorm:"primaryKey;autoIncrement"`
	// ProductID 售出商品（products.id）。
	ProductID uint64 `gorm:"column:product_id;not null;uniqueIndex:uk_product_prices_cycle,priority:1;index:idx_product_prices_product"`
	// ProductSpecID SKU（product_specs.id）；0 表示商品级价格。
	ProductSpecID uint64 `gorm:"column:product_spec_id;not null;default:0;uniqueIndex:uk_product_prices_cycle,priority:2;index:idx_product_prices_product"`
	// Cycle 计费周期，取值见 pkg/billingcycle（monthly/quarterly/annually/...）。
	Cycle    string `gorm:"column:cycle;size:20;not null;uniqueIndex:uk_product_prices_cycle,priority:3"`
	Currency string `gorm:"column:currency;size:8;not null;default:CNY;uniqueIndex:uk_product_prices_cycle,priority:4"`
	// Price 售价（对外，已含上游周期折扣）。
	Price float64 `gorm:"column:price;type:decimal(12,2);not null;default:0"`
	// CostPrice 成本（上游成本或自建成本），供实例对账使用。
	CostPrice float64 `gorm:"column:cost_price;type:decimal(12,2);not null;default:0"`
	// SetupFee / CostSetupFee 初装费（售价侧 / 成本侧）。
	SetupFee     float64 `gorm:"column:setup_fee;type:decimal(12,2);not null;default:0"`
	CostSetupFee float64 `gorm:"column:cost_setup_fee;type:decimal(12,2);not null;default:0"`
	// Source 该行的来源（upstream / markup / manual），决定是否允许人工改价。
	Source string `gorm:"column:source;size:16;not null;default:manual"`
	// Status 1 启用（可售）/ 0 停用（前台不可选、下单被拒）。
	// 刻意不设 GORM default：status=0 是合法显式值，带 default 标签会让
	// GORM 在 Create 时把 0 当成"未设置"而用默认值替代（停用档位被存成启用）。
	Status    int       `gorm:"not null"`
	Remark    string    `gorm:"size:255"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (ProductPrice) TableName() string { return "product_prices" }
