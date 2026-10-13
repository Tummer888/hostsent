package service

import "errors"

var (
	ErrCategoryHasChildren = errors.New("分类下存在子分类，无法删除")
)
