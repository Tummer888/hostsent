// Package dto 提供钱包账户子域的数据传输结构。
package dto

// WalletInfo 用户钱包信息
type WalletInfo struct {
	UserID       uint64  `json:"user_id"`
	Balance      float64 `json:"balance"`       // 可用余额
	Frozen       float64 `json:"frozen"`        // 冻结金额
	TotalIncome  float64 `json:"total_income"`  // 累计收入
	TotalExpense float64 `json:"total_expense"` // 累计支出
	Version      uint64  `json:"version"`
}

// AdjustRequest 人工调账（赠送/扣减）请求
type AdjustRequest struct {
	UserID    uint64  `json:"user_id" binding:"required"`
	Type      string  `json:"type"`                         // 默认 adjust
	Direction int     `json:"direction" binding:"required"` // 1=收入 -1=支出
	Amount    float64 `json:"amount" binding:"required"`
	BizKey    string  `json:"biz_key" binding:"required"` // 幂等键
	Remark    string  `json:"remark"`
}
