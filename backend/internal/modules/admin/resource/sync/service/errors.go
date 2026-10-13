package service

import "errors"

var (
	ErrPriceChangeHandled = errors.New("该调价事件已处置")
	ErrTaskRunning        = errors.New("已有同步任务在执行中")
)
