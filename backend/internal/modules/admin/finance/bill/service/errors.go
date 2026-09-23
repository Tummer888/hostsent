package service

import "errors"

// 账单子域业务错误
var (
	// ErrStatusConflict 状态不允许该操作
	ErrStatusConflict = errors.New("状态不允许该操作")
	// ErrBillNotFound 账单不存在
	ErrBillNotFound = errors.New("账单不存在")
	// ErrBillNotInvoicable 账单不可开票（未结清）
	ErrBillNotInvoicable = errors.New("账单未结清，暂不可申请开票")
	// ErrBillZeroAmountNotInvoicable 应结金额为 0 的账单不可开票（充值账单/空壳账单）
	ErrBillZeroAmountNotInvoicable = errors.New("该账单应结金额为 0，无需开票")
	// ErrAlreadyInvoiced 账单已开票
	ErrAlreadyInvoiced = errors.New("该账单已开票")
	// ErrInvoicePending 账单已有待处理的开票申请
	ErrInvoicePending = errors.New("该账单已有待处理的开票申请")
	// ErrInvoiceNotFound 发票申请不存在
	ErrInvoiceNotFound = errors.New("发票申请不存在")
	// ErrInvoiceStatusConflict 发票申请状态不允许该操作
	ErrInvoiceStatusConflict = errors.New("发票申请状态不允许该操作")
	// ErrInvoiceFileNotReady 发票文件未就绪（未开票，或开票时未回填文件地址）
	ErrInvoiceFileNotReady = errors.New("发票文件尚未就绪，请开票后再下载")
	// ErrInvoiceEmailNotReady 邮件下发未接入（预埋：待接税务/邮件网关后启用）
	ErrInvoiceEmailNotReady = errors.New("发票邮件下发尚未接入，请联系客服人工发送")
	// ErrRechargeNoRequired 充值账单必须带充值单号（幂等键与逐笔可追溯都靠它）
	ErrRechargeNoRequired = errors.New("充值账单缺少充值单号")
	// ErrRechargeAmountInvalid 充值账单金额必须为正
	ErrRechargeAmountInvalid = errors.New("充值账单金额必须大于 0")
)
