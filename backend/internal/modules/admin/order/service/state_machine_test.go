package service

import (
	"testing"

	"hostsent/backend/internal/modules/admin/order/model"
)

// TestOrderStateMachine 验证订单状态机：paid → provisioning → active 合法，
// 非法迁移（如 pending → active、active → paid）被拒绝。
func TestOrderStateMachine(t *testing.T) {
	// 合法路径
	legal := [][2]string{
		{model.OrderStatusPending, model.OrderStatusPaid},
		{model.OrderStatusPaid, model.OrderStatusProvisioning},
		{model.OrderStatusProvisioning, model.OrderStatusActive},
		{model.OrderStatusActive, model.OrderStatusRefunding},
		{model.OrderStatusRefunding, model.OrderStatusRefunded},
		{model.OrderStatusActive, model.OrderStatusCompleted},
		{model.OrderStatusPending, model.OrderStatusCancelled},
	}
	for _, tr := range legal {
		if err := EnsureStatus(tr[0], tr[1]); err != nil {
			t.Errorf("expected legal transition %s -> %s, got error: %v", tr[0], tr[1], err)
		}
		if !CanTransfer(tr[0], tr[1]) {
			t.Errorf("CanTransfer(%s,%s) = false, want true", tr[0], tr[1])
		}
	}

	// 非法迁移
	illegal := [][2]string{
		{model.OrderStatusPending, model.OrderStatusActive},
		{model.OrderStatusPending, model.OrderStatusProvisioning},
		{model.OrderStatusPaid, model.OrderStatusActive},
		{model.OrderStatusActive, model.OrderStatusPaid},
		{model.OrderStatusProvisioning, model.OrderStatusPaid},
		{model.OrderStatusRefunded, model.OrderStatusPaid},
	}
	for _, tr := range illegal {
		if err := EnsureStatus(tr[0], tr[1]); err == nil {
			t.Errorf("expected illegal transition %s -> %s to be rejected, got nil", tr[0], tr[1])
		}
		if CanTransfer(tr[0], tr[1]) {
			t.Errorf("CanTransfer(%s,%s) = true, want false", tr[0], tr[1])
		}
	}
}

// TestIsFinalStatus 验证终态判定。
func TestIsFinalStatus(t *testing.T) {
	final := []string{
		model.OrderStatusCancelled,
		model.OrderStatusRefunded,
		model.OrderStatusClosed,
		model.OrderStatusCompleted,
	}
	for _, s := range final {
		if !IsFinalStatus(s) {
			t.Errorf("IsFinalStatus(%q) = false, want true", s)
		}
	}
	if IsFinalStatus(model.OrderStatusActive) || IsFinalStatus(model.OrderStatusPending) {
		t.Errorf("IsFinalStatus should be false for non-final states")
	}
}
