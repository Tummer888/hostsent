// Package manual 实现线下人工支付渠道（S1 落地渠道）。
//
// 定位：多渠道接入的兜底与起点。它不具备接口收款能力，走「用户提交支付单 →
// 线下转账 → 后台确认到账」的人工闭环。接入真实渠道（支付宝/微信）后本渠道
// 仍保留，用于大额对公转账等场景。
//
// 能力（CapabilityDescriptor）：仅 collect（返回操作指引）+ query（人工确认后置为 paid）。
// 不声明 refund / payout：退款与打款由财务流程人工登记，本渠道不做接口动作。
package manual

import (
	"context"
	"fmt"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/payment"
)

// Type 渠道类型标识（落 payment_types.type 与 payment_channels.type）。
const Type = "manual"

func init() {
	payment.RegisterDescriptor(Type, payment.CapabilityDescriptor{
		Mode:       payment.ModeManual,
		Scenes:     []string{payment.SceneScan, payment.SceneH5, payment.SceneNative},
		Operations: []string{payment.OpCollect, payment.OpQuery},
		CredentialSchema: []integration.Field{
			{Key: "payee_name", Label: "收款户名", Type: integration.FieldTypeString, Required: true,
				Placeholder: "公司/个人收款户名"},
			{Key: "payee_account", Label: "收款账号", Type: integration.FieldTypeString, Required: true,
				Placeholder: "银行卡号 / 支付宝账号"},
			{Key: "payee_bank", Label: "开户行", Type: integration.FieldTypeString, Required: false,
				Placeholder: "XX银行XX支行"},
			{Key: "instruction", Label: "转账备注要求", Type: integration.FieldTypeTextarea, Required: false,
				Placeholder: "请填写订单号作为转账附言，便于对账", Default: "请在转账附言中填写支付单号"},
		},
		SignerType:    payment.SignerNone,
		CertMode:      payment.CertModeNone,
		Currency:      "CNY",
		SettleMode:    payment.SettleT0,
		SupportsQuery: true,
		Implemented:   true,
		DocURL:        "",
	})
	payment.RegisterFactory(Type, func(cfg *payment.ChannelConfig) payment.Gateway {
		return &gateway{cfg: cfg}
	})
}

type gateway struct {
	cfg *payment.ChannelConfig
}

func (g *gateway) GetType() string { return Type }

func (g *gateway) Capabilities() payment.CapabilityDescriptor {
	d, _ := payment.Descriptor(Type)
	d.Implemented = true
	return d
}

// HealthCheck 线下渠道无外部依赖，凭证齐备即视为健康。
func (g *gateway) HealthCheck(ctx context.Context) error {
	if g.cfg == nil || g.cfg.Credentials.Get("payee_account") == "" {
		return fmt.Errorf("%w: 线下渠道缺少收款账号", payment.ErrNotImplemented)
	}
	return nil
}

// Prepay 返回人工转账指引；不产生任何渠道侧动作。
func (g *gateway) Prepay(ctx context.Context, cfg *payment.ChannelConfig, req *payment.PrepayRequest) (*payment.PrepayResult, error) {
	c := cfg
	if c == nil {
		c = g.cfg
	}
	instruction := c.Credentials.Get("instruction")
	if instruction == "" {
		instruction = "请在转账附言中填写支付单号"
	}
	lines := fmt.Sprintf("请向以下账户转账 ¥%.2f：\n户名：%s\n账号：%s",
		float64(req.AmountFen)/100, c.Credentials.Get("payee_name"), c.Credentials.Get("payee_account"))
	if bank := c.Credentials.Get("payee_bank"); bank != "" {
		lines += "\n开户行：" + bank
	}
	lines += "\n" + instruction + "\n支付单号：" + req.OutTradeNo +
		"\n转账后请等待管理员确认到账（通常 1 个工作日内）。"
	return &payment.PrepayResult{
		Instructions: lines,
		Raw:          map[string]interface{}{"mode": payment.ModeManual, "out_trade_no": req.OutTradeNo},
	}, nil
}

// Query 线下渠道只有「后台确认到账」这一种状态推进方式，查询返回当前未支付态，
// 由管理端人工确认接口直接改写支付单，不走本方法。
func (g *gateway) Query(ctx context.Context, cfg *payment.ChannelConfig, req *payment.QueryRequest) (*payment.PaymentState, error) {
	return &payment.PaymentState{Status: "pending"}, nil
}
