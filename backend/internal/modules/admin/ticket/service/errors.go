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
	// ErrReplyContentRequired 回复内容不能为空（内部备注同样不允许空白）
	ErrReplyContentRequired = errors.New("回复内容不能为空")
	// ErrTicketAssigned 工单已被他人认领/分配，无法重复认领（P2-03）
	ErrTicketAssigned = errors.New("工单已被认领")
	// ErrSameAssignee 转派目标与当前处理人相同
	ErrSameAssignee = errors.New("转派目标与当前处理人相同")
	// —— S2 提交前置条件 ——
	// ErrRealnameRequired 该分类要求先完成实名认证
	ErrRealnameRequired = errors.New("该问题分类需先完成实名认证")
	// ErrBindingRequired 该分类要求关联本人名下的订单或实例
	ErrBindingRequired = errors.New("该问题分类需关联订单或实例")
	// ErrBindingNotOwned 关联的订单/实例不属于当前账号
	ErrBindingNotOwned = errors.New("关联的订单或实例不属于当前账号")
	// ErrCategoryNotAllowed 当前账号角色不可提交该分类
	ErrCategoryNotAllowed = errors.New("当前账号无权提交该问题分类")
	// ErrAttachmentTooLarge 附件超过大小上限
	ErrAttachmentTooLarge = errors.New("附件超过大小上限")
	// ErrAttachmentNotFound 附件不存在或不属于该工单
	ErrAttachmentNotFound = errors.New("附件不存在")
	// ErrAttachmentTypeNotAllowed 附件类型不在白名单内
	ErrAttachmentTypeNotAllowed = errors.New("附件类型不支持")
	// —— S3 双人复核 ——
	// ErrReviewSelf 不能复核自己提交的回复（双人复核的核心约束）
	ErrReviewSelf = errors.New("不能复核自己提交的回复")
	// ErrNotPendingReview 该回复不处于待复核状态
	ErrNotPendingReview = errors.New("该回复不处于待复核状态")
	// ErrReviewNoteRequired 驳回必须填写原因
	ErrReviewNoteRequired = errors.New("驳回必须填写复核意见")
	// ErrInvalidReviewAction 复核动作非法
	ErrInvalidReviewAction = errors.New("复核动作非法")
	// ErrInternalNoteForbidden 无内部备注权限
	ErrInternalNoteForbidden = errors.New("无内部备注权限")
)
