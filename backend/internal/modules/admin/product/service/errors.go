package service

import "errors"

// 业务错误
var (
	// ErrCategoryHasChildren 存在子分类，禁止删除
	ErrCategoryHasChildren = errors.New("分类下存在子分类，无法删除")
	// ErrProductInvalidStatus 非法状态流转
	ErrProductInvalidStatus = errors.New("非法的产品状态流转")
)
