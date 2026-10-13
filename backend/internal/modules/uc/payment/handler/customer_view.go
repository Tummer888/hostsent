package handler

import (
	withdrawdto "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/dto"
	paydto "hostsent/backend/internal/modules/admin/system/payment/dto"
)

// 用户中心支付视图：只下发「付多少钱、怎么付」需要的字段。
// 渠道凭证、接口地址、回调地址、费率、结算方式、健康状态与内部备注
// （如渠道分组、兜底标记）属于平台侧信息，不对客户暴露。

type customerChannel struct {
	ID           uint64   `json:"id"`
	ChannelCode  string   `json:"channel_code"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	TypeName     string   `json:"type_name"`
	Mode         string   `json:"mode"`
	Scenes       []string `json:"scenes"`
	MinAmountFen int64    `json:"min_amount_fen"`
	MaxAmountFen int64    `json:"max_amount_fen"`
}

type customerMethodOptions struct {
	Scene    string            `json:"scene"`
	Channels []customerChannel `json:"channels"`
	Default  string            `json:"default"`
}

func customerMethodOptionsFrom(in *paydto.MethodOptionsResponse) *customerMethodOptions {
	if in == nil {
		return nil
	}
	channels := make([]customerChannel, 0, len(in.Channels))
	for _, ch := range in.Channels {
		channels = append(channels, customerChannel{
			ID:           ch.ID,
			ChannelCode:  ch.ChannelCode,
			Name:         ch.Name,
			Type:         ch.Type,
			TypeName:     ch.TypeName,
			Mode:         ch.Mode,
			Scenes:       ch.Scenes,
			MinAmountFen: ch.MinAmountFen,
			MaxAmountFen: ch.MaxAmountFen,
		})
	}
	return &customerMethodOptions{Scene: in.Scene, Channels: channels, Default: in.Default}
}

// customerPaymentOrder 收银台支付单（去掉渠道流水号、手续费、平台备注与渠道原始参数）。
type customerPaymentOrder struct {
	ID           uint64  `json:"id"`
	PaymentNo    string  `json:"payment_no"`
	BizType      string  `json:"biz_type"`
	BizID        uint64  `json:"biz_id"`
	BizNo        string  `json:"biz_no"`
	Amount       float64 `json:"amount"`
	AmountFen    int64   `json:"amount_fen"`
	Currency     string  `json:"currency"`
	ChannelCode  string  `json:"channel_code"`
	ChannelType  string  `json:"channel_type"`
	ChannelName  string  `json:"channel_name"`
	Scene        string  `json:"scene"`
	Status       string  `json:"status"`
	PayURL       string  `json:"pay_url"`
	QRCode       string  `json:"qrcode"`
	Instructions string  `json:"instructions"`
	Subject      string  `json:"subject"`
	ExpireAt     string  `json:"expire_at"`
	PaidAt       string  `json:"paid_at"`
	CreatedAt    string  `json:"created_at"`
}

func customerPaymentOrderFrom(in *paydto.OrderInfo) *customerPaymentOrder {
	if in == nil {
		return nil
	}
	return &customerPaymentOrder{
		ID:           in.ID,
		PaymentNo:    in.PaymentNo,
		BizType:      in.BizType,
		BizID:        in.BizID,
		BizNo:        in.BizNo,
		Amount:       in.Amount,
		AmountFen:    in.AmountFen,
		Currency:     in.Currency,
		ChannelCode:  in.ChannelCode,
		ChannelType:  in.ChannelType,
		ChannelName:  in.ChannelName,
		Scene:        in.Scene,
		Status:       in.Status,
		PayURL:       in.PayURL,
		QRCode:       in.QRCode,
		Instructions: in.Instructions,
		Subject:      in.Subject,
		ExpireAt:     in.ExpireAt,
		PaidAt:       in.PaidAt,
		CreatedAt:    in.CreatedAt,
	}
}

// customerWithdraw 提现单（去掉渠道流水号、平台内部备注与打款单号；
// 审核人姓名不下发——用户只需要知道「审核通过/已打款」，不需要知道具体是谁处理的）。
type customerWithdraw struct {
	ID          uint64  `json:"id"`
	WithdrawNo  string  `json:"withdraw_no"`
	Amount      float64 `json:"amount"`
	Channel     string  `json:"channel"`
	ChannelName string  `json:"channel_name"`
	Account     string  `json:"account"`
	AccountName string  `json:"account_name"`
	BankName    string  `json:"bank_name"`
	Status      string  `json:"status"`
	PayoutNo    string  `json:"payout_no"`
	PayoutMode  string  `json:"payout_mode"`
	AuditedAt   string  `json:"audited_at"`
	PaidAt      string  `json:"paid_at"`
	CreatedAt   string  `json:"created_at"`
}

func customerWithdrawFrom(in withdrawdto.WithdrawInfo) customerWithdraw {
	return customerWithdraw{
		ID:          in.ID,
		WithdrawNo:  in.WithdrawNo,
		Amount:      in.Amount,
		Channel:     in.Channel,
		ChannelName: in.ChannelName,
		Account:     in.Account,
		AccountName: in.AccountName,
		BankName:    in.BankName,
		Status:      in.Status,
		PayoutNo:    in.PayoutNo,
		PayoutMode:  in.PayoutMode,
		AuditedAt:   in.AuditedAt,
		PaidAt:      in.PaidAt,
		CreatedAt:   in.CreatedAt,
	}
}

type customerWithdrawList struct {
	Items []customerWithdraw   `json:"items"`
	Meta  withdrawdto.ListMeta `json:"meta"`
}

func customerWithdrawListFrom(in *withdrawdto.WithdrawListResponse) *customerWithdrawList {
	if in == nil {
		return nil
	}
	items := make([]customerWithdraw, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, customerWithdrawFrom(it))
	}
	return &customerWithdrawList{Items: items, Meta: in.Meta}
}
