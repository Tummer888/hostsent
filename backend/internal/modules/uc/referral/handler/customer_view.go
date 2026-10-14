package handler

import (
	referraldto "hostsent/backend/internal/modules/admin/finance/referral/dto"
)

// 用户中心返现提现视图：审核人姓名与审核备注属于平台内部经办信息，不对客户下发；
// 客户只需要知道审核结果与时间。

type customerWithdrawal struct {
	ID         uint64  `json:"id"`
	WithdrawNo string  `json:"withdraw_no"`
	Amount     float64 `json:"amount"`
	Channel    string  `json:"channel"`
	Account    string  `json:"account"`
	Status     string  `json:"status"`
	AuditedAt  string  `json:"audited_at"`
	CreatedAt  string  `json:"created_at"`
}

type customerWithdrawalList struct {
	Items []customerWithdrawal `json:"items"`
	Meta  referraldto.ListMeta `json:"meta"`
}

func customerWithdrawalListFrom(in *referraldto.WithdrawalListResponse) *customerWithdrawalList {
	if in == nil {
		return nil
	}
	items := make([]customerWithdrawal, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, customerWithdrawalFrom(it))
	}
	return &customerWithdrawalList{Items: items, Meta: in.Meta}
}

func customerWithdrawalFrom(in referraldto.WithdrawalInfo) customerWithdrawal {
	return customerWithdrawal{
		ID:         in.ID,
		WithdrawNo: in.WithdrawNo,
		Amount:     in.Amount,
		Channel:    in.Channel,
		Account:    in.Account,
		Status:     in.Status,
		AuditedAt:  in.AuditedAt,
		CreatedAt:  in.CreatedAt,
	}
}
