package service

import "errors"

// 工单域业务错误
var (
	// ErrTicketNotFound 工单不存在
	ErrTicketNotFound = errors.New("工单不存在")
	// ErrReplyNotFound 回复不存在
	ErrReplyNotFound = errors.New("回复不存在")
	// ErrCategoryNotFound 工单分类不存在
	ErrCategoryNotFound = errors.New("工单分类不存在")
	// ErrCategoryCodeExists 分类编码已存在
	ErrCategoryCodeExists = errors.New("分类编码已存在")
	// ErrCategoryInUse 分类使用中，无法删除
	ErrCategoryInUse = errors.New("分类使用中，无法删除")
	// ErrStatusConflict 工单状态不允许该操作
	ErrStatusConflict = errors.New("工单状态不允许该操作")
	// ErrAdminNotFound 处理人不存在
	ErrAdminNotFound = errors.New("处理人不存在")
	// ErrInvalidPriority 优先级非法
	ErrInvalidPriority = errors.New("优先级非法")
	// ErrTicketAssigned 工单已被他人认领/分配，无法重复认领（P2-03）
	ErrTicketAssigned = errors.New("工单已被认领")
	// ErrSameAssignee 转派目标与当前处理人相同
	ErrSameAssignee = errors.New("转派目标与当前处理人相同")
)
