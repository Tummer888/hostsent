package service

import "errors"

// 订单域业务错误
var (
	// ErrStatusConflict 订单/退款状态不允许该操作
	ErrStatusConflict = errors.New("订单状态不允许该操作")
	// ErrOrderNotFound 订单不存在
	ErrOrderNotFound = errors.New("订单不存在")
	// ErrRefundNotFound 退款单不存在
	ErrRefundNotFound = errors.New("退款单不存在")
	// ErrRefundExceeded 退款金额超过可退金额
	ErrRefundExceeded = errors.New("退款金额超过可退金额")
	// ErrRefundModeInvalid 退款去向非法（仅支持 balance / channel，doc36 §3.2）
	ErrRefundModeInvalid = errors.New("退款去向非法，仅支持 balance（退回余额）或 channel（原路退回）")
	// ErrRefundFeeInvalid 渠道扣点非法（不得为负或超过退款本金）
	ErrRefundFeeInvalid = errors.New("渠道扣点不得为负或超过退款本金")
	// ErrVerifyNotAllowed 当前订单状态不允许核验（核验对象是已开通的机器：
	// 未支付/开通中/已作废的订单没有可核对的实体）
	ErrVerifyNotAllowed = errors.New("订单未开通完成，暂不可核验")
	// ErrOrderHasRefund 订单存在退款记录，不允许删除（资金纠纷线索必须保留）
	ErrOrderHasRefund = errors.New("该订单存在退款记录，不可删除")
	// ErrBatchEmpty 批量操作未提供有效 ID
	ErrBatchEmpty = errors.New("请至少选择一条记录")
	// ErrBatchTooLarge 批量操作超过单次上限
	ErrBatchTooLarge = errors.New("单次批量操作最多 100 条")
	// ErrRefundNotAllowed 订单不符合退款策略（未命中策略/已过时限/状态不支持），
	// 客户自助申请被拒；管理端发起退款不受此限制。
	ErrRefundNotAllowed = errors.New("该订单当前不支持自助申请退款")
	// ErrRefundChannelUnavailable 该订单没有渠道支付记录，不能选择原路退回
	// （余额支付的订单资金本就在平台内，只能退回余额）。
	ErrRefundChannelUnavailable = errors.New("该订单为余额支付，仅支持退回余额")
	// ErrPayMethodInvalid 确认收款必须指定合法的支付方式
	// （不能留空，也不能填 channel/online 这类"支付模式"而非"收款方式"）。
	ErrPayMethodInvalid = errors.New("请选择有效的支付方式")
	// ErrConfirmAmountInvalid 确认收款的实收金额非法。
	ErrConfirmAmountInvalid = errors.New("实收金额必须大于 0")
	// ErrPayTimeInvalid 到账时间格式非法（需 2006-01-02 15:04:05 或 RFC3339）。
	ErrPayTimeInvalid = errors.New("到账时间格式不正确")
	// ErrOfflinePaymentUnavailable 支付台账登记能力未配置，无法确认收款。
	// 宁可报错也不静默放过：只改订单状态而账上查不到这笔钱，是财务事故。
	ErrOfflinePaymentUnavailable = errors.New("支付台账登记能力未配置，无法确认收款")
	// ErrBalanceDeductUnavailable 余额扣款能力未配置，无法按余额支付确认收款。
	ErrBalanceDeductUnavailable = errors.New("余额扣款能力未配置，无法按余额支付确认收款")
	// ErrInsufficientBalance 客户余额不足以完成本次收款（余额支付路径）。
	ErrInsufficientBalance = errors.New("客户余额不足")
)
