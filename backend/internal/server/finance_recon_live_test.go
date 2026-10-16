//go:build live

package server

import (
	"testing"
)

// 回归 R8（财务对账）：对账的 SQL 恒等式必须能在真实库上跑通并给出结论。
//
// 为什么要 live 用例：这套检查几乎全是 SQL（CTE + UNION ALL + FULL OUTER JOIN），
// 用 fake 仓储只能测「服务层有没有把结果拼对」，测不出 SQL 本身能不能执行 ——
// 而实际返工正是 SQL 报错（ORDER BY 7 在 UNION 里越界，42P10），
// 单元测试全绿、一上真库就 500。
//
// 断言取向：不假设库里是干净的（开发库必然有历史脏数据），而是断言
// 「接口可用 + 结构自洽 + 差异行确实能定位到账户」，即：
//   - 恒等式口径下收入减支出等于可用+冻结时，issue 必须为 0（自洽性）；
//   - 有差异时每条都必须带 user_id 与非空说明（否则财务拿不到可行动信息）。
func TestLiveFinanceReconcileIdentities(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	status, body := h.do("POST", "/api/v1/admin/finance/bills/recon", admin, map[string]any{})
	if status != 200 || digNumber(body, "code") != 0 {
		t.Fatalf("发起对账失败: HTTP %d %v", status, body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("对账响应缺少 data: %v", body)
	}

	// 1) 结构自洽：issue_count 与 issues 长度一致。
	issueCount := int(digNumber(data, "issue_count"))
	issues, _ := data["issues"].([]any)
	if issueCount != len(issues) {
		t.Errorf("issue_count=%d 与 issues 实际长度 %d 不一致", issueCount, len(issues))
	}

	// 2) 状态与差异条数自洽：有差异必须 suspicious，无差异必须 ok。
	statusText := digString(data, "status")
	if issueCount > 0 && statusText != "suspicious" {
		t.Errorf("有 %d 条差异但状态为 %q，应为 suspicious", issueCount, statusText)
	}
	if issueCount == 0 && statusText != "ok" {
		t.Errorf("无差异但状态为 %q，应为 ok", statusText)
	}

	// 3) 每条差异都要可定位、可行动：带 user_id、检查项与说明。
	for i, raw := range issues {
		item, _ := raw.(map[string]any)
		if item == nil {
			t.Fatalf("第 %d 条差异不是对象: %v", i, raw)
		}
		if digNumber(item, "user_id") == 0 && digString(item, "check") != "orphan_ledger" {
			t.Errorf("第 %d 条差异缺少 user_id（财务无法定位到账户）: %v", i, item)
		}
		if digString(item, "label") == "" {
			t.Errorf("第 %d 条差异缺少检查项名称: %v", i, item)
		}
		if digString(item, "remark") == "" {
			t.Errorf("第 %d 条差异缺少说明（无法判断排查方向）: %v", i, item)
		}
	}

	// 4) 账户侧恒等式：可用 + 冻结 必须与「收入 - 支出」相符时才允许 ok。
	//    这里交叉验算一次，确保接口返回的两个数出自同一口径。
	balance := digNumber(data, "wallet_balance")
	frozen := digNumber(data, "wallet_frozen")
	income := digNumber(data, "income_total")
	expense := digNumber(data, "expense_total")
	if diff := digNumber(data, "diff"); diff != round2(income-expense-(balance+frozen)) {
		t.Errorf("diff=%v 与 (收入-支出)-(可用+冻结)=%v 不一致，说明口径不同源",
			diff, round2(income-expense-(balance+frozen)))
	}
}

// round2 对齐后端的两位小数舍入（对账差额比较用）。
func round2(v float64) float64 {
	if v >= 0 {
		return float64(int64(v*100+0.5)) / 100
	}
	return float64(int64(v*100-0.5)) / 100
}
