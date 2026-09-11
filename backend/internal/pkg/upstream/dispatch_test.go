package upstream

import (
	"context"
	"errors"
	"testing"

	"hostsent/backend/internal/pkg/model"
)

// 本文件覆盖 T5.5「暂停/销毁按链路与能力分派」的分派矩阵：
// 语义等价能力优先、降级只在语义等价时发生、彻底缺失必须显式报错（不静默）。

// fakeBase 只实现 Provider 最小公共接口。
type fakeBase struct {
	typ string
}

func (f *fakeBase) GetType() string                   { return f.typ }
func (f *fakeBase) GetName() string                   { return f.typ }
func (f *fakeBase) HealthCheck(context.Context) error { return nil }
func (f *fakeBase) Capabilities() CapabilityDescriptor {
	return CapabilityDescriptor{Kind: f.typ}
}

// suspensionOnly 只有暂停/恢复能力（无电源控制）。
type suspensionOnly struct {
	fakeBase
	suspended   []string
	unsuspended []string
}

func (s *suspensionOnly) SuspendInstance(_ context.Context, id, reason string) error {
	s.suspended = append(s.suspended, id+"|"+reason)
	return nil
}
func (s *suspensionOnly) UnsuspendInstance(_ context.Context, id string) error {
	s.unsuspended = append(s.unsuspended, id)
	return nil
}

// controlOnly 只有电源能力（无暂停能力）→ 应降级为关机/开机。
type controlOnly struct {
	fakeBase
	stopped []string
	started []string
}

func (c *controlOnly) StartInstance(_ context.Context, id string) error {
	c.started = append(c.started, id)
	return nil
}
func (c *controlOnly) StopInstance(_ context.Context, id string, force bool) error {
	c.stopped = append(c.stopped, id)
	return nil
}
func (c *controlOnly) RestartInstance(context.Context, string) error { return nil }
func (c *controlOnly) GetInstance(context.Context, string) (*model.StandardInstance, error) {
	return nil, errors.New("unused")
}
func (c *controlOnly) VNC(context.Context, string) (VNCResult, error) {
	return VNCResult{}, errors.New("unused")
}

// terminationOnly 只有终止能力（无整表管理能力）。
type terminationOnly struct {
	fakeBase
	terminated []string
}

func (t *terminationOnly) TerminateInstance(_ context.Context, id, reason string) error {
	t.terminated = append(t.terminated, id+"|"+reason)
	return nil
}

// administrationOnly 只有管理能力（无终止能力）→ 终止降级为删除。
type administrationOnly struct {
	fakeBase
	deleted []string
}

func (a *administrationOnly) ListInstances(context.Context, map[string]string) ([]*model.StandardInstance, error) {
	return nil, nil
}
func (a *administrationOnly) DeleteInstance(_ context.Context, id string) error {
	a.deleted = append(a.deleted, id)
	return nil
}
func (a *administrationOnly) ResizeInstance(context.Context, string, *model.StandardProductSpec) error {
	return nil
}

// TestSuspendDispatchMatrix 暂停分派：平台暂停态优先，其次关机，都没有则显式报错。
func TestSuspendDispatchMatrix(t *testing.T) {
	ctx := context.Background()

	sus := &suspensionOnly{fakeBase: fakeBase{typ: "mofangfinance"}}
	if err := SuspendWithFallback(ctx, sus, "h-1", "到期"); err != nil {
		t.Fatalf("平台有暂停能力时应成功: %v", err)
	}
	if len(sus.suspended) != 1 || sus.suspended[0] != "h-1|到期" {
		t.Fatalf("应调用平台暂停接口并透传原因，实际 %v", sus.suspended)
	}

	ctrl := &controlOnly{fakeBase: fakeBase{typ: "mofangyun"}}
	if err := SuspendWithFallback(ctx, ctrl, "c-1", "到期"); err != nil {
		t.Fatalf("退化关机应成功: %v", err)
	}
	if len(ctrl.stopped) != 1 || ctrl.stopped[0] != "c-1" {
		t.Fatalf("应退化为关机，实际 stopped=%v", ctrl.stopped)
	}

	// 只有终止能力：无暂停也无电源 → 必须显式报错。
	term := &terminationOnly{fakeBase: fakeBase{typ: "aws"}}
	err := SuspendWithFallback(ctx, term, "i-1", "到期")
	if !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("缺能力应返回 ErrCapabilityMissing，实际 %v", err)
	}
	if err := SuspendWithFallback(ctx, nil, "i-1", "到期"); !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("nil provider 应返回 ErrCapabilityMissing，实际 %v", err)
	}
}

