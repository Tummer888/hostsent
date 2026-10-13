package handler

import (
	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	rechdto "hostsent/backend/internal/modules/admin/finance/account/recharge/dto"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
)

// 用户中心资金流水/充值单视图：只保留客户对账需要的字段。
// 经办人 ID（平台操作员）、渠道流水号、支付单主键属于平台内部账务口径，
// 不向客户下发；账单/发票见各自 handler。

type customerTransaction struct {
	ID            uint64  `json:"id"`
	TxNo          string  `json:"tx_no"`
	Type          string  `json:"type"`
	Direction     int     `json:"direction"`
	Amount        float64 `json:"amount"`
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	OrderNo       string  `json:"order_no"`
	RefNo         string  `json:"ref_no"`
	Remark        string  `json:"remark"`
	CreatedAt     string  `json:"created_at"`
}

type customerTransactionSummary struct {
	IncomeTotal  float64 `json:"income_total"`
	ExpenseTotal float64 `json:"expense_total"`
	NetTotal     float64 `json:"net_total"`
	TxCount      int64   `json:"tx_count"`
}

type customerTransactionList struct {
	Items   []customerTransaction       `json:"items"`
	Meta    transdto.ListMeta           `json:"meta"`
	Summary *customerTransactionSummary `json:"summary"`
}

func customerTransactionListFrom(in *transdto.TransactionListResponse) *customerTransactionList {
	if in == nil {
		return nil
	}
	items := make([]customerTransaction, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, customerTransaction{
			ID:            it.ID,
			TxNo:          it.TxNo,
			Type:          it.Type,
			Direction:     it.Direction,
			Amount:        it.Amount,
			BalanceBefore: it.BalanceBefore,
			BalanceAfter:  it.BalanceAfter,
			OrderNo:       it.OrderNo,
			RefNo:         it.RefNo,
			Remark:        it.Remark,
			CreatedAt:     it.CreatedAt,
		})
	}
	return &customerTransactionList{
		Items: items,
		Meta:  in.Meta,
		Summary: &customerTransactionSummary{
			IncomeTotal:  in.Summary.IncomeTotal,
			ExpenseTotal: in.Summary.ExpenseTotal,
			NetTotal:     in.Summary.NetTotal,
			TxCount:      in.Summary.TxCount,
		},
	}
}

type customerRecharge struct {
	ID          uint64  `json:"id"`
	RechargeNo  string  `json:"recharge_no"`
	Amount      float64 `json:"amount"`
	Method      string  `json:"method"`
	Status      string  `json:"status"`
	ChannelCode string  `json:"channel_code"`
	PaidAt      string  `json:"paid_at"`
	CreatedAt   string  `json:"created_at"`
}

type customerRechargeList struct {
	Items []customerRecharge `json:"items"`
	Meta  rechdto.ListMeta   `json:"meta"`
}

func customerRechargeListFrom(in *rechdto.RechargeListResponse) *customerRechargeList {
	if in == nil {
		return nil
	}
	items := make([]customerRecharge, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, customerRecharge{
			ID:          it.ID,
			RechargeNo:  it.RechargeNo,
			Amount:      it.Amount,
			Method:      it.Method,
			Status:      it.Status,
			ChannelCode: it.ChannelCode,
			PaidAt:      it.PaidAt,
			CreatedAt:   it.CreatedAt,
		})
	}
	return &customerRechargeList{Items: items, Meta: in.Meta}
}

func customerRechargeFrom(in *rechdto.RechargeInfo) *customerRecharge {
	if in == nil {
		return nil
	}
	return &customerRecharge{
		ID:          in.ID,
		RechargeNo:  in.RechargeNo,
		Amount:      in.Amount,
		Method:      in.Method,
		Status:      in.Status,
		ChannelCode: in.ChannelCode,
		PaidAt:      in.PaidAt,
		CreatedAt:   in.CreatedAt,
	}
}

// customerWallet 我的钱包：只保留金额口径。
// version 是服务端的乐观锁计数（内部并发控制），客户端不消费，不下发。
type customerWallet struct {
	Balance      float64 `json:"balance"`
	Frozen       float64 `json:"frozen"`
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
}

func customerWalletFrom(in *accountdto.WalletInfo) *customerWallet {
	if in == nil {
		return nil
	}
	return &customerWallet{
		Balance:      in.Balance,
		Frozen:       in.Frozen,
		TotalIncome:  in.TotalIncome,
		TotalExpense: in.TotalExpense,
	}
}
