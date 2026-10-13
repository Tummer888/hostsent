package dcimcloud

import (
	"testing"
)

// TestCollectOptionsKeepsZeroIPNum NAT 链路的下发契约（doc122）。
//
// NAT 机型靠 ip_num=0 表达「不分配独立 IP」，而 0 是最容易被"空值即忽略"逻辑
// 吞掉的取值：一旦 ip_num 没下发，面板会按必填校验直接拒绝开通，或者退回默认
// 分配独立 IP —— 两种都不是 NAT 档位承诺给客户的东西。此用例把 0 在各形态下
// 都能活到 opts 里钉死。
func TestCollectOptionsKeepsZeroIPNum(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]interface{}
		// wantIPNum 允许逐例覆盖期望值（第 4 例专门验优先级，其余都是 "0"）。
		wantIPNum string
	}{
		{
			// 商品 platform_params（jsonb）走 parseConfigOptions 解析，数字一律是 float64。
			name:  "configoptions 子映射里的 float64 0",
			extra: map[string]interface{}{"configoptions": map[string]interface{}{"ip_num": float64(0), "nat_acl_limit": float64(0)}},
		},
		{
			name:  "Extra 顶层 float64 0",
			extra: map[string]interface{}{"ip_num": float64(0), "nat_acl_limit": float64(0)},
		},
		{
			// 客户选配与前端表单可能回传字符串。
			name:  "字符串 0",
			extra: map[string]interface{}{"ip_num": "0", "nat_acl_limit": "0"},
		},
		{
			// 选配走 Extra 顶层、档位基线走 configoptions，顶层优先级更高。
			name:      "顶层 0 覆盖子映射非 0",
			extra:     map[string]interface{}{"ip_num": float64(0), "nat_acl_limit": float64(0), "configoptions": map[string]interface{}{"ip_num": float64(3)}},
			wantIPNum: "0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := collectOptions(tc.extra)
			wantIPNum := tc.wantIPNum
			if wantIPNum == "" {
				wantIPNum = "0"
			}
			if opts["ip_num"] != wantIPNum {
				t.Fatalf("ip_num 应为 %q，实际 %q（0 被吞掉会导致 NAT 机型退回分配独立 IP）", wantIPNum, opts["ip_num"])
			}
			if opts["nat_acl_limit"] != "0" {
				t.Fatalf("nat_acl_limit 应为 \"0\"，实际 %q", opts["nat_acl_limit"])
			}
		})
	}
}

// TestCloudOptionKeysCoverNatParams NAT 机型所需的参数必须都在白名单里：
// 白名单外的键在 CreateInstance 里被静默丢弃，档位配了也不会下发。
func TestCloudOptionKeysCoverNatParams(t *testing.T) {
	for _, key := range []string{"ip_num", "nat_acl_limit", "nat_web_limit"} {
		if !containsString(cloudOptionKeys, key) {
			t.Fatalf("cloudOptionKeys 缺少 NAT 机型参数 %q，档位里配了也不会下发到 /clouds", key)
		}
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
