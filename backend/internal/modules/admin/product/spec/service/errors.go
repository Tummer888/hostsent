package service

import "errors"

var (
	ErrSpecValidationFailed = errors.New("规格不符合原子字典约束")
	ErrOptionSpecNotFound   = errors.New("平台配置项不存在")
	ErrProviderTypeUnknown  = errors.New("无法确定平台类型：请提供 provider_type，或选择已接入的渠道")
)
