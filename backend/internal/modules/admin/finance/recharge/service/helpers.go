package service

import (
	"fmt"
	"math/rand"
	"time"

	"hostsent/backend/internal/modules/admin/finance/recharge/dto"
	"hostsent/backend/internal/modules/admin/finance/recharge/model"
)

// genRechargeNo 生成充值单号，如 20260908RC00001。
func genRechargeNo() string {
	return fmt.Sprintf("RC%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
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

// buildRechargeInfo 构建充值单 DTO。
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

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
