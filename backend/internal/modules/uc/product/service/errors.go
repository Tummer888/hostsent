package service

import "errors"

var (
	ErrProductOffline = errors.New("商品已下架，不可购买")
)
