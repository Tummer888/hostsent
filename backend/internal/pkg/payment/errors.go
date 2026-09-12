package payment

import (
	"errors"
	"fmt"
)

// 支付域业务错误。
var (
	// ErrNotImplemented 适配器未实现该能力。
	ErrNotImplemented = errors.New("支付渠道未实现该能力")
	// ErrChannelDisabled 渠道已停用。
	ErrChannelDisabled = errors.New("支付渠道已停用")
	// ErrUnsupportedScene 渠道不支持该支付场景。
	ErrUnsupportedScene = errors.New("支付渠道不支持该支付场景")
	// ErrAmountOutOfRange 金额超出渠道限额。
	ErrAmountOutOfRange = errors.New("金额超出渠道限额")
	// ErrNoChannelAvailable 无可用支付渠道。
	ErrNoChannelAvailable = errors.New("无可用支付渠道")
	// ErrSignatureInvalid 回调验签失败。
	ErrSignatureInvalid = errors.New("支付回调验签失败")
	// ErrAmountMismatch 回调金额与本地支付单不一致。
	ErrAmountMismatch = errors.New("支付回调金额与本地支付单不一致")
	// ErrOrderNotPayable 支付单不可支付（已关闭/已支付）。
	ErrOrderNotPayable = errors.New("支付单当前状态不可支付")
	// ErrChannelNotFound 渠道不存在。
	ErrChannelNotFound = errors.New("支付渠道不存在")
	// ErrOrderNotFound 支付单不存在。
	ErrOrderNotFound = errors.New("支付单不存在")
)

// GatewayError 适配器统一错误：携带渠道、操作与底层错误，便于排障。
type GatewayError struct {
	Channel   string
	Op        string
	Code      string
	Msg       string
	Err       error
	Transient bool // 是否可重试（网络/限流类）
}

func (e *GatewayError) Error() string {
	msg := e.Msg
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("payment %s[%s] code=%s msg=%s", e.Op, e.Channel, e.Code, msg)
}

func (e *GatewayError) Unwrap() error { return e.Err }
