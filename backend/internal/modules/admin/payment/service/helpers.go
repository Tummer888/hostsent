package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
	"hostsent/backend/internal/pkg/credentials"
	"hostsent/backend/internal/pkg/crypto"
	"hostsent/backend/internal/pkg/payment"
)

// ---- 单号 ----

// genPaymentNo 生成支付单号，如 20260912P00001234。
func genPaymentNo() string {
	return fmt.Sprintf("P%s%06d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}

// genRefundNo 生成退款单号。
func genRefundNo() string {
	return fmt.Sprintf("PR%s%06d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}

// genPayoutNo 生成打款单号。
func genPayoutNo() string {
	return fmt.Sprintf("PO%s%06d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}

// genReconNo 生成对账记录号。
func genReconNo() string {
	return fmt.Sprintf("RC%s%06d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}

// ---- 分页 ----

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return 10
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ---- 渠道类型描述符 ----

// typeNames 渠道类型展示名（适配器未落库时的兜底）。
var typeNames = map[string]string{
	"manual":    "线下人工",
	"alipay":    "支付宝",
	"wechatpay": "微信支付",
	"unionpay":  "云闪付",
	"bestpay":   "翼支付",
}

// TypeName 返回渠道类型展示名。
func TypeName(t string) string {
	if name, ok := typeNames[t]; ok {
		return name
	}
	return t
}

// descriptorFor 取渠道类型能力描述符（注册表优先，落库兜底，最后保守默认）。
func (s *channelService) descriptorFor(providerType string) payment.CapabilityDescriptor {
	implemented := payment.IsRegistered(providerType)
	if d, ok := payment.Descriptor(providerType); ok {
		d.Implemented = true
		return d
	}
	if d, ok := s.loadStoredDescriptor(providerType); ok {
		d.Implemented = implemented
		return d
	}
	return payment.CapabilityDescriptor{Mode: payment.ModeAPI, Currency: "CNY", SignerType: payment.SignerNone, Implemented: implemented}
}

// loadStoredDescriptor 读取 payment_types 中落库的描述符。
func (s *channelService) loadStoredDescriptor(providerType string) (payment.CapabilityDescriptor, bool) {
	var d payment.CapabilityDescriptor
	if s.typeRepo == nil {
		return d, false
	}
	row, err := s.typeRepo.FindByType(context.Background(), providerType)
	if err != nil || strings.TrimSpace(row.DescriptorJSON) == "" {
		return d, false
	}
	if err := json.Unmarshal([]byte(row.DescriptorJSON), &d); err != nil {
		return d, false
	}
	return d, true
}

// encryptSecret 用配置密钥加密敏感值（收款账号等）。
func encryptSecret(plain, secretKey string) (string, error) {
	if plain == "" {
		return "", nil
	}
	enc, err := crypto.Encrypt(plain, secretKey)
	if err != nil {
		return "", err
	}
	return credentials.EncPrefix + enc, nil
}

// decryptSecret 解密敏感值；非密文原样返回（兼容历史明文）。
func decryptSecret(stored, secretKey string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, credentials.EncPrefix) {
		return stored, nil
	}
	plain, err := crypto.Decrypt(strings.TrimPrefix(stored, credentials.EncPrefix), secretKey)
	if err != nil {
		return "", credentials.ErrDecryptFailed
	}
	return plain, nil
}

// jsonMarshal 序列化渠道类型描述符（落库）。
func jsonMarshal(v interface{}) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ---- 渠道 DTO ----

// buildChannelInfo 组装渠道信息（凭证按描述符脱敏）。
func (s *channelService) buildChannelInfo(c model.PaymentChannel) dto.ChannelInfo {
	desc := s.descriptorFor(c.Type)
	masked := map[string]string{}
	if creds, err := credentials.Decode(c.Credentials); err == nil {
		masked = creds.Mask(desc.CredentialSchema)
	}
	scenes := decodeScenes(c.Scenes)
	if len(scenes) == 0 {
		scenes = desc.Scenes
	}
	return dto.ChannelInfo{
		ID:           c.ID,
		ChannelCode:  c.ChannelCode,
		Name:         c.Name,
		Type:         c.Type,
		TypeName:     TypeName(c.Type),
		Mode:         desc.Mode,
		Credentials:  masked,
		Endpoint:     c.Endpoint,
		NotifyURL:    c.NotifyURL,
		ReturnURL:    c.ReturnURL,
		Scenes:       scenes,
		Priority:     c.Priority,
		Weight:       c.Weight,
		FeeRate:      c.FeeRate,
		SettleMode:   c.SettleMode,
		MinAmountFen: c.MinAmountFen,
		MaxAmountFen: c.MaxAmountFen,
		Environment:  c.Environment,
		HealthStatus: c.HealthStatus,
		LastError:    c.LastError,
		LastCheckAt:  formatTime(c.LastCheckAt),
		Status:       c.Status,
		IsDefault:    c.IsDefault,
		Remark:       c.Remark,
		Capabilities: desc,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    c.UpdatedAt.Format(time.RFC3339),
	}
}

func decodeScenes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func encodeScenes(scenes []string) string {
	if len(scenes) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(scenes)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// ---- 支付单 DTO ----

func buildOrderInfo(o model.PaymentOrder, channelName string) dto.OrderInfo {
	return dto.OrderInfo{
		ID:           o.ID,
		PaymentNo:    o.PaymentNo,
		UserID:       o.UserID,
		BizType:      o.BizType,
		BizID:        o.BizID,
		BizNo:        o.BizNo,
		Amount:       fenToYuan(o.AmountFen),
		AmountFen:    o.AmountFen,
		Currency:     o.Currency,
		ChannelID:    o.ChannelID,
		ChannelCode:  o.ChannelCode,
		ChannelType:  o.ChannelType,
		ChannelName:  channelName,
		Scene:        o.Scene,
		Status:       o.Status,
		ChannelTx:    o.ChannelTx,
		PayURL:       o.PayURL,
		QRCode:       o.QRCode,
		PrepayParams: o.PrepayParams,
		Instructions: o.Instructions,
		Subject:      o.Subject,
		Fee:          fenToYuan(o.FeeFen),
		ExpireAt:     formatTime(o.ExpireAt),
		PaidAt:       formatTime(o.PaidAt),
		Remark:       o.Remark,
		CreatedAt:    o.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    o.UpdatedAt.Format(time.RFC3339),
	}
}

func buildCallbackInfo(l model.PaymentCallbackLog) dto.CallbackInfo {
	return dto.CallbackInfo{
		ID:           l.ID,
		ChannelID:    l.ChannelID,
		ChannelCode:  l.ChannelCode,
		NotifyID:     l.NotifyID,
		PaymentNo:    l.PaymentNo,
		RawBody:      l.RawBody,
		VerifyOK:     l.VerifyOK,
		Amount:       fenToYuan(l.AmountFen),
		HandleStatus: l.HandleStatus,
		HandleMsg:    l.HandleMsg,
		SourceIP:     l.SourceIP,
		CreatedAt:    l.CreatedAt.Format(time.RFC3339),
	}
}

func buildRefundInfo(r model.PaymentRefund) dto.RefundInfo {
	return dto.RefundInfo{
		ID:            r.ID,
		RefundNo:      r.RefundNo,
		PaymentNo:     r.PaymentNo,
		OrderRefundNo: r.OrderRefundNo,
		UserID:        r.UserID,
		Amount:        fenToYuan(r.AmountFen),
		Status:        r.Status,
		Reason:        r.Reason,
		FailReason:    r.FailReason,
		RefundedAt:    formatTime(r.RefundedAt),
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
	}
}

func buildPayoutInfo(p model.PaymentPayout) dto.PayoutInfo {
	return dto.PayoutInfo{
		ID:          p.ID,
		PayoutNo:    p.PayoutNo,
		WithdrawID:  p.WithdrawID,
		WithdrawNo:  p.WithdrawNo,
		UserID:      p.UserID,
		Amount:      fenToYuan(p.AmountFen),
		ChannelID:   p.ChannelID,
		ChannelCode: p.ChannelCode,
		Mode:        p.Mode,
		Status:      p.Status,
		ChannelTx:   p.ChannelTx,
		ReceiptURL:  p.ReceiptURL,
		FailReason:  p.FailReason,
		Attempts:    p.Attempts,
		OperatorID:  p.OperatorID,
		PaidAt:      formatTime(p.PaidAt),
		Remark:      p.Remark,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
}

// buildAccountInfo 组装收款账户 DTO：账号列落库为密文，须先解密再脱敏展示，
// 否则前端会看到 base64 乱码；解密失败时退化为固定掩码，绝不回显密文。
func buildAccountInfo(a model.UserPayoutAccount, secretKey string) dto.PayoutAccountInfo {
	return dto.PayoutAccountInfo{
		ID:          a.ID,
		Channel:     a.Channel,
		AccountNo:   maskAccountNo(a.AccountNo, secretKey),
		AccountName: a.AccountName,
		BankName:    a.BankName,
		Branch:      a.Branch,
		IsDefault:   a.IsDefault,
		Verified:    a.Verified,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
	}
}

func maskAccountNo(stored, secretKey string) string {
	if stored == "" {
		return ""
	}
	if !strings.HasPrefix(stored, credentials.EncPrefix) {
		return credentials.MaskSecret(stored)
	}
	plain, err := decryptSecret(stored, secretKey)
	if err != nil || plain == "" {
		return "****"
	}
	return credentials.MaskSecret(plain)
}

func buildReconRecordInfo(r model.PaymentReconRecord) dto.ReconRecordInfo {
	return dto.ReconRecordInfo{
		ID:            r.ID,
		ReconNo:       r.ReconNo,
		ChannelCode:   r.ChannelCode,
		Period:        r.Period,
		LocalAmount:   fenToYuan(r.LocalAmountFen),
		LocalCount:    r.LocalCount,
		ChannelAmount: fenToYuan(r.ChannelAmountFen),
		ChannelCount:  r.ChannelCount,
		Diff:          fenToYuan(r.DiffFen),
		Status:        r.Status,
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
	}
}

// ---- 金额 ----

func yuanToFen(yuan float64) int64 {
	return int64(yuan*100 + 0.5)
}

func fenToYuan(fen int64) float64 {
	return float64(fen) / 100
}
