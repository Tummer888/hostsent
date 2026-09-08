package service

import (
	"fmt"
	"math/rand"
	"time"

	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	billmodel "hostsent/backend/internal/modules/admin/finance/bill/model"
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
func buildBillInfo(b billmodel.Bill) dto.BillInfo {
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
