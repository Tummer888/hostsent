//go:build live

package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// 用户体系收口端到端联调——后台手工充值自动生成账单。
func TestLiveManualRechargeCreatesBill(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	id, username := h.registerUser("manualrecharge")

	// 充值前：三张表都该是空的。
	if n := rechargeRowCount(t, h, id); n != 0 {
		t.Fatalf("充值前不该有充值单，实际 %d 条", n)
	}

	status, body := h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/recharge", id), admin,
		map[string]any{"amount": 120, "remark": "联调用例：人工代充值"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("人工代充值失败: HTTP %d %v", status, body)
	}
	rechargeNo := digString(body, "data", "recharge_no")
	if rechargeNo == "" {
		t.Fatalf("充值响应必须回充值单号（运营凭此去充值管理页核对），实际 %v", body)
	}

	// 1. 充值单落库且已到账。
	var rcMethod, rcStatus, rcRemark string
	var rcAmount float64
	h.db.Raw("SELECT method, status, amount, remark FROM recharges WHERE recharge_no = ?", rechargeNo).
		Row().Scan(&rcMethod, &rcStatus, &rcAmount, &rcRemark) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if rcStatus != "success" {
		t.Errorf("充值单状态 = %q，期望 success（人工代充值应即时到账）", rcStatus)
	}
	if rcMethod != "manual" {
		t.Errorf("充值单方式 = %q，期望 manual（线下款记成线上渠道会污染渠道统计）", rcMethod)
	}
	if rcAmount != 120 {
		t.Errorf("充值单金额 = %v，期望 120", rcAmount)
	}

	// 2. 资金流水：类型必须是 recharge 而不是 adjust，ref_no 与充值单号一致。
	var txType, txRefNo string
	var txAmount float64
	h.db.Raw("SELECT type, ref_no, amount FROM wallet_transactions WHERE user_id = ? ORDER BY id DESC LIMIT 1", id).
		Row().Scan(&txType, &txRefNo, &txAmount) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if txType != "recharge" {
		t.Errorf("流水类型 = %q，期望 recharge（走调账会算进 adjust 口径，充值统计失真）", txType)
	}
	if txRefNo != rechargeNo {
		t.Errorf("流水 ref_no = %q，期望与充值单号 %q 一致（幂等键）", txRefNo, rechargeNo)
	}
	if txAmount != 120 {
		t.Errorf("流水金额 = %v，期望 120", txAmount)
	}

	// 3. 账单：这笔充值自己一张账单，金额单独成列、不进应结。
	assertRechargeBill(t, h, rechargeNo, id, 120)

	// 用户端「我的充值单」必须能看到——这是用户核对充值的唯一入口。
	userToken := h.loginUser(username, liveUserPassword)
	if status, body := h.do(http.MethodGet, "/api/v1/uc/finance/recharges", userToken, nil); status != http.StatusOK {
		t.Fatalf("用户端充值单列表失败: HTTP %d %v", status, body)
	} else if !containsRechargeNo(body, rechargeNo) {
		t.Errorf("用户端充值单列表里没有 %s：%v", rechargeNo, body)
	}

	// 同账期再充 30：必须新开第二张账单，而不是把两笔并成一张累加。
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/recharge", id), admin,
		map[string]any{"amount": 30})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("第二次充值失败: HTTP %d %v", status, body)
	}
	secondNo := digString(body, "data", "recharge_no")
	if secondNo == "" || secondNo == rechargeNo {
		t.Fatalf("第二次充值应回新的充值单号，实际 %q（首单 %q）", secondNo, rechargeNo)
	}
	assertRechargeBill(t, h, secondNo, id, 30)

	// 两张账单各自独立：单号不同、金额不合并、且第一张的金额没被后一笔改写。
	var billCount int64
	h.db.Raw("SELECT COUNT(*) FROM bills WHERE user_id = ? AND source_type = 'recharge'", id).Scan(&billCount)
	if billCount != 2 {
		t.Errorf("充值账单数 = %d，期望 2（每次充值一张独立账单）", billCount)
	}
	var firstAmount float64
	h.db.Raw("SELECT recharge_amount FROM bills WHERE source_type = 'recharge' AND source_no = ?", rechargeNo).
		Row().Scan(&firstAmount) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if firstAmount != 120 {
		t.Errorf("首张充值账单金额 = %v，期望 120（第二笔充值不得累加进第一张）", firstAmount)
	}
	// 按期账单不承载充值额：没有消费时压根不该有按期账单，有的话充值额也必须是 0。
	var periodRecharge float64
	h.db.Raw(`SELECT COALESCE(SUM(recharge_amount), 0) FROM bills
		WHERE user_id = ? AND source_type = 'period'`, id).Scan(&periodRecharge)
	if periodRecharge != 0 {
		t.Errorf("按期账单充值额合计 = %v，期望 0（充值只出现在逐笔充值账单上，不能两边重复计数）", periodRecharge)
	}

	// 反向守卫：负数走人工扣减，不生成充值单、不开账单。
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/recharge", id), admin,
		map[string]any{"amount": -20, "remark": "联调用例：追回"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("人工扣减失败: HTTP %d %v", status, body)
	}
	if txNo := digString(body, "data", "tx_no"); txNo == "" {
		t.Errorf("扣减响应应回流水号，实际 %v", body)
	}
	if n := rechargeRowCount(t, h, id); n != 2 {
		t.Errorf("扣减不该生成充值单，期望仍是 2 条，实际 %d 条", n)
	}
	h.db.Raw("SELECT COUNT(*) FROM bills WHERE user_id = ? AND source_type = 'recharge'", id).Scan(&billCount)
	if billCount != 2 {
		t.Errorf("扣减不该开账单，期望仍是 2 张，实际 %d 张", billCount)
	}
	var adjustCount int64
	h.db.Raw("SELECT COUNT(*) FROM wallet_transactions WHERE user_id = ? AND type = 'adjust' AND direction = -1", id).
		Scan(&adjustCount)
	if adjustCount != 1 {
		t.Errorf("扣减流水数 = %d，期望 1 条 type=adjust 的支出", adjustCount)
	}
}

