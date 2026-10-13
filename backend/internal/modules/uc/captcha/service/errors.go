package service

import "errors"

var (
	ErrNeedVerification = errors.New("该操作需要二次验证")
	ErrSceneDisabled    = errors.New("该场景验证策略不可用")
	ErrTargetUnbound    = errors.New("账号未绑定对应的手机号或邮箱")
)
