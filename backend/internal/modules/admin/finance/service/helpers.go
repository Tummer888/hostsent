package service

import (
	"fmt"
	"math/rand"
	"time"

	"hostsent/backend/internal/modules/admin/finance/dto"
	"hostsent/backend/internal/modules/admin/finance/model"
)

// —— 单号生成 ——

func genTxNo() string {
	return fmt.Sprintf("W%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

func genRechargeNo() string {
	return fmt.Sprintf("RC%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

func genWithdrawNo() string {
	return fmt.Sprintf("WD%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

func genBillNo() string {
	return fmt.Sprintf("BILL%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

// —— 账期 ——

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

// —— DTO 构建 ——

func buildWalletInfo(acc *model.WalletAccount) *dto.WalletInfo {
	return &dto.WalletInfo{
		UserID:       acc.UserID,
		Balance:      acc.Balance,
		Frozen:       acc.Frozen,
		TotalIncome:  acc.TotalIncome,
		TotalExpense: acc.TotalExpense,
		Version:      acc.Version,
	}
}

func buildTransactionInfo(tx model.WalletTransaction) dto.TransactionInfo {
	return dto.TransactionInfo{
		ID:            tx.ID,
		TxNo:          tx.TxNo,
		UserID:        tx.UserID,
		Type:          tx.Type,
		Direction:     tx.Direction,
		Amount:        tx.Amount,
		BalanceBefore: tx.BalanceBefore,
		BalanceAfter:  tx.BalanceAfter,
		OrderID:       tx.OrderID,
		OrderNo:       tx.OrderNo,
		RefNo:         tx.RefNo,
		Remark:        tx.Remark,
		OperatorID:    tx.OperatorID,
		CreatedAt:     tx.CreatedAt.Format(time.RFC3339),
	}
}

func buildRechargeInfo(rc model.Recharge) dto.RechargeInfo {
	return dto.RechargeInfo{
		ID:         rc.ID,
		RechargeNo: rc.RechargeNo,
		UserID:     rc.UserID,
		Amount:     rc.Amount,
		Method:     rc.Method,
		Status:     rc.Status,
		ChannelTx:  rc.ChannelTx,
		PaidAt:     formatTime(rc.PaidAt),
		Remark:     rc.Remark,
		CreatedAt:  rc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  rc.UpdatedAt.Format(time.RFC3339),
	}
}

func buildWithdrawInfo(w model.Withdraw) dto.WithdrawInfo {
	return dto.WithdrawInfo{
		ID:          w.ID,
		WithdrawNo:  w.WithdrawNo,
		UserID:      w.UserID,
		Amount:      w.Amount,
		Channel:     w.Channel,
		Account:     w.Account,
		Status:      w.Status,
		AuditBy:     w.AuditBy,
		AuditByName: w.AuditByName,
		AuditedAt:   formatTime(w.AuditedAt),
		PaidAt:      formatTime(w.PaidAt),
		Remark:      w.Remark,
		CreatedAt:   w.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   w.UpdatedAt.Format(time.RFC3339),
	}
}

func buildBillInfo(b model.Bill) dto.BillInfo {
	return dto.BillInfo{
		ID:           b.ID,
		BillNo:       b.BillNo,
		UserID:       b.UserID,
		Period:       b.Period,
		TotalAmount:  b.TotalAmount,
		RefundAmount: b.RefundAmount,
		Status:       b.Status,
		CreatedAt:    b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    b.UpdatedAt.Format(time.RFC3339),
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
