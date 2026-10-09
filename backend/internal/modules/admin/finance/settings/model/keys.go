// Package model 提供财务参数的键定义（唯一定义处）。
//
// 为什么单独抽一个 model 包：三个消费方（账务核心 / 对账 / 统计）与 seed、财务参数页
// 都要引用同一组键，键名分散定义必然漂移 —— 漂移的表现是「页面保存了但没人读」，
// 正是本轮要修掉的历史问题（5 个 finance_* 键整仓零引用）。这里刻意不含业务逻辑。
package model

// 配置分组：可写财务参数统一落 system_configs 的该分组。
const ConfigGroup = "finance"

// 财务参数键。
const (
	// ConfigKeyAdjustEnabled 允许人工调账：关闭后调账接口直接拒绝。
	ConfigKeyAdjustEnabled = "finance.adjust_enabled"
	// ConfigKeyReconTolerance 对账差异容差（元）：超过该值判为账实可疑。
	ConfigKeyReconTolerance = "finance.recon_tolerance"
	// ConfigKeyBalanceWarning 余额预警阈值（元）：总览统计低余额钱包。
	ConfigKeyBalanceWarning = "finance.balance_warning"
)

// 默认值：键缺失或取值非法时各消费方回落到此值（与 seed 写入的默认值一致）。
const (
	DefaultAdjustEnabled  = "true"
	DefaultReconTolerance = "0.01"
	DefaultBalanceWarning = "50"
)
