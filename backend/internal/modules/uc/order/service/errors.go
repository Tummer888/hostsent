package service

import "errors"

var (
	ErrProductOffline = errors.New("商品已下架，不可购买")
	// ErrInsufficientBalance 余额不足
	ErrInsufficientBalance = errors.New("余额不足，请先充值")
	// ErrProvisionFailed 开通上游失败（已扣款，可稍后在订单页重试）
	ErrProvisionFailed = errors.New("开通资源失败，已扣款，可稍后重试")
	// ErrOrderNotFound 订单不存在或不属于当前账号（两者对外同一文案，避免探测他人订单号）
	ErrOrderNotFound = errors.New("订单不存在")
	// ErrOrderNotPayable 订单当前状态不可支付（已支付/已取消/已超时关单）
	ErrOrderNotPayable = errors.New("订单当前状态不可支付，请刷新后重试")
	// ErrOrderNotCancellable 仅待支付订单可取消
	ErrOrderNotCancellable = errors.New("仅待支付订单可取消")
	// ErrPayModeInvalid 支付方式非法
	ErrPayModeInvalid = errors.New("不支持的支付方式")
)