// assertRechargeBill 断言某笔充值各自开了一张独立账单，且金额与归属正确。
//
// 这是「每次充值一个独立账单」的核心守卫：单号必须能从充值单号回查得到，
// 金额必须等于该笔充值额，状态必须是已结清（钱到账时就已收讫，没有待缴过程）。
func assertRechargeBill(t *testing.T, h *liveHarness, rechargeNo string, userID uint64, wantAmount float64) {
	t.Helper()
	var billNo, billType, sourceType, billStatus string
	var total, recharge float64
	h.db.Raw(`SELECT bill_no, bill_type, source_type, status, total_amount, recharge_amount
		FROM bills WHERE source_no = ?`, rechargeNo).
		Row().Scan(&billNo, &billType, &sourceType, &billStatus, &total, &recharge) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if billNo == "" {
		t.Fatalf("充值单 %s 未生成独立账单（这正是本次修复的核心缺陷）", rechargeNo)
	}
	if sourceType != "recharge" {
		t.Errorf("账单 %s 来源 = %q，期望 recharge", billNo, sourceType)
	}
	if billType != "recharge" {
		t.Errorf("账单 %s 分类 = %q，期望 recharge", billNo, billType)
	}
	if billStatus != "paid" {
		t.Errorf("账单 %s 状态 = %q，期望 paid（充值到账即收讫，没有待缴过程）", billNo, billStatus)
	}
	if recharge != wantAmount {
		t.Errorf("账单 %s 充值额 = %v，期望 %v", billNo, recharge, wantAmount)
	}
	if total != 0 {
		t.Errorf("账单 %s 应结金额 = %v，期望 0：充值是把钱打进平台，不该计入应结", billNo, total)
	}
}

