package pricing

import (
	"context"
	"testing"
)

func basePrice(unit float64, categoryID uint64) Deps {
	return Deps{BasePrice: func(context.Context, uint64) (float64, uint64, error) {
		return unit, categoryID, nil
	}}
}

func rateRule(source string, value float64) func(context.Context, uint64, uint64, uint64) (*Rule, error) {
	return func(context.Context, uint64, uint64, uint64) (*Rule, error) {
		return &Rule{Source: source, Code: source + "-code", Type: TypeRate, Value: value, PolicyID: 7}, nil
	}
}

func TestResolveNoRules(t *testing.T) {
	svc := NewService(basePrice(100, 1), StackModeBest)
	quote, err := svc.Resolve(context.Background(), ResolveInput{UserID: 1, ProductID: 2, Quantity: 3})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if quote.OriginalAmount != 300 || quote.DiscountAmount != 0 || quote.FinalAmount != 300 {
		t.Fatalf("无规则应原价成交，实际 %+v", quote)
	}
	if quote.Source != "" || quote.PolicyID != nil {
		t.Fatalf("无规则不应有来源，实际 source=%q policy=%v", quote.Source, quote.PolicyID)
	}
}

func TestResolveBestPicksLargestDiscount(t *testing.T) {
	deps := basePrice(100, 1)
	// 代理 9 折（省 10），用户组 8 折（省 20）→ best 取用户组。
	deps.AgentRule = rateRule(SourceAgent, 0.9)
	deps.GroupRule = rateRule(SourceGroup, 0.8)
	svc := NewService(deps, StackModeBest)

	quote, err := svc.Resolve(context.Background(), ResolveInput{UserID: 1, ProductID: 2, Quantity: 1})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if quote.FinalAmount != 80 {
		t.Fatalf("best 应取最优用户组价 80，实际 %v", quote.FinalAmount)
	}
	if quote.Source != SourceGroup {
		t.Fatalf("best 来源应为用户组，实际 %q", quote.Source)
	}
	if len(quote.Snapshot) != 2 {
		t.Fatalf("快照应保留全部命中规则，实际 %d 条", len(quote.Snapshot))
	}
}

func TestResolveStackAppliesSequentially(t *testing.T) {
	deps := basePrice(100, 1)
	deps.AgentRule = rateRule(SourceAgent, 0.9) // 100 → 90
	deps.GroupRule = rateRule(SourceGroup, 0.5) // 90  → 45
	svc := NewService(deps, StackModeStack)

	quote, err := svc.Resolve(context.Background(), ResolveInput{UserID: 1, ProductID: 2, Quantity: 1})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if quote.FinalAmount != 45 {
		t.Fatalf("stack 应顺序叠加为 45，实际 %v", quote.FinalAmount)
	}
	if quote.DiscountAmount != 55 {
		t.Fatalf("stack 优惠应为 55，实际 %v", quote.DiscountAmount)
	}
	if quote.Source != SourceGroup {
		t.Fatalf("stack 来源应取最后一条，实际 %q", quote.Source)
	}
}

func TestResolveManualOverridesPipeline(t *testing.T) {
	deps := basePrice(100, 1)
	deps.GroupRule = rateRule(SourceGroup, 0.5) // 有折扣也不该生效
	svc := NewService(deps, StackModeBest)

	manual := 120.0
	quote, err := svc.Resolve(context.Background(), ResolveInput{
		UserID: 1, ProductID: 2, Quantity: 1, ManualAmount: &manual,
	})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if quote.FinalAmount != 120 {
		t.Fatalf("手动改价应直接作为实付 120，实际 %v", quote.FinalAmount)
	}
	if quote.Source != SourceManual {
		t.Fatalf("手动改价来源应为 manual，实际 %q", quote.Source)
	}
	if quote.OriginalAmount != 100 || quote.DiscountAmount != 0 {
		t.Fatalf("手动加价不应算出负优惠，实际 %+v", quote)
	}
}

func TestResolveClampsInvalidRateAndAmountFloor(t *testing.T) {
	deps := basePrice(100, 1)
	// 非法折扣率（>1）按不打折处理；直减超过原价时实付归零不为负。
	deps.AgentRule = rateRule(SourceAgent, 1.5)
	deps.GroupRule = func(context.Context, uint64, uint64, uint64) (*Rule, error) {
		return &Rule{Source: SourceGroup, Code: "amt", Type: TypeAmount, Value: 500}, nil
	}
	svc := NewService(deps, StackModeBest)

	quote, err := svc.Resolve(context.Background(), ResolveInput{UserID: 1, ProductID: 2, Quantity: 1})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if quote.FinalAmount != 0 {
		t.Fatalf("直减超过原价应实付为 0，实际 %v", quote.FinalAmount)
	}
	if quote.DiscountAmount != 100 {
		t.Fatalf("优惠不应超过原价，实际 %v", quote.DiscountAmount)
	}
}

func TestResolveQuantityDefaultsToOne(t *testing.T) {
	svc := NewService(basePrice(30, 1), StackModeBest)
	quote, err := svc.Resolve(context.Background(), ResolveInput{UserID: 1, ProductID: 2, Quantity: 0})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if quote.OriginalAmount != 30 {
		t.Fatalf("数量缺省应为 1，实际原价 %v", quote.OriginalAmount)
	}
}

// TestResolveSpecBasePriceOverridesProductBase SKU 定价覆盖商品级基础价，折扣照常叠加。
func TestResolveSpecBasePriceOverridesProductBase(t *testing.T) {
	deps := basePrice(100, 1)
	deps.SpecBasePrice = func(_ context.Context, in ResolveInput) (float64, uint64, bool, error) {
		if in.SpecCode == "big" {
			return 200, 0, true, nil
		}
		return 0, 0, false, nil
	}
	deps.GroupRule = rateRule(SourceGroup, 0.9)
	svc := NewService(deps, StackModeBest)

	quote, err := svc.Resolve(context.Background(), ResolveInput{UserID: 1, ProductID: 2, SpecCode: "big", Quantity: 1})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if quote.OriginalAmount != 200 {
		t.Fatalf("SKU 原价应为 200，实际 %v", quote.OriginalAmount)
	}
	if quote.FinalAmount != 180 {
		t.Fatalf("SKU 价叠加 9 折应实付 180，实际 %v", quote.FinalAmount)
	}
}

// TestResolveSpecBasePriceFallsBack 未定价 SKU / 未选规格回落商品级基础价。
func TestResolveSpecBasePriceFallsBack(t *testing.T) {
	deps := basePrice(100, 1)
	deps.SpecBasePrice = func(context.Context, ResolveInput) (float64, uint64, bool, error) {
		return 999, 0, false, nil
	}
	svc := NewService(deps, StackModeBest)
	for _, code := range []string{"", "unknown"} {
		quote, err := svc.Resolve(context.Background(), ResolveInput{UserID: 1, ProductID: 2, SpecCode: code, Quantity: 2})
		if err != nil {
			t.Fatalf("Resolve 失败: %v", err)
		}
		if quote.OriginalAmount != 200 {
			t.Fatalf("spec_code=%q 应回落商品基础价 200，实际 %v", code, quote.OriginalAmount)
		}
	}
}
