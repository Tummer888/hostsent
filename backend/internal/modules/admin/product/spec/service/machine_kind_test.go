package service

import (
	"encoding/json"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/product/spec/model"
)

// TestNormalizeAndValidateMachineKind 覆盖机型和平台参数基线的互斥校验（doc122）。
//
// 这是本次改造的核心价值所在：机型在魔方云侧完全由参数取值决定，面板不做组合校验，
// 所以「NAT 参数配进 VPS 档位」这类错误在保存时拦住，才不会一路带到面板。
func TestNormalizeAndValidateMachineKind(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		params      string
		selections  string
		wantKind    string
		wantErrPart string // 期望报错包含的片段；空表示应通过
	}{
		// ---- 未标注：存量模板兼容路径 ----
		{
			name:     "未标注机型跳过校验（存量模板可继续保存）",
			kind:     "",
			params:   `{"ip_num":"3","nat_acl_limit":"-1"}`,
			wantKind: model.MachineKindUnset,
		},
		{
			name:        "非法机型被拒",
			kind:        "kvm",
			params:      `{}`,
			wantErrPart: "不合法",
		},

		// ---- 独立 IP 机型 ----
		{
			name:     "VPS：ip_num=1 且无 NAT 能力，通过",
			kind:     "vps",
			params:   `{"ip_num":1,"nat_acl_limit":-1,"nat_web_limit":-1}`,
			wantKind: model.MachineKindVPS,
		},
		{
			name:     "VPS：未给 ip_num，通过（由平台默认/其他层补）",
			kind:     "vps",
			params:   `{"area":"1"}`,
			wantKind: model.MachineKindVPS,
		},
		{
			name:        "VPS：ip_num=0 被拒（那是 NAT 的取值）",
			kind:        "vps",
			params:      `{"ip_num":0}`,
			wantErrPart: "必须 >= 1",
		},
		{
			name:        "VPS：nat_acl_limit=0 被拒（与独立 IP 互斥）",
			kind:        "vps",
			params:      `{"ip_num":1,"nat_acl_limit":0}`,
			wantErrPart: "nat_acl_limit 必须为 -1",
		},
		{
			name:        "VPS：nat_web_limit=0 被拒",
			kind:        "vps",
			params:      `{"ip_num":1,"nat_web_limit":0}`,
			wantErrPart: "nat_web_limit 必须为 -1",
		},
		{
			name:        "VPS：ip_num 非数字被拒",
			kind:        "vps",
			params:      `{"ip_num":"abc"}`,
			wantErrPart: "不是数字",
		},

		// ---- NAT 机型 ----
		{
			name:     "NAT：ip_num=0 且有端口转发，通过",
			kind:     "nat",
			params:   `{"ip_num":0,"nat_acl_limit":0,"nat_web_limit":-1}`,
			wantKind: model.MachineKindNAT,
		},
		{
			name:     "NAT：ip_num=0 且有共享建站，通过",
			kind:     "nat",
			params:   `{"ip_num":0,"nat_acl_limit":-1,"nat_web_limit":10}`,
			wantKind: model.MachineKindNAT,
		},
		{
			name:     "NAT：ip_num 为字符串 0 也认（前端表单回传字符串）",
			kind:     "nat",
			params:   `{"ip_num":"0","nat_acl_limit":"0"}`,
			wantKind: model.MachineKindNAT,
		},
		{
			name:        "NAT：缺 ip_num 被拒（必选参数，平台会直接拒绝开通）",
			kind:        "nat",
			params:      `{"nat_acl_limit":0}`,
			wantErrPart: "显式设置 ip_num = 0",
		},
		{
			name:        "NAT：ip_num=1 被拒（NAT 不该分配独立 IP）",
			kind:        "nat",
			params:      `{"ip_num":1,"nat_acl_limit":0}`,
			wantErrPart: "ip_num 必须为 0",
		},
		{
			name:        "NAT：无任何访问入口被拒（最危险配置：开出来连不上）",
			kind:        "nat",
			params:      `{"ip_num":0,"nat_acl_limit":-1,"nat_web_limit":-1}`,
			wantErrPart: "至少开通一种访问入口",
		},
		{
			name:        "NAT：两个 nat 参数都缺被拒",
			kind:        "nat",
			params:      `{"ip_num":0}`,
			wantErrPart: "至少开通一种访问入口",
		},
		{
			name:        "NAT：ip_num 暴露为客户选配项被拒（选配优先级会覆盖基线 0）",
			kind:        "nat",
			params:      `{"ip_num":0,"nat_acl_limit":0}`,
			selections:  `{"ip_num":{"range":[0,5]}}`,
			wantErrPart: "不能作为客户可选配置项",
		},
		{
			name:       "NAT：选配项不含 ip_num 时通过",
			kind:       "nat",
			params:     `{"ip_num":0,"nat_acl_limit":0}`,
			selections: `{"bw":{"range":[1,100]}}`,
			wantKind:   model.MachineKindNAT,
		},

		// ---- 机型大小写与前缀空白 ----
		{
			name:     "机型大小写与空白归一",
			kind:     "  NAT  ",
			params:   `{"ip_num":0,"nat_acl_limit":0}`,
			wantKind: model.MachineKindNAT,
		},

		// ---- JSON 容错 ----
		{
			name:        "平台参数 JSON 非法被拒",
			kind:        "vps",
			params:      `{not json}`,
			wantErrPart: "JSON 非法",
		},
		{
			name:     "平台参数为 null / 空：vps 跳过数值校验",
			kind:     "vps",
			params:   `null`,
			wantKind: model.MachineKindVPS,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind, err := normalizeAndValidateMachineKind(tc.kind, json.RawMessage(tc.params), json.RawMessage(tc.selections))
			if tc.wantErrPart == "" {
				if err != nil {
					t.Fatalf("期望通过，实际报错：%v", err)
				}
				if kind != tc.wantKind {
					t.Fatalf("期望机型 %q，实际 %q", tc.wantKind, kind)
				}
				return
			}
			if err == nil {
				t.Fatalf("期望报错包含 %q，实际通过（返回机型 %q）", tc.wantErrPart, kind)
			}
			if !strings.Contains(err.Error(), tc.wantErrPart) {
				t.Fatalf("期望报错包含 %q，实际：%v", tc.wantErrPart, err)
			}
			if kind != "" {
				t.Fatalf("报错时不应返回机型，实际 %q", kind)
			}
		})
	}
}

