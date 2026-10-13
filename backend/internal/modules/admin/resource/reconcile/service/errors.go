package service

import "errors"

// 差异台账处置相关错误。
var (
	// ErrInvalidFindingStatus 非法处置状态（只接受 open/ack/ignored）。
	ErrInvalidFindingStatus = errors.New("非法的差异处置状态")
	// ErrFindingNotFound 差异行不存在。
	ErrFindingNotFound = errors.New("差异记录不存在")
)
