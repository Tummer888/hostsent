package service

import "errors"

var (
	ErrBillNotPayable = errors.New("账单不可支付")
)
