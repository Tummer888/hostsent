package service

import (
	"errors"

	"hostsent/backend/internal/modules/admin/product/flashdiscount/repository"
)

var (
	ErrInvalidValue = errors.New("折扣值非法：折扣率需在 0 到 1 之间，直减金额需大于 0")
	// ErrInvalidWindow 生效窗口非法（结束早于开始）。
	ErrInvalidWindow = errors.New("结束时间不能早于开始时间")
	// ErrInvalidItem 定向条目非法。
	ErrInvalidItem = errors.New("定向条目必须选择具体的分类或商品")
	// ErrBelowCost 折扣低于目标成本率，会亏本。
	ErrBelowCost = errors.New("折扣低于目标成本率，会亏本销售")
	// ErrNoTarget 既没有定向条目、作用范围也不是全站 —— 活动不会命中任何商品。
	ErrNoTarget = errors.New("请选择作用范围或添加定向分类/商品，否则活动不会命中任何商品")
	// ErrNotFound / ErrCodeTaken 是 repository 层同义哨兵的 service 再出口：
	// handler 只允许依赖本包，经本包判断语义（CONTRIBUTING §3）。
	ErrNotFound = repository.ErrNotFound
	// ErrCodeTaken 活动编码重复。
	ErrCodeTaken = repository.ErrCodeTaken
)
