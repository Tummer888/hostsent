package mofangfinance

import (
	"context"
	"os"
	"testing"
	"time"

	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// liveZjmfProvider 返回真机魔方财务（haikayun）配置，用于确认财务型下游下单开通链路。
// 该测试会真实向上游下单并通过 apply_credit 扣减上游账户余额（创建真实主机），
// 必须显式设置 LIVE_ZJMF=1 才会执行，避免误触发扣费。
func liveZjmfProvider(t *testing.T) *MoFangFinanceProvider {
	if os.Getenv("LIVE_ZJMF") == "" {
		t.Skip("set LIVE_ZJMF=1 to run live mofangfinance provisioning (spends upstream balance)")
	}
	return NewMoFangFinanceProvider(&upstream.ProviderConfig{
		Type:         "mofangfinance",
		APIEndpoint:  "https://www.haikayun.com/",
		APIKey:       os.Getenv("ZJMF_USER"),
		APISecret:    os.Getenv("ZJMF_PASS"),
		UpstreamType: "zjmf_api",
		Secure:       true,
		Timeout:      40,
	})
}

// TestCreateInstanceLive 端到端验证财务型上游开通：fetch 商品配置 → 下单 → 结算 → 余额支付 → 取回主机。
func TestCreateInstanceLive(t *testing.T) {
	p := liveZjmfProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1) 获取上游商品配置（含 config_groups 的 upstream_id）
	prod, err := p.GetProduct(ctx, "222")
	if err != nil {
		t.Fatalf("GetProduct failed: %v", err)
	}
	raw, _ := prod.RawSpecs["config_groups"]

	// 2) 构造开通请求（等价 BuildProvisionRequest 产出的 Extra）
	extra := map[string]interface{}{
		"upstream_pid":  prod.UpstreamID,
		"config_groups": raw,
		"os":            prod.Specs.OS,
	}
	if prod.Specs.CPU > 0 {
		extra["cpu"] = prod.Specs.CPU
	}
	if prod.Specs.Memory > 0 {
		extra["memory"] = prod.Specs.Memory
	}
	req := &model.CreateInstanceRequest{
		ProviderType: ProviderType,
		Name:         "lite-live-" + randStr(6),
		BillingMode:  "monthly",
		Extra:        extra,
	}

	// 3) 开通并断言返回上游主机 ID
	inst, err := p.CreateInstance(ctx, req)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	if inst.UpstreamID == "" {
		t.Fatalf("CreateInstance returned empty UpstreamID")
	}
	t.Logf("provisioned upstream host_id=%s name=%s public_ip=%s status=%s", inst.UpstreamID, inst.Name, inst.PublicIP, inst.Status)

	// 4) 幂等复查：再次拉取详情应能取到同一主机
	got, err := p.GetInstance(ctx, inst.UpstreamID)
	if err != nil {
		t.Fatalf("GetInstance after create failed: %v", err)
	}
	t.Logf("re-fetch host_id=%s domainstatus-aware status=%s", got.UpstreamID, got.Status)
}