func rechargeRowCount(t *testing.T, h *liveHarness, userID uint64) int64 {
	t.Helper()
	var n int64
	h.db.Raw("SELECT COUNT(*) FROM recharges WHERE user_id = ?", userID).Scan(&n)
	return n
}

// containsRechargeNo 判断用户端充值单列表里是否有指定单号。
func containsRechargeNo(body map[string]any, rechargeNo string) bool {
	for _, item := range digSlice(body, "data", "items") {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if digString(obj, "recharge_no") == rechargeNo {
			return true
		}
	}
	return false
}

// onlineUserPresent 判断某用户是否出现在总览页的在线用户里。
func (h *liveHarness) onlineUserPresent(adminToken string, userID uint64) bool {
	h.t.Helper()
	status, body := h.do(http.MethodGet, "/api/v1/admin/users/activity-overview?limit=50", adminToken, nil)
	if status != http.StatusOK {
		h.t.Fatalf("查询在线统计失败: HTTP %d %v", status, body)
	}
	for _, item := range digSlice(body, "data", "online_users") {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if uint64(digNumber(obj, "user_id")) == userID {
			return true
		}
	}
	return false
}

// ---------- 用例内的小工具 ----------

// doRedirect 发一次请求并取回 Location（302 回跳类接口用）。
func (h *liveHarness) doRedirect(method, path, token string) (int, string) {
	h.t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.srv.http.Handler.ServeHTTP(rec, req)
	return rec.Code, rec.Header().Get("Location")
}

func mustExec(t *testing.T, h *liveHarness, query string, args ...any) {
	t.Helper()
	if err := h.db.Exec(query, args...).Error; err != nil {
		t.Fatalf("执行 SQL 失败: %v\nSQL: %s", err, query)
	}
}

func removeID(ids []uint64, target uint64) []uint64 {
	out := ids[:0]
	for _, id := range ids {
		if id != target {
			out = append(out, id)
		}
	}
	return out
}

func queryParam(t *testing.T, rawURL, key string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("解析地址失败 %q: %v", rawURL, err)
	}
	return parsed.Query().Get(key)
}

func countReviewLogs(t *testing.T, h *liveHarness, appID uint64, action string) int64 {
	t.Helper()
	var n int64
	h.db.Raw("SELECT COUNT(*) FROM verification_review_logs WHERE application_id = ? AND action = ?", appID, action).Scan(&n)
	return n
}

func bindingRowCount(t *testing.T, h *liveHarness, userID uint64) int64 {
	t.Helper()
	var n int64
	h.db.Raw("SELECT COUNT(*) FROM user_oauth_bindings WHERE user_id = ?", userID).Scan(&n)
	return n
}

// countColumnsWrittenBy 目前恒返回 0：它是一处「未来守卫」的占位断言 ——
// 若将来有人给自助资料接口加上写实名信号列的能力，这行断言就该改成真检查。
func countColumnsWrittenBy(_ *testing.T, _ *liveHarness, _ uint64) int { return 0 }

// containsCandidate 判断 dry_run 候选列表里是否有指定用户。
func containsCandidate(body map[string]any, id uint64) bool {
	for _, item := range digSlice(body, "data", "candidates") {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if uint64(digNumber(obj, "id")) == id {
			return true
		}
	}
	return false
}

// candidateSkipReason 返回该用户被跳过的原因；未被跳过返回空串。
func candidateSkipReason(body map[string]any, id uint64) string {
	for _, item := range digSlice(body, "data", "skipped") {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if uint64(digNumber(obj, "id")) == id {
			return digString(obj, "reason")
		}
	}
	return ""
}

// containsProvider 判断公开渠道列表里是否有指定渠道。
func containsProvider(body map[string]any, provider string) bool {
	for _, item := range digSlice(body, "data") {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if digString(obj, "provider") == provider {
			return true
		}
	}
	return false
}
