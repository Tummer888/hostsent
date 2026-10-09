package service

import "errors"

var (
	// ErrSalesDisabled 销售提成功能未开启。
	ErrSalesDisabled = errors.New("销售提成功能未开启")
	// ErrCustomerNotFound 客户不存在或不属于可操作范围。
	ErrCustomerNotFound = errors.New("客户不存在")
	// ErrRelationNotFound 归属关系不存在。
	ErrRelationNotFound = errors.New("归属关系不存在")
	// ErrRelationProtected 归属处于保护期内，不允许改派。
	ErrRelationProtected = errors.New("客户处于归属保护期内，暂不可变更")
	// ErrSameOwner 目标销售与当前归属一致，无需变更。
	ErrSameOwner = errors.New("该客户已归属所选销售")
	// ErrInvalidSales 目标销售不可用（未开启销售能力或已离职）。
	ErrInvalidSales = errors.New("所选销售不可用")
	// ErrReasonRequired 变更原因必填。
	ErrReasonRequired = errors.New("请填写变更原因")
	// ErrOutOfScope 超出数据范围（非本部门/非本人客户）。
	ErrOutOfScope = errors.New("无权操作该客户")

	// ErrInvalidAmount 金额非法。
	ErrInvalidAmount = errors.New("金额非法")
	// ErrBelowMinWithdraw 低于最低提现金额。
	ErrBelowMinWithdraw = errors.New("低于最低提现金额")
	// ErrInsufficientCommission 提成可用余额不足（含为负的欠款场景）。
	ErrInsufficientCommission = errors.New("提成可用余额不足")
	// ErrWithdrawNotFound 提现单不存在。
	ErrWithdrawNotFound = errors.New("提现单不存在")
	// ErrWithdrawStatus 提现单状态不允许当前操作。
	ErrWithdrawStatus = errors.New("提现单状态不允许该操作")
	// ErrPayoutNotConfigured 未装配打款通道，无法发起打款。
	ErrPayoutNotConfigured = errors.New("打款通道未配置")
	// ErrPayoutReceiptRequired 登记打款需要渠道流水号或回执链接之一。
	ErrPayoutReceiptRequired = errors.New("请填写渠道流水号或回执链接")

	// ErrInvalidPeriod 业绩周期格式非法（应为 YYYY-MM）。
	ErrInvalidPeriod = errors.New("业绩周期格式应为 YYYY-MM")
	// ErrInvalidTarget 业绩目标入参非法。
	ErrInvalidTarget = errors.New("业绩目标入参非法")
)
