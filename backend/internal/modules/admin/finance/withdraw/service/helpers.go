package service

import (
	"fmt"
	"math/rand"
	"time"

	"hostsent/backend/internal/modules/admin/finance/withdraw/dto"
	"hostsent/backend/internal/modules/admin/finance/withdraw/model"
)

// genWithdrawNo 生成提现单号，如 20260908WD00001。
func genWithdrawNo() string {
	return fmt.Sprintf("WD%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
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

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
