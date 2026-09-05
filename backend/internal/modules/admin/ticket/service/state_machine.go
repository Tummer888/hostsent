// Package service 提供工单域的业务编排。
package service

import "hostsent/backend/internal/modules/admin/ticket/model"

// transitionTable 定义允许的工单状态迁移。
//
//	open ──管理员回复──▶ in_progress ──▶ waiting_user ──用户回复──▶ in_progress
//	  │                      │
//	  │                      └──已解决──▶ resolved ──▶ closed / in_progress（追问重开）
//	  └──用户取消──▶ cancelled
var transitionTable = map[string]map[string]bool{
	model.TicketStatusOpen: {
		model.TicketStatusInProgress: true,
		model.TicketStatusCancelled:  true,
	},
	model.TicketStatusInProgress: {
		model.TicketStatusWaitingUser: true,
		model.TicketStatusResolved:    true,
		model.TicketStatusClosed:      true,
	},
	model.TicketStatusWaitingUser: {
		model.TicketStatusInProgress: true,
		model.TicketStatusResolved:   true,
		model.TicketStatusClosed:     true,
	},
	model.TicketStatusResolved: {
		model.TicketStatusClosed:     true,
		model.TicketStatusInProgress: true, // 用户追问重新打开
	},
}

// CanTransfer 校验状态迁移是否合法。
func CanTransfer(from, to string) bool {
	allowed, ok := transitionTable[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// EnsureStatus 校验状态迁移，非法返回 ErrStatusConflict。
func EnsureStatus(from, to string) error {
	if !CanTransfer(from, to) {
		return ErrStatusConflict
	}
	return nil
}

// IsFinalStatus 校验是否为终态（不可再流转）。
func IsFinalStatus(status string) bool {
	switch status {
	case model.TicketStatusClosed, model.TicketStatusCancelled:
		return true
	}
	return false
}

// IsValidStatus 校验是否为合法工单状态枚举。
func IsValidStatus(status string) bool {
	switch status {
	case model.TicketStatusOpen,
		model.TicketStatusInProgress,
		model.TicketStatusWaitingUser,
		model.TicketStatusResolved,
		model.TicketStatusClosed,
		model.TicketStatusCancelled:
		return true
	}
	return false
}
