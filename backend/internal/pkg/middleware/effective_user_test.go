package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	appauth "hostsent/backend/internal/pkg/auth"
)

// fakeSubPermResolver 内存版子账号权限解析器。
type fakeSubPermResolver struct {
	perms map[uint64][]string
}

func (f *fakeSubPermResolver) PermissionsOf(_ context.Context, userID uint64) ([]string, error) {
	return f.perms[userID], nil
}

func newTestContext(userID, ownerID uint64, isSub bool) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(userClaimsContextKey, &appauth.UserClaims{
		UserID:      userID,
		Username:    "member",
		OwnerUserID: ownerID,
		IsSub:       isSub,
	})
	return c, rec
}

// TestEffectiveUserIDReturnsOwner 子账号取到主账号 ID，操作人仍是子账号自身。
func TestEffectiveUserIDReturnsOwner(t *testing.T) {
	c, _ := newTestContext(9, 5, true)

	if got := EffectiveUserID(c); got != 5 {
		t.Fatalf("EffectiveUserID = %d, want 5（主账号）", got)
	}
	if got := ActorUserID(c); got != 9 {
		t.Fatalf("ActorUserID = %d, want 9（子账号自身）", got)
	}
	if !IsSubAccount(c) {
		t.Fatal("IsSubAccount = false, want true")
	}

	// 主账号：归属与操作人都是自身
	main, _ := newTestContext(7, 0, false)
	if got := EffectiveUserID(main); got != 7 {
		t.Fatalf("主账号 EffectiveUserID = %d, want 7", got)
	}
	if got := ActorUserID(main); got != 7 {
		t.Fatalf("主账号 ActorUserID = %d, want 7", got)
	}
	if IsSubAccount(main) {
		t.Fatal("主账号 IsSubAccount = true, want false")
	}
}

// TestSubAccountDeniedRecharge 子账号调用资金入口被硬编码拒绝，主账号放行。
func TestSubAccountDeniedRecharge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sub, subRec := newTestContext(9, 5, true)
	handler := RejectSubAccount()
	handler(sub)
	if !sub.IsAborted() {
		t.Fatal("子账号充值请求未被拒绝")
	}
	var body map[string]any
	if err := json.Unmarshal(subRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if code, _ := body["code"].(float64); int(code) != 40301 {
		t.Fatalf("子账号充值响应 code = %v, want 40301", body["code"])
	}

	main, mainRec := newTestContext(7, 0, false)
	RejectSubAccount()(main)
	if main.IsAborted() {
		t.Fatalf("主账号充值被拒绝: %s", mainRec.Body.String())
	}
}

// TestSubAccountRespectsPermissionSet 子账号只放行已授予的权限码。
func TestSubAccountRespectsPermissionSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resolver := &fakeSubPermResolver{perms: map[uint64][]string{9: {appauth.PermInstanceView, appauth.PermBillingView}}}

	// 持有 instance:view → 放行
	allowed, _ := newTestContext(9, 5, true)
	RequireUserPermission(resolver, appauth.PermInstanceView)(allowed)
	if allowed.IsAborted() {
		t.Fatal("持有 instance:view 仍被拒绝")
	}

	// 未持有 instance:operate → 拒绝
	denied, deniedRec := newTestContext(9, 5, true)
	RequireUserPermission(resolver, appauth.PermInstanceOperate)(denied)
	if !denied.IsAborted() {
		t.Fatal("未持有 instance:operate 却放行")
	}
	var body map[string]any
	if err := json.Unmarshal(deniedRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if code, _ := body["code"].(float64); int(code) != 40301 {
		t.Fatalf("越权响应 code = %v, want 40301", body["code"])
	}

	// 主账号无权限码也放行
	main, _ := newTestContext(7, 0, false)
	RequireUserPermission(resolver, appauth.PermSubAccountManage)(main)
	if main.IsAborted() {
		t.Fatal("主账号被权限中间件拦截")
	}

	// 子账号即使被授予 subaccount:manage（被过滤后的注入）也不放行
	crafted, _ := newTestContext(9, 5, true)
	RequireUserPermission(&fakeSubPermResolver{perms: map[uint64][]string{9: {appauth.PermSubAccountManage}}}, appauth.PermSubAccountManage)(crafted)
	if !crafted.IsAborted() {
		t.Fatal("子账号持有 subaccount:manage 仍应被拒绝")
	}
}

// TestGrantablePermissionFilterDeniesDangerousCodes 权限码白名单过滤掉资金/实名/成员管理。
func TestGrantablePermissionFilterDeniesDangerousCodes(t *testing.T) {
	if appauth.IsGrantableUserPermission("billing:recharge") {
		t.Fatal("billing:recharge 不应可授予子账号")
	}
	if appauth.IsGrantableUserPermission("realname:submit") {
		t.Fatal("realname:submit 不应可授予子账号")
	}
	if appauth.IsGrantableUserPermission(appauth.PermSubAccountManage) {
		t.Fatal("subaccount:manage 不应可授予子账号")
	}
	if !appauth.IsGrantableUserPermission(appauth.PermInstanceOperate) {
		t.Fatal("instance:operate 应可授予子账号")
	}
}