// TestParamNumber 守住数值解析：字符串/数字/空值三种形态，且非数字文本必须报错
// （静默当 0 会让 "abc" 变成合法的 NAT ip_num）。
func TestParamNumber(t *testing.T) {
	params := map[string]any{
		"num":    float64(0),
		"str":    "10",
		"neg":    "-1",
		"empty":  "",
		"nil":    nil,
		"spaced": " 3 ",
	}
	cases := []struct {
		key      string
		wantVal  float64
		wantHas  bool
		wantFail bool
	}{
		{key: "num", wantVal: 0, wantHas: true},
		{key: "str", wantVal: 10, wantHas: true},
		{key: "neg", wantVal: -1, wantHas: true},
		{key: "empty", wantVal: 0, wantHas: false},
		{key: "nil", wantVal: 0, wantHas: false},
		{key: "spaced", wantVal: 3, wantHas: true},
		{key: "absent", wantVal: 0, wantHas: false},
	}
	for _, tc := range cases {
		val, has, err := paramNumber(params, tc.key)
		if tc.wantFail {
			if err == nil {
				t.Fatalf("%s：期望报错", tc.key)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s：意外报错 %v", tc.key, err)
		}
		if val != tc.wantVal || has != tc.wantHas {
			t.Fatalf("%s：期望 (%v,%v)，实际 (%v,%v)", tc.key, tc.wantVal, tc.wantHas, val, has)
		}
	}

	if _, _, err := paramNumber(map[string]any{"bad": "abc"}, "bad"); err == nil {
		t.Fatal("非数字文本应报错")
	}
	if _, _, err := paramNumber(map[string]any{"bad": []any{1}}, "bad"); err == nil {
		t.Fatal("非标量应为报错")
	}
}
