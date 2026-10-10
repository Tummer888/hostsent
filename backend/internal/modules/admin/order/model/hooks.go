package model

import (
	"strings"

	"gorm.io/gorm"
)

// 订单 jsonb 列的空值归一化。
//
// orders / order_items 的 config_options 是 jsonb（迁移 075 加入：客户选配项选择是
// 履约的唯一真源），而 Postgres 的 jsonb 不接受空串（22P02: invalid input syntax
// for type json），Go 里 string 字段的零值恰好就是 ""。未选配的创建路径都不设置这个
// 字段 —— 无选配商品下单、续费订单、后台代下单、演示种子 —— 原样落库会让整笔下单
// 被数据库打回（续费链路的 live 用例 TestLiveRenewalDualChain 就是这样失败的）。
//
// 与 logcenter 的 nullJSON 取 `null` 不同，这里写空对象 `{}`：选配项读侧
// （orderConfigSelections / 订单详情）按 map 解码，空对象是更自然的「无选配」表示，
// 且 "" 与 "{}" 的读侧行为一致（都判为无选配）。
//
// 只挂 BeforeCreate 不挂 BeforeSave：这一列在创建后不再被更新路径改写
// （改选配要另开订单），挂 BeforeSave 反而会把 struct 更新牵连进来。
const emptyConfigOptions = "{}"

func (o *Order) BeforeCreate(_ *gorm.DB) error {
	o.ConfigOptions = normalizeConfigOptions(o.ConfigOptions)
	return nil
}

func (i *OrderItem) BeforeCreate(_ *gorm.DB) error {
	i.ConfigOptions = normalizeConfigOptions(i.ConfigOptions)
	return nil
}

func normalizeConfigOptions(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return emptyConfigOptions
	}
	return raw
}
