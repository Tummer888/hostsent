package service

import "errors"

var (
	ErrConfigKeyDuplicate = errors.New("配置键已存在")
)
