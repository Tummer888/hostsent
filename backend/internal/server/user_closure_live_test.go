//go:build live

package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	ticketrepo "hostsent/backend/internal/modules/admin/ticket/repository"
	"time"
)

// 用户体系收口端到端联调——注销/恢复/实名/OAuth 绑定登录（doc104 §10.3）。
// TestLiveUserSoftDeleteAndRestore 覆盖 doc104 §10.3 第 1 行。
//
// 断言链：注销后 deleted_at 非空且 status=cancelled → 登录被拒（且提示是
// 「用户名或密码错误」而不是「账号已被禁用」，否则等于向攻击者确认该账号存在）
// → 同 username 可重新注册 → 恢复时若已被占用则明确失败 → 清理冲突行后恢复成功，
// status_before_delete 精确还原。
func TestLiveUserSoftDeleteAndRestore(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	id, username := h.registerUser("softdel")
	h.loginUser(username, liveUserPassword)

	// —— 注销 ——
	status, body := h.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d/deletion-check", id), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("注销前置校验失败: HTTP %d %v", status, body)
	}
	if canDelete, _ := digValue(body, "data", "can_delete").(bool); !canDelete {
		t.Fatalf("新注册用户应可注销，实际 can_delete=false: %v", body)
	}

	status, body = h.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", id), admin, map[string]any{
		"reason": "联调用例：注销", "force": true,
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("注销失败: HTTP %d %v", status, body)
	}

	var row struct {
		Status             string
		StatusBeforeDelete string
		DeletedAt          *time.Time
		DeletedBy          uint64
	}
	if err := h.db.Raw(
		`SELECT status, status_before_delete, deleted_at, deleted_by FROM users WHERE id = ?`, id,
	).Scan(&row).Error; err != nil {
		t.Fatalf("读取注销后用户行失败: %v", err)
	}
	if row.DeletedAt == nil {
		t.Error("注销后 deleted_at 仍为空")
	}
	if row.Status != "cancelled" {
		t.Errorf("注销后 status = %q，期望 cancelled", row.Status)
	}
	if row.StatusBeforeDelete != "active" {
		t.Errorf("status_before_delete = %q，期望 active", row.StatusBeforeDelete)
	}
	if row.DeletedBy == 0 {
		t.Error("deleted_by 为 0：注销留痕缺失（F5 回归）")
	}

	// —— R4（doc104 §10.5）：注销信息是 profile 的一部分，不得让详情页整体失败 ——
	// 只断言 service 层「按节降级」不够：新加的 deleted_by_name 是联表别名，
	// 写错 JOIN 别名会让整条聚合查询报错（那是 profile 段，会 500）。
	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/users/%d/detail-aggregate", id), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("已注销用户的详情聚合失败: HTTP %d %v", status, body)
	}
	if digValue(body, "data", "profile", "deleted_at") == nil {
		t.Error("R4：详情聚合未带出注销时间")
	}
	if reason := digString(body, "data", "profile", "delete_reason"); reason == "" {
		t.Error("R4：详情聚合未带出注销原因")
	}
	if name := digString(body, "data", "profile", "deleted_by_name"); name == "" {
		t.Error("R4：注销人显示名为空（deleted_by_name 联表别名未生效）")
	}
	if digValue(body, "data", "degraded") == nil {
		t.Error("R4：degraded 必须是数组（前端会 join），不能缺失或为 null")
	}

	// —— 注销用户不得再登录，且不能泄露「账号存在」——
	status, body = h.do(http.MethodPost, "/api/v1/uc/auth/login", "", map[string]any{
		"username": username, "password": liveUserPassword,
	})
	if message := digString(body, "message"); message != "用户名或密码错误" {
		t.Errorf("注销用户登录提示 = %q，期望「用户名或密码错误」（不得暴露账号存在）", message)
	}
	if status == http.StatusOK && digNumber(body, "code") == 0 {
		t.Error("注销用户仍能登录")
	}

	// —— 同 username 可重新注册（部分唯一索引只约束未注销行）——
	status, body = h.do(http.MethodPost, "/api/v1/uc/auth/register", "", map[string]any{
		"username": username,
		"password": liveUserPassword,
		"email":    username + "_reuse@example.com",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("注销后同名注册应成功，实际 HTTP %d %v", status, body)
	}
	reusedID := uint64(digNumber(body, "data", "id"))
	if reusedID == 0 || reusedID == id {
		t.Fatalf("同名重注册返回的 id 异常: %d", reusedID)
	}
	h.created = append(h.created, reusedID)

	// —— 恢复时必须明确失败（账号名已被占用）——
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/restore", id), admin, nil)
	if status != http.StatusConflict {
		t.Fatalf("占用冲突下恢复应返回 409，实际 HTTP %d %v", status, body)
	}
	if code := digNumber(body, "code"); code != 40904 {
		t.Errorf("冲突恢复的业务码 = %v，期望 40904", code)
	}

	// —— 清掉冲突行后恢复成功，状态精确还原 ——
	if err := h.db.Exec("DELETE FROM users WHERE id = ?", reusedID).Error; err != nil {
		t.Fatalf("清理同名冲突用户失败: %v", err)
	}
	h.created = removeID(h.created, reusedID)

	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/restore", id), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("恢复失败: HTTP %d %v", status, body)
	}
	if err := h.db.Raw(
		`SELECT status, status_before_delete, deleted_at, deleted_by FROM users WHERE id = ?`, id,
	).Scan(&row).Error; err != nil {
		t.Fatalf("读取恢复后用户行失败: %v", err)
	}
	if row.DeletedAt != nil {
		t.Error("恢复后 deleted_at 未清空")
	}
	if row.Status != "active" {
		t.Errorf("恢复后 status = %q，期望精确还原为 active", row.Status)
	}
	if row.StatusBeforeDelete != "" || row.DeletedBy != 0 {
		t.Errorf("恢复后快照未清空: status_before_delete=%q deleted_by=%d", row.StatusBeforeDelete, row.DeletedBy)
	}

	// —— 状态快照的「精确」还原：注销前是 disabled，恢复后必须仍是 disabled ——
	id2, username2 := h.registerUser("softdel_disabled")
	status, body = h.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/status", id2), admin,
		map[string]any{"status": "disabled"})
	if status != http.StatusOK {
		t.Fatalf("置为 disabled 失败: HTTP %d %v", status, body)
	}
	status, body = h.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", id2), admin, map[string]any{
		"reason": "联调用例：禁用态注销", "force": true,
	})
	if status != http.StatusOK {
		t.Fatalf("禁用态注销失败: HTTP %d %v", status, body)
	}
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/restore", id2), admin, nil)
	if status != http.StatusOK {
		t.Fatalf("恢复（禁用态）失败: HTTP %d %v", status, body)
	}
	var restored string
	h.db.Raw("SELECT status FROM users WHERE id = ?", id2).Scan(&restored)
	if restored != "disabled" {
		t.Errorf("恢复后 status = %q，期望精确还原为 disabled（不得顺手解封）", restored)
	}
	_ = username2
}

