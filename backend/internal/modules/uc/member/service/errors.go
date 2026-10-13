package service

import "errors"

var (
	ErrNotAccountOwner = errors.New("仅主账号可管理成员")
	// ErrMemberQuotaExceeded 子账号数量超出等级上限。
	ErrMemberQuotaExceeded = errors.New("成员数量已达当前等级上限，请提升等级或删除闲置成员")
	// ErrMemberNotFound 成员不存在或不属于当前账号。
	ErrMemberNotFound = errors.New("成员不存在")
	// ErrUsernameTaken 用户名已被占用。
	ErrUsernameTaken = errors.New("用户名已存在")
	// ErrEmailTaken 邮箱已被占用。
	ErrEmailTaken = errors.New("邮箱已被注册")
	// ErrPhoneTaken 手机号已被占用。
	ErrPhoneTaken = errors.New("手机号已被使用")
	// ErrInvalidStatus 非法的状态值。
	ErrInvalidStatus = errors.New("非法的成员状态")
)
