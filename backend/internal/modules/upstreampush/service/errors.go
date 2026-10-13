package service

import "errors"

var (
	ErrSignature = errors.New("签名验证失败")
)