// TestLiveUserPurgeRetention 覆盖 doc104 §10.3 第 2 行。
//
// 把 deleted_at 回拨到 200 天前 → 跑一轮 purge → 用户行与全部子表行消失，
// 平台合规日志（admin_audit_logs）保留。
//
// 顺带覆盖硬删除清单的两处易漏点：order_items 与 payment_callback_logs 没有
// user_id 列，只能经父表反查；sales_commission_transactions 是业务员的佣金台账，
// 只做去标识化（customer_user_id 置 0）而不是整行删除。
func TestLiveUserPurgeRetention(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	id, username := h.registerUser("purge")
	orderNo := fmt.Sprintf("ZZLIVE%d", rand.Intn(1_000_000))
	paymentNo := fmt.Sprintf("ZZLIVEPAY%d", rand.Intn(1_000_000))
	commissionNo := fmt.Sprintf("ZZLIVECOM%d", rand.Intn(1_000_000))

	// 子表数据：钱包、登录日志、第三方绑定、角色
	if err := h.db.Exec(`INSERT INTO wallet_accounts (user_id, balance, frozen, total_income, total_expense, version)
		VALUES (?, 0, 0, 0, 0, 0)`, id).Error; err != nil {
		t.Fatalf("造钱包数据失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO login_logs (user_id, username, login_type, result, ip, platform)
		VALUES (?, ?, 'password', 'success', '127.0.0.1', 'web')`, id, username).Error; err != nil {
		t.Fatalf("造登录日志失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO user_oauth_bindings (user_id, provider, openid, unionid, nickname, avatar, status, bound_at)
		VALUES (?, ?, ?, '', '', '', 'active', NOW())`, id, liveOAuthType, "purge-openid").Error; err != nil {
		t.Fatalf("造绑定数据失败: %v", err)
	}
	// 角色关联不再手工造：注册链路已默认绑定 roles.code='user'，
	// 硬删除必须把这条注册时就存在的关联一并清掉。
	var roleCount int64
	h.db.Raw("SELECT COUNT(*) FROM user_roles WHERE user_id = ?", id).Scan(&roleCount)
	if roleCount == 0 {
		t.Fatalf("注册后应有默认客户角色关联")
	}

	// 订单 + 明细（order_items 无 user_id，必须经 orders 反查）
	if err := h.db.Exec(`INSERT INTO orders (order_no, user_id, status, created_at, updated_at)
		VALUES (?, ?, 'completed', NOW(), NOW())`, orderNo, id).Error; err != nil {
		t.Fatalf("造订单失败: %v", err)
	}
	var orderID uint64
	h.db.Raw("SELECT id FROM orders WHERE order_no = ?", orderNo).Scan(&orderID)
	if err := h.db.Exec(`INSERT INTO order_items (order_id, product_name, price, amount)
		VALUES (?, '联调用例商品', 1, 1)`, orderID).Error; err != nil {
		t.Fatalf("造订单明细失败: %v", err)
	}

	// 支付单 + 回调日志（payment_callback_logs 无 user_id，按 payment_no 反查）
	if err := h.db.Exec(`INSERT INTO payment_orders (payment_no, user_id, biz_type, amount_fen)
		VALUES (?, ?, 'order', 100)`, paymentNo, id).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO payment_callback_logs (payment_no) VALUES (?)`, paymentNo).Error; err != nil {
		t.Fatalf("造支付回调日志失败: %v", err)
	}

	// 佣金台账（属于业务员，硬删除时只去标识化）
	if err := h.db.Exec(`INSERT INTO sales_commission_transactions
		(tx_no, admin_id, type, direction, amount, biz_type, ref_no, customer_user_id)
		VALUES (?, 1, 'commission', 1, 10, 'order', ?, ?)`, commissionNo, orderNo, id).Error; err != nil {
		t.Fatalf("造佣金台账失败: %v", err)
	}

	// 等级历史（R7）：注销不得回退等级，硬删除也不得改写历史语义。
	// 造一个「已消费 2000、等级 1」的用户，并留一条升级轨迹。
	if err := h.db.Exec(
		"UPDATE users SET total_consume_amount = 2000, user_level_id = 1 WHERE id = ?", id).Error; err != nil {
		t.Fatalf("造累计消费失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO user_level_change_logs
		(user_id, from_level_code, to_level_id, to_level_code, total_consume_amount, benefits_snapshot, reason, created_at)
		VALUES (?, '', 1, 'standard', 2000, '{}', 'consume_upgrade', NOW())`, id).Error; err != nil {
		t.Fatalf("造等级变更日志失败: %v", err)
	}

	// 平台合规日志：硬删除后必须仍在。
	// detail 是 jsonb 列，必须写合法 JSON 而不是裸字符串；参数还要显式 ::text，
	// 否则 PG 在 jsonb_build_object 里推不出参数类型（42P18）。
	auditMark := "zzlive-purge-" + fmt.Sprint(rand.Intn(1_000_000))
	if err := h.db.Exec(`INSERT INTO admin_audit_logs (admin_id, admin_name, action, resource_type, detail)
		VALUES (1, 'admin', 'zzlive_retention', 'users', jsonb_build_object('mark', ?::text))`, auditMark).Error; err != nil {
		t.Fatalf("造审计日志失败: %v", err)
	}
	t.Cleanup(func() {
		h.db.Exec("DELETE FROM admin_audit_logs WHERE detail ->> 'mark' = ?::text", auditMark)
	})

	// —— 注销并把 deleted_at 回拨到留存期之外 ——
	status, body := h.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", id), admin, map[string]any{
		"reason": "联调用例：留存期清理", "force": true,
	})
	if status != http.StatusOK {
		t.Fatalf("注销失败: HTTP %d %v", status, body)
	}
	if err := h.db.Exec(
		"UPDATE users SET deleted_at = NOW() - INTERVAL '200 days' WHERE id = ?", id,
	).Error; err != nil {
		t.Fatalf("回拨 deleted_at 失败: %v", err)
	}

	// R7：注销本身不得回退累计消费与等级（退款同理）。
	// 这是**设计而非缺陷** —— 等级是权益授予依据，撤销要走人为动作与留痕。
	var consumeAfterDelete float64
	var levelAfterDelete *uint64
	h.db.Raw("SELECT total_consume_amount, user_level_id FROM users WHERE id = ?", id).
		Row().Scan(&consumeAfterDelete, &levelAfterDelete) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if consumeAfterDelete != 2000 {
		t.Errorf("R7：注销回退了累计消费：%v（期望 2000）", consumeAfterDelete)
	}
	if levelAfterDelete == nil || *levelAfterDelete != 1 {
		t.Errorf("R7：注销回退了用户等级：%v（期望 1）", levelAfterDelete)
	}

	// —— dry_run 必须一个字节都不写 ——
	status, body = h.do(http.MethodPost, "/api/v1/admin/users/purge", admin, map[string]any{
		"dry_run": true, "limit": 200,
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("dry_run 清理失败: HTTP %d %v", status, body)
	}
	// dry_run 是 bool，不是数字：用 digValue 判真值，别拿 digNumber 去比 1。
	if v, _ := digValue(body, "data", "dry_run").(bool); !v {
		t.Errorf("dry_run 未回显: %v", digValue(body, "data", "dry_run"))
	}
	if !containsCandidate(body, id) {
		t.Fatalf("dry_run 候选列表未包含 id=%d: %v", id, digValue(body, "data", "candidates"))
	}
	var stillThere int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE id = ?", id).Scan(&stillThere)
	if stillThere != 1 {
		t.Fatal("dry_run 阶段用户行已被删除：预览必须是只读的")
	}

	// —— 真正执行一轮 ——
	status, body = h.do(http.MethodPost, "/api/v1/admin/users/purge", admin, map[string]any{
		"dry_run": false, "limit": 200,
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("执行清理失败: HTTP %d %v", status, body)
	}
	if purged := digNumber(body, "data", "purged"); purged < 1 {
		t.Fatalf("purged = %v，期望至少 1（本轮应清掉该用户）: %v", purged, body)
	}
	if skipReason := candidateSkipReason(body, id); skipReason != "" {
		t.Fatalf("该用户被跳过而非清理: %s", skipReason)
	}

	var count int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE id = ?", id).Scan(&count)
	if count != 0 {
		t.Error("清理后用户行仍存在")
	}
	for _, check := range []struct {
		name  string
		query string
	}{
		{"钱包账户", "SELECT COUNT(*) FROM wallet_accounts WHERE user_id = ?"},
		{"登录日志", "SELECT COUNT(*) FROM login_logs WHERE user_id = ?"},
		{"第三方绑定", "SELECT COUNT(*) FROM user_oauth_bindings WHERE user_id = ?"},
		{"用户角色", "SELECT COUNT(*) FROM user_roles WHERE user_id = ?"},
		{"订单主表", "SELECT COUNT(*) FROM orders WHERE user_id = ?"},
	} {
		var n int64
		h.db.Raw(check.query, id).Scan(&n)
		if n != 0 {
			t.Errorf("清理后 %s 仍有 %d 行", check.name, n)
		}
	}
	// 经父表反查的两张表
	var itemCount, callbackCount int64
	h.db.Raw("SELECT COUNT(*) FROM order_items WHERE order_id = ?", orderID).Scan(&itemCount)
	if itemCount != 0 {
		t.Errorf("清理后 order_items 仍有 %d 行（order_of_user 反查失效）", itemCount)
	}
	h.db.Raw("SELECT COUNT(*) FROM payment_callback_logs WHERE payment_no = ?", paymentNo).Scan(&callbackCount)
	if callbackCount != 0 {
		t.Errorf("清理后 payment_callback_logs 仍有 %d 行（callback_of_user 反查失效）", callbackCount)
	}
	// 佣金台账：行在，但客户 ID 已去标识化
	var commissionCustomer uint64
	var commissionRows int64
	h.db.Raw("SELECT COUNT(*) FROM sales_commission_transactions WHERE tx_no = ?", commissionNo).Scan(&commissionRows)
	if commissionRows != 1 {
		t.Errorf("佣金台账行被删除：它是业务员的财务凭证，应只去标识化")
	} else {
		h.db.Raw("SELECT customer_user_id FROM sales_commission_transactions WHERE tx_no = ?", commissionNo).Scan(&commissionCustomer)
		if commissionCustomer != 0 {
			t.Errorf("佣金台账 customer_user_id = %d，期望置 0", commissionCustomer)
		}
	}
	// 平台合规日志保留
	var auditCount int64
	h.db.Raw("SELECT COUNT(*) FROM admin_audit_logs WHERE detail ->> 'mark' = ?", auditMark).Scan(&auditCount)
	if auditCount != 1 {
		t.Error("admin_audit_logs 被清理：合规日志不属于用户个人数据，必须保留")
	}
	// 清理任务留痕
	var jobCount int64
	h.db.Raw("SELECT COUNT(*) FROM job_run_logs WHERE job_name = 'user_purge'").Scan(&jobCount)
	if jobCount == 0 {
		t.Error("job_run_logs 未记录 user_purge：留存期清理必须可追溯")
	}

	h.created = removeID(h.created, id) // 已物理删除，无需再清理
}

// TestLiveVerificationReview 覆盖 doc104 §10.3 第 3 行。
//
// 提交 → 审核通过 → users.real_name_verified_at 被写 + verification_review_logs
// 有 approve 行；驳回写理由；重复审核返回 409。
func TestLiveVerificationReview(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	// —— 通过路径 ——
	id, username := h.registerUser("vreview")
	token := h.loginUser(username, liveUserPassword)

	status, body := h.do(http.MethodPost, "/api/v1/uc/verification", token, map[string]any{
		"verification_type": "personal",
		"real_name":         "张三丰",
		"id_number":         "110101199003071234",
		"mobile":            "13800138000",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("提交实名申请失败: HTTP %d %v", status, body)
	}
	appID := uint64(digNumber(body, "data", "id"))
	if appID == 0 {
		t.Fatalf("提交未返回申请 ID: %v", body)
	}
	if masked := digString(body, "data", "id_number_masked"); masked == "110101199003071234" || masked == "" {
		t.Errorf("证件号未脱敏: %q", masked)
	}
	// 证件号必须落密文，明文不得入库
	var encrypted string
	h.db.Raw("SELECT id_number_encrypted FROM verification_applications WHERE id = ?", appID).Scan(&encrypted)
	if encrypted == "" || strings.Contains(encrypted, "110101199003071234") {
		t.Errorf("证件号密文异常（应为加密串）: %q", encrypted)
	}
	// 提交即应写一条 submit 审核轨迹
	if n := countReviewLogs(t, h, appID, "submit"); n != 1 {
		t.Errorf("submit 审核轨迹 = %d 条，期望 1", n)
	}

	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/verifications/%d/approve", appID), admin,
		map[string]any{"note": "联调用例：通过"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("审核通过失败: HTTP %d %v", status, body)
	}

	var verifiedAt *time.Time
	var verifiedSource string
	h.db.Raw("SELECT real_name_verified_at, real_name_verified_source FROM users WHERE id = ?", id).
		Row().Scan(&verifiedAt, &verifiedSource) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if verifiedAt == nil {
		t.Fatal("通过后 users.real_name_verified_at 未写入（实名信任信号缺失）")
	}
	if verifiedSource != "manual" {
		t.Errorf("real_name_verified_source = %q，期望 manual", verifiedSource)
	}
	if n := countReviewLogs(t, h, appID, "approve"); n != 1 {
		t.Errorf("approve 审核轨迹 = %d 条，期望 1", n)
	}

	// 用户端状态卡应变为 approved
	_, body = h.do(http.MethodGet, "/api/v1/uc/verification", token, nil)
	if got := digString(body, "data", "status"); got != "approved" {
		t.Errorf("用户端实名状态 = %q，期望 approved", got)
	}

	// 重复审核 → 409（整单审核，状态迁移只允许 pending → approved）
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/verifications/%d/approve", appID), admin, nil)
	if status != http.StatusConflict {
		t.Fatalf("重复审核应返回 409，实际 HTTP %d %v", status, body)
	}

	// —— 驳回路径 ——
	id2, username2 := h.registerUser("vreject")
	token2 := h.loginUser(username2, liveUserPassword)
	status, body = h.do(http.MethodPost, "/api/v1/uc/verification", token2, map[string]any{
		"verification_type": "personal",
		"real_name":         "李四光",
		"id_number":         "11010119900307123X",
	})
	if status != http.StatusOK {
		t.Fatalf("第二次提交失败: HTTP %d %v", status, body)
	}
	appID2 := uint64(digNumber(body, "data", "id"))

	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/verifications/%d/reject", appID2), admin,
		map[string]any{"reject_reason": "证件影像不清晰", "reject_reason_code": "blurred"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("驳回失败: HTTP %d %v", status, body)
	}
	var rejectReason, appStatus string
	h.db.Raw("SELECT status, reject_reason FROM verification_applications WHERE id = ?", appID2).
		Row().Scan(&appStatus, &rejectReason) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if appStatus != "rejected" {
		t.Errorf("驳回后状态 = %q，期望 rejected", appStatus)
	}
	if rejectReason != "证件影像不清晰" {
		t.Errorf("驳回理由未落库: %q", rejectReason)
	}
	if n := countReviewLogs(t, h, appID2, "reject"); n != 1 {
		t.Errorf("reject 审核轨迹 = %d 条，期望 1", n)
	}
	// 用 COUNT 而不是把 NULL 扫进 *time.Time：NULL 扫进指针类型的可预期性
	// 依赖驱动实现，断言「有没有」这件事用计数最直白。
	var verifiedCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE id = ? AND real_name_verified_at IS NOT NULL", id2).Scan(&verifiedCount)
	if verifiedCount != 0 {
		t.Error("驳回的用户不应被标记为已实名")
	}
	// 驳回理由必填，且必须是 400（参数问题）而不是 500（服务端故障）：
	// 5xx 会让监控把「运营忘了填理由」算成真实故障告警。
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/verifications/%d/reject", appID2), admin, nil)
	if status != http.StatusBadRequest {
		t.Errorf("空理由的驳回应返回 400，实际 HTTP %d %v", status, body)
	}
}

// TestLiveRealnameBackdoorClosed 覆盖 doc104 §10.3 第 4 行（F13 的核心断言）。
//
// 改展示名（real_name）**不得**让用户被判定为已实名 —— 实名状态只认
// users.real_name_verified_at 这一个信号。
func TestLiveRealnameBackdoorClosed(t *testing.T) {
	h := newLiveHarness(t)
	id, username := h.registerUser("backdoor")
	token := h.loginUser(username, liveUserPassword)

	status, body := h.do(http.MethodPut, "/api/v1/uc/auth/profile", token, map[string]any{
		"name": "王五实名",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("更新资料失败: HTTP %d %v", status, body)
	}
	if got := digString(body, "data", "name"); got != "王五实名" {
		t.Errorf("显示名未更新: %q", got)
	}

	var verifiedAt *time.Time
	var realName string
	h.db.Raw("SELECT real_name_verified_at, real_name FROM users WHERE id = ?", id).
		Row().Scan(&verifiedAt, &realName) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if realName != "王五实名" {
		t.Errorf("real_name 落库 = %q，期望「王五实名」", realName)
	}
	if verifiedAt != nil {
		t.Fatal("改展示名后被判定为已实名：real_name_verified_at 后门未堵住（F13 回归）")
	}

	// 实名状态卡必须仍是 none
	_, body = h.do(http.MethodGet, "/api/v1/uc/verification", token, nil)
	if got := digString(body, "data", "status"); got != "none" {
		t.Errorf("实名状态 = %q，期望 none（改名字不该产生任何实名记录）", got)
	}

	// 反向确认：只有审核通过才写信任信号。这里直接调审核链路代价高，
	// 改为断言「信号列只能由核验链路写入」——即自助接口集合里没有任何一条
	// 能改到 real_name_verified_at（下面这行是给未来的自己看的守卫）。
	if n := countColumnsWrittenBy(t, h, id); n != 0 {
		t.Errorf("发现 %d 处自助接口能写入实名信号列", n)
	}

	// 工单前置条件走的是 ticket 模块自己的查询（doc104 §5.3 明确它也必须换口径）。
	// 只断言核验模块不够：同一用户在「实名状态卡」显示未实名、在「工单分类」
	// 却可以提交需要实名的工单，是同一个后门的两种表现。
	precondition := ticketrepo.NewPreconditionRepository(h.db)
	if ok, err := precondition.IsRealnameVerified(context.Background(), id); err != nil {
		t.Fatalf("工单前置条件查询失败: %v", err)
	} else if ok {
		t.Fatal("改展示名后工单前置条件判定为已实名：ticket/precondition_repo 的后门未堵住（F13 回归）")
	}

	// 反向确认：写入信任信号后，工单前置条件必须立刻认。
	//
	// 这里不查 /uc/verification 的状态卡：那个接口的 status 取自**最新一张申请单**
	// （none/pending/approved/rejected），而信任信号是给跨模块（工单、下单）用的
	// 判据。正常流程里两者同生同灭（审核通过 = 写信号 + 申请置 approved），
	// 直接 UPDATE 信号列只影响后者——用状态卡来断言反而测错了对象。
	now := time.Now()
	mustExec(t, h, "UPDATE users SET real_name_verified_at = ? WHERE id = ?", now, id)
	if ok, err := precondition.IsRealnameVerified(context.Background(), id); err != nil {
		t.Fatalf("工单前置条件查询失败: %v", err)
	} else if !ok {
		t.Fatal("real_name_verified_at 已写入，工单前置条件仍判定为未实名（口径未对齐）")
	}
}

// TestLiveOAuthBindAndLogin 覆盖 doc104 §10.3 第 5 行。
//
// mock provider 回调 → 绑定行写入、主绑定快照同步；解绑最后一个登录方式被拒；
// 自动注册用户归入默认组。
func TestLiveOAuthBindAndLogin(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	// 造一条已启用的模拟渠道配置（凭证为空即可，描述符没有必填字段）。
	//
	// 用 upsert 而不是裸 INSERT：本文件 init() 里注册了 livemock 描述符，
	// 应用启动时的 seed 已经替我们插过一行（且默认停用），裸 INSERT 会撞
	// uk_oauth_providers_provider。这里只需要保证「存在且启用」。
	// ON CONFLICT 必须带上 WHERE deleted_at IS NULL，才能命中那条部分唯一索引。
	if err := h.db.Exec(`INSERT INTO oauth_providers
		(provider, name, enabled, mode, descriptor, credentials, scopes, icon, sort_order, health_status, remark, created_at, updated_at)
		VALUES (?, '联调模拟渠道', true, 'api', '{}'::jsonb, '{}'::jsonb, '', 'link', 0, '', '', NOW(), NOW())
		ON CONFLICT (provider) WHERE deleted_at IS NULL
		DO UPDATE SET enabled = true, name = EXCLUDED.name, updated_at = NOW()`,
		liveOAuthType).Error; err != nil {
		t.Fatalf("造渠道配置失败: %v", err)
	}
	t.Cleanup(func() {
		h.db.Exec("DELETE FROM oauth_providers WHERE provider = ?", liveOAuthType)
	})

	// 公开渠道列表应能看到它（且不含任何凭证字段）
	status, body := h.do(http.MethodGet, "/api/v1/uc/oauth/providers", "", nil)
	if status != http.StatusOK {
		t.Fatalf("读取可用渠道失败: HTTP %d %v", status, body)
	}
	if !containsProvider(body, liveOAuthType) {
		t.Fatalf("已启用的渠道未出现在公开列表: %v", digValue(body, "data"))
	}

	// —— 登录 + 自动注册 ——
	openID := fmt.Sprintf("live-openid-%d", rand.Intn(1_000_000))
	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/uc/oauth/%s/authorize", liveOAuthType), "", nil)
	if status != http.StatusOK {
		t.Fatalf("获取授权地址失败: HTTP %d %v", status, body)
	}
	authURL := digString(body, "data", "authorize_url")
	state := queryParam(t, authURL, "state")
	if state == "" {
		t.Fatalf("授权地址缺少 state: %s", authURL)
	}

	status, redirectURL := h.doRedirect(http.MethodGet,
		fmt.Sprintf("/api/v1/uc/oauth/%s/callback?code=%s&state=%s", liveOAuthType, openID, url.QueryEscape(state)), "")
	if status != http.StatusFound {
		t.Fatalf("回调应 302 回跳前端，实际 HTTP %d", status)
	}
	if errParam := queryParam(t, redirectURL, "error"); errParam != "" {
		t.Fatalf("回调返回错误: %s（%s）", errParam, redirectURL)
	}
	ticket := queryParam(t, redirectURL, "ticket")
	if ticket == "" {
		t.Fatalf("回跳地址缺少一次性票据: %s", redirectURL)
	}

	status, body = h.do(http.MethodPost, "/api/v1/uc/oauth/exchange", "", map[string]any{"ticket": ticket})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("票据换令牌失败: HTTP %d %v", status, body)
	}
	newUserToken := digString(body, "data", "token")
	newUserID := uint64(digNumber(body, "data", "user", "id"))
	if newUserToken == "" || newUserID == 0 {
		t.Fatalf("换令牌返回不完整: %v", body)
	}
	h.created = append(h.created, newUserID)

	// 自动注册用户归入默认用户组
	var groupID *uint64
	var newUsername string
	h.db.Raw("SELECT user_group_id, username FROM users WHERE id = ?", newUserID).Row().Scan(&groupID, &newUsername) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	var defaultGroupID uint64
	h.db.Raw("SELECT id FROM user_groups WHERE is_default = true AND status = 'active' LIMIT 1").Scan(&defaultGroupID)
	if defaultGroupID == 0 {
		t.Fatal("未找到默认用户组，无法校验归组")
	}
	if groupID == nil || *groupID != defaultGroupID {
		t.Errorf("自动注册用户未归入默认组：group=%v 期望 %d", groupID, defaultGroupID)
	}
	if !strings.HasPrefix(newUsername, liveOAuthType+"_") {
		t.Errorf("自动注册用户名 = %q，期望以渠道前缀开头", newUsername)
	}

	// 绑定行 + 主绑定快照同步
	var bindingCount int64
	h.db.Raw("SELECT COUNT(*) FROM user_oauth_bindings WHERE user_id = ? AND provider = ? AND openid = ? AND status = 'active'",
		newUserID, liveOAuthType, openID).Scan(&bindingCount)
	if bindingCount != 1 {
		t.Fatalf("绑定行未写入：count=%d", bindingCount)
	}
	var snapProvider, snapOpenID string
	h.db.Raw("SELECT oauth_provider, oauth_openid FROM users WHERE id = ?", newUserID).
		Row().Scan(&snapProvider, &snapOpenID) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if snapProvider != liveOAuthType || snapOpenID != openID {
		t.Errorf("主绑定快照未同步：provider=%q openid=%q", snapProvider, snapOpenID)
	}

	// —— 已登录用户绑定新渠道 ——
	ownerID, ownerName := h.registerUser("oauthbind")
	ownerToken := h.loginUser(ownerName, liveUserPassword)
	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/uc/oauth/%s/bind-authorize", liveOAuthType), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("获取绑定授权地址失败: HTTP %d %v", status, body)
	}
	bindState := queryParam(t, digString(body, "data", "authorize_url"), "state")
	if bindState == "" {
		t.Fatal("绑定授权地址缺少 state")
	}
	bindOpenID := fmt.Sprintf("live-bind-%d", rand.Intn(1_000_000))
	status, redirectURL = h.doRedirect(http.MethodGet,
		fmt.Sprintf("/api/v1/uc/oauth/%s/callback?code=%s&state=%s", liveOAuthType, bindOpenID, url.QueryEscape(bindState)), "")
	if status != http.StatusFound {
		t.Fatalf("绑定回调应 302，实际 HTTP %d", status)
	}
	if errParam := queryParam(t, redirectURL, "error"); errParam != "" {
		t.Fatalf("绑定回调返回错误: %s", errParam)
	}
	var ownerBinding int64
	h.db.Raw("SELECT COUNT(*) FROM user_oauth_bindings WHERE user_id = ? AND provider = ? AND openid = ?",
		ownerID, liveOAuthType, bindOpenID).Scan(&ownerBinding)
	if ownerBinding != 1 {
		t.Fatalf("绑定行未写入：count=%d", ownerBinding)
	}

	// 绑定流程的票据换票必须回 bind_done 而不是当登录成功。
	//
	// 绑定走的是同一条 ticket → exchange 链路（回调是免登录入口，只有票据能
	// 安全带回结果）。若前端把它当登录处理，会给已登录用户写一个空 token 的
	// 「已登录」状态；若后端不区分，这里返回的 token 就是空的，前端只能猜。
	bindTicket := queryParam(t, redirectURL, "ticket")
	if bindTicket == "" {
		t.Fatal("绑定回调缺少一次性票据")
	}
	status, body = h.do(http.MethodPost, "/api/v1/uc/oauth/exchange", "", map[string]any{"ticket": bindTicket})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("绑定票据换票失败: HTTP %d %v", status, body)
	}
	if !digBool(body, "data", "bind_done") {
		t.Errorf("绑定流程应回 bind_done=true，实际 %v", body)
	}
	if tok := digString(body, "data", "token"); tok != "" {
		t.Errorf("绑定流程不该签发令牌，实际拿到 %q", tok)
	}
	// 绑定不该影响发起绑定的那个登录态。
	if status, _ := h.do(http.MethodGet, "/api/v1/uc/auth/userinfo", ownerToken, nil); status != http.StatusOK {
		t.Errorf("绑定后原登录态失效了: HTTP %d", status)
	}

	// 我的绑定列表：解绑守卫预判为 true（该用户还有密码这一种方式）
	status, body = h.do(http.MethodGet, "/api/v1/uc/oauth/bindings", ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取我的绑定失败: HTTP %d %v", status, body)
	}
	if len(digSlice(body, "data")) == 0 {
		t.Fatal("我的绑定列表为空")
	}

	// —— 解绑守卫：把该用户的其他登录方式全部清空后，解绑必须被拒 ——
	if err := h.db.Exec("UPDATE users SET password_hash = '', phone = '' WHERE id = ?", ownerID).Error; err != nil {
		t.Fatalf("清空登录方式失败: %v", err)
	}
	status, body = h.do(http.MethodDelete,
		fmt.Sprintf("/api/v1/uc/oauth/bindings/%s", liveOAuthType), ownerToken, nil)
	if status != http.StatusConflict {
		t.Fatalf("解绑最后一个登录方式应返回 409，实际 HTTP %d %v", status, body)
	}
	if n := bindingRowCount(t, h, ownerID); n != 1 {
		t.Error("被拒的解绑仍然删掉了绑定行")
	}

	// 恢复一种登录方式后可以解绑，且主绑定快照被清空
	if err := h.db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", "$2a$10$liveplaceholderhash", ownerID).Error; err != nil {
		t.Fatalf("恢复登录方式失败: %v", err)
	}
	status, body = h.do(http.MethodDelete,
		fmt.Sprintf("/api/v1/uc/oauth/bindings/%s", liveOAuthType), ownerToken, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("解绑失败: HTTP %d %v", status, body)
	}
	if n := bindingRowCount(t, h, ownerID); n != 0 {
		t.Error("解绑后绑定行仍存在")
	}
	var afterProvider, afterOpenID string
	h.db.Raw("SELECT oauth_provider, oauth_openid FROM users WHERE id = ?", ownerID).Row().Scan(&afterProvider, &afterOpenID) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if afterProvider != "" || afterOpenID != "" {
		t.Errorf("解绑后主绑定快照未清空：provider=%q openid=%q", afterProvider, afterOpenID)
	}

	// 自动注册用户仍有密码（随机哈希），解绑应放行
	status, body = h.do(http.MethodDelete,
		fmt.Sprintf("/api/v1/uc/oauth/bindings/%s", liveOAuthType), newUserToken, nil)
	if status != http.StatusOK {
		t.Errorf("有密码的用户解绑应放行，实际 HTTP %d %v", status, body)
	}
	_ = admin
}