// TestUnsuspendDispatchMatrix 恢复分派：平台解除暂停优先，其次开机，都没有则显式报错。
func TestUnsuspendDispatchMatrix(t *testing.T) {
	ctx := context.Background()

	sus := &suspensionOnly{fakeBase: fakeBase{typ: "mofangfinance"}}
	if err := UnsuspendWithFallback(ctx, sus, "h-1"); err != nil {
		t.Fatalf("平台有恢复能力时应成功: %v", err)
	}
	if len(sus.unsuspended) != 1 || sus.unsuspended[0] != "h-1" {
		t.Fatalf("应调用平台恢复接口，实际 %v", sus.unsuspended)
	}

	ctrl := &controlOnly{fakeBase: fakeBase{typ: "mofangyun"}}
	if err := UnsuspendWithFallback(ctx, ctrl, "c-1"); err != nil {
		t.Fatalf("退化开机应成功: %v", err)
	}
	if len(ctrl.started) != 1 {
		t.Fatalf("应退化为开机，实际 %v", ctrl.started)
	}

	if err := UnsuspendWithFallback(ctx, &terminationOnly{fakeBase: fakeBase{typ: "aws"}}, "i-1"); !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("缺恢复能力应显式报错，实际 %v", err)
	}
}

// TestTerminateDispatchMatrix 销毁分派：终止语义优先，其次删除，都没有则显式报错。
func TestTerminateDispatchMatrix(t *testing.T) {
	ctx := context.Background()

	term := &terminationOnly{fakeBase: fakeBase{typ: "mofangfinance"}}
	if err := TerminateWithFallback(ctx, term, "h-1", "到期销毁"); err != nil {
		t.Fatalf("平台有终止能力时应成功: %v", err)
	}
	if len(term.terminated) != 1 || term.terminated[0] != "h-1|到期销毁" {
		t.Fatalf("应调用平台终止接口，实际 %v", term.terminated)
	}

	admin := &administrationOnly{fakeBase: fakeBase{typ: "mofangyun"}}
	if err := TerminateWithFallback(ctx, admin, "c-1", "到期销毁"); err != nil {
		t.Fatalf("退化删除应成功: %v", err)
	}
	if len(admin.deleted) != 1 || admin.deleted[0] != "c-1" {
		t.Fatalf("应退化为删除，实际 %v", admin.deleted)
	}

	if err := TerminateWithFallback(ctx, &controlOnly{fakeBase: fakeBase{typ: "x"}}, "i-1", "r"); !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("缺销毁能力应显式报错，实际 %v", err)
	}
}

// TestRenewWithCapability 续费分派：未实现 InstanceRenewal 时必须返回带能力名的缺失错误。
func TestRenewWithCapability(t *testing.T) {
	ctx := context.Background()
	_, err := RenewWithCapability(ctx, &controlOnly{fakeBase: fakeBase{typ: "mofangyun"}}, &RenewRequest{ProviderInstanceID: "c-1"})
	if !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("缺续费能力应返回 ErrCapabilityMissing，实际 %v", err)
	}
	var mce *MissingCapabilityError
	if !errors.As(err, &mce) {
		t.Fatalf("应携带 MissingCapabilityError 以便展示原因，实际 %T", err)
	}
	if mce.Capability != OpRenew || mce.Provider != "mofangyun" {
		t.Fatalf("缺失错误内容异常: %+v", mce)
	}
	if _, err := RenewWithCapability(ctx, nil, &RenewRequest{}); !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("nil provider 应返回 ErrCapabilityMissing，实际 %v", err)
	}
}
