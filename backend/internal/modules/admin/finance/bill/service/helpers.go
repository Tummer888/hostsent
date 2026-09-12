package service

import (
	"fmt"
	"math/rand"
	"time"

	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	billmodel "hostsent/backend/internal/modules/admin/finance/bill/model"
	"hostsent/backend/internal/pkg/money"
)

// genBillNo 生成账单号，如 20260908BILL00001。
func genBillNo() string {
	return fmt.Sprintf("BILL%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

// periodRange 将账期（如 202608）解析为起止时间（当月 1 日 00:00:00 ~ 月末 23:59:59）。
func periodRange(period string) (*time.Time, *time.Time, error) {
	ts, err := time.ParseInLocation("200601", period, time.Local)
	if err != nil {
		return nil, nil, ErrStatusConflict
	}
	start := time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0).Add(-time.Second)
	return &start, &end, nil
}

// normalizePage/normalizePageSize 复用 repository 中的分页归一化逻辑。
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

// buildBillInfo 构建账单 DTO。
//
// net_amount = total_amount - refund_fee_amount：原路退回时渠道手续费不退，
// 账单金额（应结）只冲减本金，但平台收入统计基数还要再扣掉扣点（doc36 §3.2）。
func buildBillInfo(b billmodel.Bill) dto.BillInfo {
	return dto.BillInfo{
		ID:                  b.ID,
		BillNo:              b.BillNo,
		UserID:              b.UserID,
		Period:              b.Period,
		TotalAmount:         b.TotalAmount,
		RefundAmount:        b.RefundAmount,
		Status:              b.Status,
		BillType:            b.BillType,
		ConsumeAmount:       b.ConsumeAmount,
		RenewalAmount:       b.RenewalAmount,
		ChannelRefundAmount: b.ChannelRefundAmount,
		RefundFeeAmount:     b.RefundFeeAmount,
		NetAmount:           money.Round2(b.TotalAmount - b.RefundFeeAmount),
		PaidAmount:          float64(b.PaidAmountFen) / 100,
		PaidMethod:          b.PaidMethod,
		PaidChannelID:       b.PaidChannelID,
		PaidAt:              formatTimePtr(b.PaidAt),
		InvoiceStatus:       defaultInvoiceStatus(b.InvoiceStatus),
		InvoiceNo:           b.InvoiceNo,
		InvoicedAt:          formatTimePtr(b.InvoicedAt),
		CreatedAt:           b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           b.UpdatedAt.Format(time.RFC3339),
	}
}

// buildInvoiceInfo 构建发票申请 DTO。
func buildInvoiceInfo(row billmodel.InvoiceRow) dto.InvoiceInfo {
	return dto.InvoiceInfo{
		ID:           row.ID,
		RequestNo:    row.RequestNo,
		BillID:       row.BillID,
		BillNo:       row.BillNo,
		UserID:       row.UserID,
		Username:     row.Username,
		InvoiceType:  row.InvoiceType,
		Title:        row.Title,
		TaxNo:        row.TaxNo,
		Amount:       row.Amount,
		Email:        row.Email,
		Status:       row.Status,
		Channel:      row.Channel,
		ExternalNo:   row.ExternalNo,
		FileURL:      row.FileURL,
		RejectReason: row.RejectReason,
		OperatorID:   row.OperatorID,
		IssuedAt:     formatTimePtr(row.IssuedAt),
		CreatedAt:    row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    row.UpdatedAt.Format(time.RFC3339),
	}
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// defaultInvoiceStatus 兼容旧行：模型默认 none，列为空时按未开票处理。
func defaultInvoiceStatus(s string) string {
	if s == "" {
		return billmodel.InvoiceStatusNone
	}
	return s
}

// genInvoiceNo 生成发票申请单号，如 INV202609121200001234。
func genInvoiceNo() string {
	return fmt.Sprintf("INV%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}