// TestLiveVerificationOrphanRepair 覆盖 doc104 §10.3 第 6 行。
//
// 迁移后 user_id = 0 的孤儿行按 username 回绑；无对应用户的行带 orphan_seed 标记。
// 造一条孤儿行再重跑迁移文件（迁移本身按幂等设计，重复执行安全）。
func TestLiveVerificationOrphanRepair(t *testing.T) {
	h := newLiveHarness(t)

	_, username := h.registerUser("orphan")
	ghostName := fmt.Sprintf("zzlive_ghost_%d", rand.Intn(1_000_000))

	mustExec(t, h, `INSERT INTO verification_applications
		(user_id, username, verification_type, status, real_name, subject_name, id_type, id_number_masked, submitted_at)
		VALUES (0, ?, 'personal', 'pending', '孤儿甲', '孤儿甲', 'IDENTITY_CARD', '110101********1234', NOW())`, username)
	mustExec(t, h, `INSERT INTO verification_applications
		(user_id, username, verification_type, status, real_name, subject_name, id_type, id_number_masked, submitted_at)
		VALUES (0, ?, 'personal', 'pending', '孤儿乙', '孤儿乙', 'IDENTITY_CARD', '110101********5678', NOW())`, ghostName)

	var reboundBefore, markedBefore int64
	h.db.Raw("SELECT COUNT(*) FROM verification_applications WHERE user_id = 0 AND username = ?", username).Scan(&reboundBefore)
	if reboundBefore != 1 {
		t.Fatalf("造孤儿行失败：count=%d", reboundBefore)
	}

	sqlBytes, err := os.ReadFile("../../migrations/051_user_soft_delete_oauth_realname.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	if err := h.db.Exec(string(sqlBytes)).Error; err != nil {
		t.Fatalf("重跑迁移 051 失败: %v", err)
	}

	var reboundID uint64
	h.db.Raw("SELECT user_id FROM verification_applications WHERE username = ? AND real_name = '孤儿甲'", username).Scan(&reboundID)
	var expectedID uint64
	h.db.Raw("SELECT id FROM users WHERE username = ?", username).Scan(&expectedID)
	if reboundID != expectedID || reboundID == 0 {
		t.Errorf("孤儿行未按 username 回绑：user_id=%d，期望 %d", reboundID, expectedID)
	}

	var ghostUserID uint64
	var ghostFlags string
	h.db.Raw("SELECT user_id, COALESCE(risk_flags, '') FROM verification_applications WHERE username = ?", ghostName).
		Row().Scan(&ghostUserID, &ghostFlags) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if ghostUserID != 0 {
		t.Errorf("无法回绑的孤儿行被错误地绑到了 %d", ghostUserID)
	}
	if !strings.Contains(ghostFlags, "orphan_seed") {
		t.Errorf("无法回绑的孤儿行缺少 orphan_seed 标记：risk_flags=%q", ghostFlags)
	}
	_ = markedBefore

	// 幂等：再跑一次不应重复追加标记
	if err := h.db.Exec(string(sqlBytes)).Error; err != nil {
		t.Fatalf("第三次执行迁移失败（幂等性存疑）: %v", err)
	}
	var flagsAfter string
	h.db.Raw("SELECT COALESCE(risk_flags, '') FROM verification_applications WHERE username = ?", ghostName).Scan(&flagsAfter)
	if strings.Count(flagsAfter, "orphan_seed") != 1 {
		t.Errorf("orphan_seed 标记被重复追加：%q", flagsAfter)
	}
}

