//go:build live

// 下单即建「待开通实例」端到端联调用例（零成本路径）：
// 后台代下单（pay_mode=create，不扣款不投递开通）→ 占位实例应立刻出现在该用户
// 的实例列表（待开通）→ 取消订单后占位实例被删除。
//
// 为什么不用余额下单：那会真的扣款并向上游下单结算（成本与真实副作用）。
// 本用例走"仅创建待支付"路径，链路（订单落库 → 建占位 → 取消 → 删占位）完全一致。
//
// 运行：go test -tags live ./internal/server/ -run TestLiveOrderCreatesPendingInstance -v
package server

import (
	"fmt"
	"net/http"
	"testing"
)

func TestLiveOrderCreatesPendingInstance(t *testing.T) {
	h := newLiveHarness(t)
	if h.db == nil {
		t.Skip("无联调库")
	}

	// 1) 找一个可开通的对接商品（能构建出开通请求 → 会建占位）。
	var productID uint64
	if err := h.db.Raw(`SELECT p.id FROM products p
		WHERE p.status = 1 AND p.source_mode = 'upstream' AND p.source_product_id > 0
		ORDER BY p.id LIMIT 1`).Scan(&productID).Error; err != nil || productID == 0 {
		t.Skipf("联调库缺少可开通的对接商品，跳过：%v", err)
	}
	// 2) 造一个测试用户（代下单目标；registerUser 自动登记清理）。
	userID, username := h.registerUser("inst")

	adminToken := h.loginAdmin()

	// 3) 后台代下单（仅创建待支付：不扣款、不投递开通任务）。
	status, body := h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/orders", userID), adminToken,
		map[string]any{"product_id": productID, "pay_mode": "create", "billing_cycle": "monthly"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("后台代下单失败: HTTP %d %v", status, body)
	}
	orderID := uint64(digNumber(body, "data", "id"))
	if orderID == 0 {
		t.Fatalf("代下单未返回订单 ID: %v", body)
	}

	// 4) 占位实例应已建立（订单落库即刻，不等支付/开通）。
	var placeholder struct {
		ID                 uint64
		InstanceID         string
		Status             string
		ProviderInstanceID string
		OrderID            uint64
		UserID             uint64
	}
	if err := h.db.Raw(`SELECT id, instance_id, status, provider_instance_id, order_id, user_id
		FROM instances WHERE order_id = ?`, orderID).Scan(&placeholder).Error; err != nil {
		t.Fatalf("查询占位实例失败: %v", err)
	}
	if placeholder.ID == 0 {
		t.Fatalf("下单后未建立「待开通」实例占位（用户实测问题：开通失败后实例模块什么都看不到）")
	}
	if placeholder.ProviderInstanceID != "" || placeholder.Status != "creating" {
		t.Errorf("占位实例口径错误: status=%q provider_instance_id=%q", placeholder.Status, placeholder.ProviderInstanceID)
	}
	if placeholder.UserID != userID {
		t.Errorf("占位实例归属错误: user_id=%d want %d", placeholder.UserID, userID)
	}

	// 5) 用户视角：主机列表可见且标记 pending（前端显示「待开通」）。
	token := h.userToken(userID, username)
	status, body = h.do(http.MethodGet, "/api/v1/uc/instances", token, nil)
	if status != http.StatusOK {
		t.Fatalf("主机列表 HTTP %d: %v", status, body)
	}
	seen := false
	for _, raw := range digSlice(body, "data", "items") {
		it, ok := raw.(map[string]any)
		if !ok || digNumber(it, "order_id") != float64(orderID) {
			continue
		}
		seen = true
		if !digBool(it, "pending") {
			t.Errorf("占位实例未标记 pending: %v", it)
		}
	}
	if !seen {
		t.Fatalf("下单后实例列表看不到待开通实例: %v", body)
	}

	// 6) 取消订单（后台）→ 占位实例随之删除（未支付订单作废不留幽灵机）。
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/orders/%d/cancel", orderID), adminToken, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("取消订单失败: HTTP %d %v", status, body)
	}
	var cnt int64
	h.db.Raw("SELECT count(*) FROM instances WHERE order_id = ?", orderID).Scan(&cnt)
	if cnt != 0 {
		t.Fatalf("订单作废后占位实例应被删除，实际仍有 %d 行", cnt)
	}
}
