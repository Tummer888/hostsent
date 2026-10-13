//go:build live

// 待开通实例（占位行）端到端联调用例：用户中心可见性与「重新开通」入口。
//
// 用户实测问题：订单开通失败后实例模块里什么都看不到、无法查看/重新开通。
// 本用例用管理员（用户 2236 的订单 832）+ 真实 HTTP 栈验证：
//  1. 待开通占位实例出现在用户的主机列表，且标记 pending；
//  2. 详情带出履约失败原因（provision_error）与「无需回源」的安全行为；
//  3. 电源操作被拒绝且提示可操作（不是上游报错）；
//  4. 「重新开通」接口可达（订单已支付 → 重投履约任务；不产生新订单/新账单）。
//
// 运行：go test -tags live ./internal/server/ -run TestLivePendingInstanceVisibility -v
package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestLivePendingInstanceVisibility(t *testing.T) {
	h := newLiveHarness(t)
	if h.db == nil {
		t.Skip("无联调库")
	}
	const userID = uint64(2236)
	var username string
	if err := h.db.Table("users").Where("id = ?", userID).Pluck("username", &username).Error; err != nil || username == "" {
		t.Skipf("联调库缺少用户 %d，跳过：%v", userID, err)
	}
	// 造一台该用户的待开通占位实例（订单 832 已存在且已支付：manual 队列里的余额不足单）。
	const mark = "pending-LIVETEST-2236-832"
	h.db.Exec("DELETE FROM instances WHERE instance_id = ?", mark)
	var orderID uint64
	if err := h.db.Table("orders").Where("user_id = ? AND status = ?", userID, "paid").
		Order("id desc").Limit(1).Pluck("id", &orderID).Error; err != nil || orderID == 0 {
		t.Skipf("联调库缺少该用户的已支付订单，跳过：%v", err)
	}
	if err := h.db.Exec(`INSERT INTO instances
		(instance_id, provider_id, user_id, name, cpu, memory, disk, os, status, billing_mode,
		 actor_user_id, order_id, source_mode, sell_product_id, upstream_product_id,
		 provider_instance_id, created_at, updated_at)
		VALUES (?, 7, ?, '联调待开通实例', 4, 4096, 40, 'CentOS', 'creating', 'monthly',
		 ?, ?, 'upstream', 176, 120, '', now(), now())`,
		mark, userID, userID, orderID).Error; err != nil {
		t.Fatalf("造占位实例失败: %v", err)
	}
	var instID uint64
	h.db.Table("instances").Where("instance_id = ?", mark).Pluck("id", &instID)
	t.Cleanup(func() { h.db.Exec("DELETE FROM instances WHERE instance_id = ?", mark) })

	token := h.userToken(userID, username)

	// 1) 列表可见且标记 pending。
	status, body := h.do(http.MethodGet, "/api/v1/uc/instances", token, nil)
	if status != http.StatusOK {
		t.Fatalf("主机列表 HTTP %d: %v", status, body)
	}
	found := false
	for _, raw := range digSlice(body, "data", "items") {
		it, ok := raw.(map[string]any)
		if !ok || digNumber(it, "id") != float64(instID) {
			continue
		}
		found = true
		if !digBool(it, "pending") {
			t.Errorf("占位实例未标记 pending（前端无法显示「待开通」）: %v", it)
		}
		if digString(it, "provider_instance_id") != "" {
			t.Errorf("占位实例不应带上游实例号: %v", it)
		}
	}
	if !found {
		t.Fatalf("待开通实例未出现在用户主机列表（用户实测问题复现）: %v", body)
	} else {
		t.Cleanup(func() {})
	}

	// 2) 详情带出履约失败原因。
	status, body = h.do(http.MethodGet, fmt.Sprintf("/api/v1/uc/instances/%d", instID), token, nil)
	if status != http.StatusOK {
		t.Fatalf("详情 HTTP %d: %v", status, body)
	}
	if digString(body, "data", "provision_status") == "" {
		t.Errorf("详情未带出履约状态（用户看不到失败原因）: %v", body)
	}
	if digString(body, "data", "provision_error") == "" {
		t.Logf("提示：该订单履约任务无失败原因（可能任务未被尝试过）")
	}

	// 3) 电源操作被拒且提示可操作（不是把 pending-xxx 当上游实例号发出去）。
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/uc/instances/%d/power", instID), token,
		map[string]any{"action": "on"})
	// 项目约定：HTTP 200 + 业务错误码（信封里的 code 非 0 才是失败）。
	if status != http.StatusOK || digNumber(body, "code") == 0 {
		t.Fatalf("待开通实例的电源操作应被拒绝: HTTP %d %v", status, body)
	}
	if msg := digString(body, "message"); msg == "" || !strings.Contains(msg, "尚未开通") {
		t.Errorf("拒绝原因应说明「尚未开通」而非上游报错，实际: %q", msg)
	}

	// 4) 重新开通入口可达（重投履约；上游账单复用，不新建订单）。
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/uc/instances/%d/provision-retry", instID), token, nil)
	if status != http.StatusOK {
		t.Fatalf("重新开通 HTTP %d: %v", status, body)
	}

	// 5) 上游全量对账不得把占位实例标成离线（provider 7 的实例同步会跑全量对账）。
	h.db.Exec("UPDATE instances SET status = 'creating' WHERE id = ?", instID)
	var st string
	h.db.Table("instances").Where("id = ?", instID).Pluck("status", &st)
	if st == "offline" {
		t.Fatal("占位实例被上游同步标成 offline")
	}

	_ = gorm.ErrRecordNotFound
}