// TestLiveUserStatusEnumGuard 覆盖 doc104 §10.3 第 7 行（F3 回归）。
//
// PATCH /users/:id/status 传非法枚举必须 400，而不是把脏状态写进库。
func TestLiveUserStatusEnumGuard(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	id, _ := h.registerUser("statusguard")

	status, body := h.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/status", id), admin,
		map[string]any{"status": "banana"})
	if status != http.StatusBadRequest {
		t.Fatalf("非法状态应返回 400，实际 HTTP %d %v", status, body)
	}
	var stored string
	h.db.Raw("SELECT status FROM users WHERE id = ?", id).Scan(&stored)
	if stored != "active" {
		t.Errorf("非法请求改动了库中状态: %q", stored)
	}

	// 合法值仍应放行（守卫不能过度拦截）
	status, body = h.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/status", id), admin,
		map[string]any{"status": "disabled"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("合法状态被拒: HTTP %d %v", status, body)
	}
	h.db.Raw("SELECT status FROM users WHERE id = ?", id).Scan(&stored)
	if stored != "disabled" {
		t.Errorf("合法状态未落库: %q", stored)
	}
}

// TestLiveAssignRolesEmptyGuard 覆盖 doc104 §10.3 第 8 行（F4 回归）。
//
// 空 role_ids 必须 400 —— 放行会静默清空用户角色，是权限事故。
func TestLiveAssignRolesEmptyGuard(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	id, _ := h.registerUser("roleguard")

	// 注册链路会默认绑定 roles.code='user'，所以基线不是 0：先取快照，
	// 断言空请求既没有新增也没有清空角色。
	before := liveUserRoleIDs(t, h, id)
	if len(before) == 0 {
		t.Fatalf("注册后应有默认客户角色，实际无角色关联")
	}

	status, body := h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/roles", id), admin,
		map[string]any{"role_ids": []uint64{}})
	if status != http.StatusBadRequest {
		t.Fatalf("空角色列表应返回 400，实际 HTTP %d %v", status, body)
	}
	after := liveUserRoleIDs(t, h, id)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("空请求改动了角色关联：请求前 %v，请求后 %v", before, after)
	}

	// 非空仍应放行。角色 ID 不能硬编码：历史库与全新引导库的 roles.id 布局
	// 不同（seed 顺序差异），按 code 动态取「客户域」角色。
	var userRoleID uint64
	h.db.Raw("SELECT id FROM roles WHERE scope = 'user' ORDER BY id LIMIT 1").Scan(&userRoleID)
	if userRoleID == 0 {
		t.Fatal("缺少客户域角色（roles.scope='user'），无法验证合法分配")
	}
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/roles", id), admin,
		map[string]any{"role_ids": []uint64{userRoleID}})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("合法角色分配被拒: HTTP %d %v", status, body)
	}
	if after = liveUserRoleIDs(t, h, id); !reflect.DeepEqual(after, []uint64{userRoleID}) {
		t.Errorf("角色未落库或未收敛: %v", after)
	}
}

// liveUserRoleIDs 读取用户的角色 ID 集合（升序），用于比较请求前后的角色关联。
func liveUserRoleIDs(t *testing.T, h *liveHarness, userID uint64) []uint64 {
	t.Helper()
	var ids []uint64
	if err := h.db.Raw("SELECT role_id FROM user_roles WHERE user_id = ? ORDER BY role_id", userID).
		Scan(&ids).Error; err != nil {
		t.Fatalf("读取用户角色失败: %v", err)
	}
	return ids
}

// TestLiveUserUpdatePartialAndConflict 覆盖 doc104 §10.5 的 R1 与 R2。
//
// R2 是本轮索引改造里风险最高的一项：迁移 051 把 idx_users_username /
// idx_users_email 换成了带 WHERE deleted_at IS NULL 的部分唯一索引，约束名变了。
// 冲突识别靠错误文本匹配（PG 的 23505 不带列名），认不出新名字就会把 409 退化成
// 500 —— 运营看到的是「服务器内部错误」，然后对着一个不存在的服务端故障排查半天。
//
// R1 与它相邻：唯一冲突只有在「只传部分字段」时才会被触发（只改邮箱不该动用户名），
// 两者一起测才能同时钉住「nil 不修改」与「冲突仍映射 409」。
