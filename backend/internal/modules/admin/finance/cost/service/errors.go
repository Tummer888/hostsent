package service

import "errors"

var (
	ErrItemNotFound = errors.New("成本项不存在")
	// ErrInvalidItem 成本项参数非法。
	ErrInvalidItem = errors.New("成本项参数非法")
	// ErrProviderNotFound 渠道不存在或不可用。
	ErrProviderNotFound = errors.New("渠道不存在")
	// ErrBalanceUnsupported 该渠道不支持自动查询余额（适配器未实现 AccountReader）。
	ErrBalanceUnsupported = errors.New("该渠道类型暂不支持自动查询余额")
	// ErrLedgerUnsupported 该渠道不支持读取上游账本（适配器未实现 FinanceLedgerReader）。
	ErrLedgerUnsupported = errors.New("该渠道类型暂不支持读取上游账本（消费/充值流水）")
)
