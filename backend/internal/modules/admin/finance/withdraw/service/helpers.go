package service

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/finance/withdraw/dto"
	"hostsent/backend/internal/modules/admin/finance/withdraw/model"
)

// genWithdrawNo 生成提现单号，如 WD20260912153000123456。
func genWithdrawNo() string {
	return fmt.Sprintf("WD%s%06d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}

// maskTail 收款账号脱敏：保留末 4 位。
func maskTail(account string) string {
	account = strings.TrimSpace(account)
	if len(account) <= 4 {
		return "****"
	}
	return "****" + account[len(account)-4:]
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

// buildWithdrawInfo 构建提现单 DTO。
func buildWithdrawInfo(w model.Withdraw) dto.WithdrawInfo {
	return dto.WithdrawInfo{
		ID:          w.ID,
		WithdrawNo:  w.WithdrawNo,
		UserID:      w.UserID,
		Amount:      w.Amount,
		Channel:     w.Channel,
		ChannelName: channelName(w.Channel),
		Account:     w.Account,
		AccountName: w.AccountName,
		BankName:    w.BankName,
		Status:      w.Status,
		PayoutNo:    w.PayoutNo,
		PayoutMode:  w.PayoutMode,
		ChannelTx:   w.ChannelTx,
		AuditBy:     w.AuditBy,
		AuditByName: w.AuditByName,
		AuditedAt:   formatTime(w.AuditedAt),
		PaidAt:      formatTime(w.PaidAt),
		Remark:      w.Remark,
		CreatedAt:   w.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   w.UpdatedAt.Format(time.RFC3339),
	}
}

func channelName(channel string) string {
	switch channel {
	case "bank":
		return "银行卡"
	case "alipay":
		return "支付宝"
	default:
		return channel
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
