// Package dto 定义分销模块对外传输的数据结构。
package dto

import "time"

const (
	SettlementStatusDraft     = "draft"
	SettlementStatusConfirmed = "confirmed"
	SettlementStatusPaid      = "paid"
	SettlementStatusCancelled = "cancelled"
)

var SettlementStatuses = map[string]struct{}{
	SettlementStatusDraft:     {},
	SettlementStatusConfirmed: {},
	SettlementStatusPaid:      {},
	SettlementStatusCancelled: {},
}

// SettlementListQuery 定义结算单列表的筛选条件。
type SettlementListQuery struct {
	Page      int       `form:"page"`
	PageSize  int       `form:"page_size"`
	AgentID   uint64    `form:"agent_id"`
	Status    string    `form:"status"`
	StartDate time.Time `form:"start_date" time_format:"2006-01-02"`
	EndDate   time.Time `form:"end_date" time_format:"2006-01-02"`
	Keyword   string    `form:"keyword"`
}

type SettlementCreateRequest struct {
	AgentID         uint64    `json:"agent_id" binding:"required"`
	SettlementNo    string    `json:"settlement_no"`
	PeriodStart     time.Time `json:"period_start" binding:"required"`
	PeriodEnd       time.Time `json:"period_end" binding:"required"`
	DeductionTotal  float64   `json:"deduction_total"`
	Remark          string    `json:"remark"`
	CommissionIDs   []uint64  `json:"commission_ids"`
}

type SettlementUpdateRequest struct {
	PeriodStart    time.Time `json:"period_start" binding:"required"`
	PeriodEnd      time.Time `json:"period_end" binding:"required"`
	DeductionTotal float64   `json:"deduction_total"`
	Remark         string    `json:"remark"`
}

type SettlementStatusChangeRequest struct {
	ConfirmedBy *uint64 `json:"confirmed_by"`
	Remark      string  `json:"remark"`
}

type SettlementInfo struct {
	ID               uint64     `json:"id"`
	AgentID          uint64     `json:"agent_id"`
	AgentName        string     `json:"agent_name"`
	SettlementNo     string     `json:"settlement_no"`
	PeriodStart      time.Time  `json:"period_start"`
	PeriodEnd        time.Time  `json:"period_end"`
	CommissionTotal  float64    `json:"commission_total"`
	DeductionTotal   float64    `json:"deduction_total"`
	PayableTotal     float64    `json:"payable_total"`
	CommissionCount  int        `json:"commission_count"`
	Status           string     `json:"status"`
	ConfirmedBy      *uint64    `json:"confirmed_by"`
	ConfirmedByName  string     `json:"confirmed_by_name"`
	ConfirmedAt      *time.Time `json:"confirmed_at"`
	PaidAt           *time.Time `json:"paid_at"`
	Remark           string     `json:"remark"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// SettlementListMeta 描述结算单列表分页信息。
type SettlementListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// SettlementListResponse 描述结算单列表的返回结构。
type SettlementListResponse struct {
	Items []SettlementInfo   `json:"items"`
	Meta  SettlementListMeta `json:"meta"`
}
