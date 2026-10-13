package service

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"testing"
)

// TestSignString_PhpBaseline 用 PHP 7.3 生成的基线向量校验 signString 与上游 createSign
// 逐字节一致。
//
// 为什么必须锁死这组向量：验签是免登录入口的唯一防线，一旦 Go 侧 json.Marshal 的输出
// 与 PHP json_encode 有哪怕一个字节差异（字段顺序、空格、斜杠转义），所有真实推送都会
// 被判为伪造并被丢弃——而且是静默的安全失败：本地日志只会说"验签失败"。
//
// 基线由 PHP 脚本用与上游 createSign 相同的语句生成：
//
//	$params = ["id"=>$id]; $params["token"]=$token; $params["rand_str"]=$rand;
//	ksort($params, SORT_STRING); $str = json_encode($params);
//	echo strtoupper(md5($str));
func TestSignString_PhpBaseline(t *testing.T) {
	cases := []struct {
		name     string
		id       string
		token    string
		rand     string
		wantRaw  string
		wantSign string
	}{
		{
			name:     "常规 32 位令牌",
			id:       "123",
			token:    "0123456789abcdef0123456789abcdef",
			rand:     "Ab3xY9",
			wantRaw:  `{"id":"123","rand_str":"Ab3xY9","token":"0123456789abcdef0123456789abcdef"}`,
			wantSign: "9BF0B6F787BE260D62B85E0FD882D6FA",
		},
		{
			name:     "id 为 0 与全 f 令牌",
			id:       "0",
			token:    "ffffffffffffffffffffffffffffffff",
			rand:     "aaaaaa",
			wantRaw:  `{"id":"0","rand_str":"aaaaaa","token":"ffffffffffffffffffffffffffffffff"}`,
			wantSign: "82C80773B0BF3F54B54FB4BA1DEA7316",
		},
		{
			name:     "大整数 id（不能退化为科学计数法）",
			id:       "9007199254740993",
			token:    "deadbeefdeadbeefdeadbeefdeadbeef",
			rand:     "Zz9Qq1",
			wantRaw:  `{"id":"9007199254740993","rand_str":"Zz9Qq1","token":"deadbeefdeadbeefdeadbeefdeadbeef"}`,
			wantSign: "F69C51B4B1D84316D2A86B75F3D97C43",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := signString(tc.id, tc.token, tc.rand)
			if err != nil {
				t.Fatalf("signString 出错: %v", err)
			}
			if got != tc.wantRaw {
				t.Fatalf("被签内容不一致\n got: %s\nwant: %s", got, tc.wantRaw)
			}
			sum := md5.Sum([]byte(got))
			if sign := strings.ToUpper(hex.EncodeToString(sum[:])); sign != tc.wantSign {
				t.Fatalf("签名不一致 got=%s want=%s", sign, tc.wantSign)
			}
		})
	}
}

// TestVerify 覆盖验签的通过/失败分支，以及"缺任一要素即拒绝"。
func TestVerify(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	const id, randStr, sign = "123", "Ab3xY9", "9BF0B6F787BE260D62B85E0FD882D6FA"

	if !Verify(id, randStr, sign, token) {
		t.Fatal("正确签名应通过")
	}
	// 大小写不敏感（上游给的是大写，但不要因为格式差异拒绝合法请求）。
	if !Verify(id, randStr, strings.ToLower(sign), token) {
		t.Fatal("小写签名也应通过")
	}

	bad := []struct {
		name               string
		id, rand, sig, tok string
	}{
		{"签名不符", id, randStr, "00000000000000000000000000000000", token},
		{"令牌不符", id, randStr, sign, "ffffffffffffffffffffffffffffffff"},
		{"id 被改", "124", randStr, sign, token},
		{"rand_str 被改", id, "Ab3xY0", sign, token},
		{"签名缺失", id, randStr, "", token},
		{"rand_str 缺失", id, "", sign, token},
		{"id 缺失", "", randStr, sign, token},
		{"令牌缺失", id, randStr, sign, ""},
	}
	for _, tc := range bad {
		if Verify(tc.id, tc.rand, tc.sig, tc.tok) {
			t.Errorf("%s：应验签失败", tc.name)
		}
	}
}

// TestNormalizeUnix 秒/毫秒/非法值。
func TestNormalizeUnix(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"1798761600", 1798761600, true},
		{"1798761600000", 1798761600, true}, // 毫秒归一为秒
		{"", 0, false},
		{"abc", 0, false},
		{"0", 0, false},
		{"-5", 0, false},
	}
	for _, tc := range cases {
		got, ok := normalizeUnix(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("normalizeUnix(%q) = (%d,%v)，期望 (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// TestFirstIP 取末个非空 IP（主 IP 走 dedicatedip，附加 IP 才是 private_ip）。
func TestFirstIP(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"1.2.3.4", "1.2.3.4"},
		{"1.2.3.4,5.6.7.8", "5.6.7.8"},
		{"1.2.3.4, 5.6.7.8 ,9.9.9.9", "9.9.9.9"},
		{"1.2.3.4\n5.6.7.8", "5.6.7.8"},
		{" , ", ""},
	}
	for _, tc := range cases {
		if got := firstIP(tc.in); got != tc.want {
			t.Errorf("firstIP(%q) = %q，期望 %q", tc.in, got, tc.want)
		}
	}
}

// TestMergeRawData 合并语义：空值不覆盖、非 JSON 既有内容按空对象处理。
func TestMergeRawData(t *testing.T) {
	// 既有 JSON 保留未提及字段。
	got := mergeRawData(`{"keep":"v","domainstatus":"Active"}`, map[string]any{"domainstatus": "Suspended"})
	if !strings.Contains(got, `"keep":"v"`) || !strings.Contains(got, `"domainstatus":"Suspended"`) {
		t.Errorf("合并结果不正确: %s", got)
	}
	// 空字符串不覆盖既有值。
	got = mergeRawData(`{"domainstatus":"Active"}`, map[string]any{"domainstatus": ""})
	if got != `{"domainstatus":"Active"}` {
		t.Errorf("空值不应覆盖: %s", got)
	}
	// 脏数据（非 JSON）不 panic，按空对象处理。
	got = mergeRawData(`not json`, map[string]any{"domainstatus": "Active"})
	if !strings.Contains(got, `"domainstatus":"Active"`) {
		t.Errorf("脏数据应按空对象处理: %s", got)
	}
	// 无变化时原样返回（避免无意义刷新 updated_at）。
	src := `{"domainstatus":"Active"}`
	if got = mergeRawData(src, map[string]any{"domainstatus": "Active"}); got != src {
		t.Errorf("无变化应原样返回: %s", got)
	}
}

// TestDomainStatusToLocal 上游状态映射与 zjmf 拉取侧口径一致。
func TestDomainStatusToLocal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Active", "running"},
		{"Pending", "creating"},
		{"Suspended", "stopped"},
		{"Cancelled", "stopped"},
		{"Fraud", "stopped"},
		{"Deleted", "deleted"},
	}
	for _, tc := range cases {
		if got, ok := domainStatusToLocal[tc.in]; !ok || got != tc.want {
			t.Errorf("domainStatusToLocal[%q] = (%q,%v)，期望 %q", tc.in, got, ok, tc.want)
		}
	}
	if _, ok := domainStatusToLocal["Weird"]; ok {
		t.Error("未知状态不应有映射（应保持本地原状态）")
	}
}
