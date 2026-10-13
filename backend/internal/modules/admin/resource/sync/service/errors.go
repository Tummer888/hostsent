package service

import "errors"

var (
	ErrPriceChangeHandled = errors.New("该调价事件已处置")
	errScopeNoHandler     = errors.New("该同步类型暂无可用处理器")
	ErrTaskRunning        = errors.New("已有同步任务在执行中")
)
