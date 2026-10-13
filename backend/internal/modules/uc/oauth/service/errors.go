package service

import "errors"

var (
	ErrProviderUnknown = errors.New("未知的第三方登录渠道")
	// ErrProviderNotConfigured 渠道存在但未启用或凭证不可用。
	ErrProviderNotConfigured = errors.New("该第三方登录渠道未启用")
	// ErrBindingNotFound 当前用户没有该渠道的绑定。
	ErrBindingNotFound = errors.New("未绑定该第三方账号")
	// ErrNoInviteCode 保留给未来扩展；当前不返回。
	ErrNoInviteCode = errors.New("邀请码无效")
)
