package service

import "errors"

var (
	ErrArticleNotFound   = errors.New("文章不存在")
	ErrInvalidKind       = errors.New("不支持的内容类型")
	ErrSlugRequired      = errors.New("无法生成有效标识，请手动填写 slug")
	ErrCategoryNotFound  = errors.New("分类不存在")
	ErrCategoryHasChild  = errors.New("分类下存在子分类，无法删除")
	ErrCategoryKindMix   = errors.New("分类类型与文章类型不一致")
	ErrLinkNotFound      = errors.New("友情链接不存在")
	ErrLinkURLRequired   = errors.New("链接地址不能为空")
	ErrTermsBodyRequired = errors.New("条款/隐私政策的正文不能为空")
)
