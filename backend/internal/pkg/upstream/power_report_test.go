package upstream_test

import (
	"testing"

	_ "hostsent/backend/internal/pkg/upstream/dcimcloud"
	_ "hostsent/backend/internal/pkg/upstream/zjmf"

	"hostsent/backend/internal/pkg/upstream"
)

// 电源状态回报能力位：不同上游对"状态"的语义完全不同，必须如实声明。
//
// 背景（本轮实测定位的真缺陷）：财务型上游（魔方财务）的 host/header 只返回
// domainstatus（Active/Pending/Suspended，**服务**状态）与 status（Paid/Unpaid，
// **账单**状态），**没有任何机器电源字段**。而适配器把 Active 映射成 running ——
// 于是本地"已关机"一旦回源就被冲成"运行中"：运营点完关机、界面显示已关机，
// 一同步就变回来，以为关机没生效（用户反馈"状态显示混乱"的根因之一）。
//
// 这两条断言把事实钉住：改错了会让上面那个症状复活。
func TestPowerStateReportingCapability(t *testing.T) {
	cases := []struct {
		providerType string
		want         bool
		why          string
	}{
		{
			providerType: "dcimcloud",
			want:         true,
			why:          "魔方云 GET /clouds/{id}/status 返回 on/off/wait_reboot，是真实电源状态",
		},
		{
			providerType: "zjmf",
			want:         false,
			why:          "魔方财务只有 domainstatus（服务）/status（账单），没有电源字段",
		},
	}
	for _, tc := range cases {
		t.Run(tc.providerType, func(t *testing.T) {
			d, ok := upstream.Descriptor(tc.providerType)
			if !ok {
				t.Fatalf("描述符未注册（%s）：%s", tc.providerType, tc.why)
			}
			if d.ReportsPowerState != tc.want {
				t.Errorf("%s.ReportsPowerState = %v, want %v —— %s",
					tc.providerType, d.ReportsPowerState, tc.want, tc.why)
			}
		})
	}
}

// 兜底语义：描述符缺失（自研/测试桩）时保守返回 true，保持既有行为。
func TestSupportsPowerStateReportingFallback(t *testing.T) {
	if !upstream.SupportsPowerStateReporting(nil) {
		t.Error("nil provider 应保守返回 true")
	}
}
