package manual

import (
	"context"
	"errors"
	"testing"

	"hostsent/backend/internal/pkg/realname"
)

// 人工审核是「未接入任何三方核验」时的合法兜底路径，因此它必须：
//  1. 已注册描述符与工厂（否则后台看不到这个选项）；
//  2. 标记为 Builtin（不可删除、不可停用）；
//  3. DirectVerify 明确返回未实现 —— 服务层据此把申请放进人工队列，
//     而不是把「未接入」误判成「核验失败」并自动驳回用户。
func TestManualProvider_RegisteredAndBuiltin(t *testing.T) {
	d, ok := realname.Descriptor(Type)
	if !ok {
		t.Fatal("manual 描述符必须已注册")
	}
	if !d.Builtin {
		t.Fatal("manual 必须标记为内置兜底")
	}
	if d.Mode != "manual" {
		t.Fatalf("manual 的交互形态应为 manual，实际 %s", d.Mode)
	}
	if !realname.IsRegistered(Type) {
		t.Fatal("manual 工厂必须已注册")
	}
	// Implemented 是运行时按工厂注册情况算出来的，只认 AllDescriptors 的结果。
	implemented := false
	for _, item := range realname.AllDescriptors() {
		if item.Type == Type {
			implemented = item.Implemented
		}
	}
	if !implemented {
		t.Fatal("manual 必须被判定为已实现（它是真实可用的业务路径）")
	}
}

func TestManualProvider_DirectVerifyNotImplemented(t *testing.T) {
	p, err := realname.New(Type, realname.ProviderConfig{Type: Type})
	if err != nil {
		t.Fatalf("构造 manual provider 失败: %v", err)
	}
	if p.Type() != Type {
		t.Fatalf("Type 应为 %s", Type)
	}
	_, err = p.DirectVerify(context.Background(), realname.ProviderConfig{Type: Type}, realname.VerifyRequest{})
	if err == nil {
		t.Fatal("人工审核没有自动核验能力，必须返回未实现")
	}
	if !errors.Is(err, realname.ErrAdapterNotImplemented) {
		t.Fatalf("错误应为 ErrAdapterNotImplemented，实际: %v", err)
	}
	if realname.IsInitializer(p) {
		t.Fatal("人工审核不需要用户跳转，不应被判定为 Initializer")
	}
}

// 凭证校验走描述符：review_hint 是选填，因此空凭证也必须通过
// （内置兜底不能因为「没填提示语」就自判不可用）。
func TestManualProvider_TestWithEmptyCredentials(t *testing.T) {
	p, err := realname.New(Type, realname.ProviderConfig{Type: Type})
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if err := p.Test(context.Background(), realname.ProviderConfig{Type: Type}); err != nil {
		t.Fatalf("空凭证下人工审核仍应可用: %v", err)
	}
}
