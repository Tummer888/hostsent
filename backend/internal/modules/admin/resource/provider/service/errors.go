package service

import "errors"

var (
	ErrPlatformResourcesUnsupported = errors.New("该渠道未提供平台资源目录读取能力")
	ErrAccountBalanceUnsupported    = errors.New("该渠道类型暂不支持自动查询余额，请手工录入快照")
	ErrUpstreamLedgerUnsupported    = errors.New("该渠道类型暂不支持读取上游账本（消费/充值流水），请手工录入")
)
