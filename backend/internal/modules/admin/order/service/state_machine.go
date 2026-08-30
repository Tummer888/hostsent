// Package service 提供订单域的业务编排。
package service

import "hostsent/backend/internal/modules/admin/order/model"

// transitionTable 定义允许的订单状态迁移。
var transitionTable = map[string]map[string]bool{
	model.OrderStatusPending: {
		model.OrderStatusPaid:      true,
		model.OrderStatusCancelled: true,
	},
	model.OrderStatusPaid: {
		model.OrderStatusProvisioning: true,
		model.OrderStatusRefunded:     true,
	},
	model.OrderStatusProvisioning: {
		model.OrderStatusActive:   true,
		model.OrderStatusRefunded: true,
	},
	model.OrderStatusActive: {
		model.OrderStatusRefunding: true,
		model.OrderStatusCompleted: true,
		model.OrderStatusClosed:    true,
	},
	model.OrderStatusRefunding: {
		model.OrderStatusRefunded: true,
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
	case model.OrderStatusCancelled, model.OrderStatusRefunded, model.OrderStatusClosed, model.OrderStatusCompleted:
		return true
	}
	return false
}
