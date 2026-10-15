package service

import "errors"

var (
	ErrCategoryHasChildren = errors.New("分类下存在子分类，无法删除")
	// ErrCategoryInUse 分类下仍有商品（含其子树）。
	// 允许删除会让这些商品的 category_id 变成悬空值 ——
	// 界面显示不出分类名、也统计不进任何分类，只能落进"未分类"。
	ErrCategoryInUse = errors.New("分类下仍有商品，请先把这些商品移到其它分类再删除")
)
