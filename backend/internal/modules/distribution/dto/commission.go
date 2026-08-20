package dto

import "time"

const (
	CommissionStatusPending   = "pending"
	CommissionStatusFrozen    = "frozen"
	CommissionStatusAvailable = "available"
	CommissionStatusSettled   = "settled"
	CommissionStatusCancelled = "cancelled"
)

var CommissionStatuses = map[string]struct{}{
	CommissionStatusPending:   {},
	CommissionStatusFrozen:    {},
	CommissionStatusAvailable: {},
	CommissionStatusSettled:   {},
	CommissionStatusCancelled: {},
}

type CommissionListQuery struct {
	Page           int    `form:"page"`
	PageSize       int    `form:"page_size"`
	AgentID        uint64 `form:"agent_id"`
	Status         string `form:"status"`
	CommissionType string `form:"commission_type"`
	Keyword        string `form:"keyword"`
}

type CommissionCreateRequest struct {
	AgentID        uint64     `json:"agent_id" binding:"required"`
	SubordinateID  *uint64    `json:"subordinate_id"`
	SettlementID   *uint64    `json:"settlement_id"`
	OrderNo        string     `json:"order_no" binding:"required"`
	SourceType     string     `json:"source_type"`
	CommissionType string     `json:"commission_type"`
	BaseAmount     float64    `json:"base_amount"`
	Rate           float64    `json:"rate"`
	Amount         float64    `json:"amount"`
	Status         string     `json:"status"`
	FreezeUntil    *time.Time `json:"freeze_until"`
	SettledAt      *time.Time `json:"settled_at"`
	Remark         string     `json:"remark"`
}

type CommissionUpdateRequest struct {
	SubordinateID  *uint64    `json:"subordinate_id"`
	SettlementID   *uint64    `json:"settlement_id"`
	OrderNo        string     `json:"order_no" binding:"required"`
	SourceType     string     `json:"source_type" binding:"required"`
	CommissionType string     `json:"commission_type" binding:"required"`
	BaseAmount     float64    `json:"base_amount"`
	Rate           float64    `json:"rate"`
	Amount         float64    `json:"amount"`
	Status         string     `json:"status" binding:"required"`
	FreezeUntil    *time.Time `json:"freeze_until"`
	SettledAt      *time.Time `json:"settled_at"`
	Remark         string     `json:"remark"`
}

type CommissionStatusChangeRequest struct {
	FreezeUntil *time.Time `json:"freeze_until"`
	Remark      string     `json:"remark"`
}

type CommissionInfo struct {
	ID              uint64     `json:"id"`
	AgentID         uint64     `json:"agent_id"`
	AgentName       string     `json:"agent_name"`
	SubordinateID   *uint64    `json:"subordinate_id"`
	SubordinateName string     `json:"subordinate_name"`
	SettlementID    *uint64    `json:"settlement_id"`
	OrderNo         string     `json:"order_no"`
	SourceType      string     `json:"source_type"`
	CommissionType  string     `json:"commission_type"`
	BaseAmount      float64    `json:"base_amount"`
	Rate            float64    `json:"rate"`
	Amount          float64    `json:"amount"`
	Status          string     `json:"status"`
	FreezeUntil     *time.Time `json:"freeze_until"`
	SettledAt       *time.Time `json:"settled_at"`
	Remark          string     `json:"remark"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CommissionListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type CommissionListResponse struct {
	Items []CommissionInfo   `json:"items"`
	Meta  CommissionListMeta `json:"meta"`
}
