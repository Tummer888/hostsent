package service

import (
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/resource/reconcile/dto"
	reconmodel "hostsent/backend/internal/modules/admin/resource/reconcile/model"
	"hostsent/backend/internal/modules/admin/resource/reconcile/repository"
)

// TestBuildItem_CarriesServiceDimension 对账行必须同时下发两个维度。
//
// 用户实测误报的展示侧根因：对外只给了一个 status_name（机器电源维度），
// 页面拿它当"本地状态"与上游 domainstatus 并列，于是"本地 已停止 / 上游 运行中"
// 看起来就是矛盾。修复后本地侧改用服务维度（local_service_state_name），
// 机器状态仍保留给"这台机器开没开机"的场景 —— 两者必须都在，且不能互相冒充。
func TestBuildItem_CarriesServiceDimension(t *testing.T) {
	row := repository.ReconcileRow{
		ID:                9,
		InstanceID:        "2090",
		Name:              "hs-8errv323-mki82x",
		Status:            "stopped", // 机器维度：已关机
		LocalServiceState: "active",  // 服务维度：服务仍在（未到期未暂停）
		UpstreamStatus:    "Active",
		StatusAnomaly:     "ok",
		PriceAnomaly:      "ok",
		ExpireAnomaly:     "ok",
	}
	item := buildItem(row, map[uint64]string{}, map[uint64]repository.ProviderMeta{})

	if item.StatusName != "已停止" {
		t.Errorf("机器维度展示名 = %q, want 已停止", item.StatusName)
	}
	if item.LocalServiceState != "active" {
		t.Errorf("本地服务维度值 = %q, want active", item.LocalServiceState)
	}
	if item.LocalServiceStateName != "服务中" {
		t.Errorf("本地服务维度展示名 = %q, want 服务中", item.LocalServiceStateName)
	}
	if item.StatusAnomalyName != "" {
		t.Errorf("关机 + 上游 Active 不应有状态异常名，实际 %q", item.StatusAnomalyName)
	}
	if !strings.Contains(item.Detail, "一致") {
		t.Errorf("全一致时结论应含「一致」，实际 %q", item.Detail)
	}
}

// TestBuildItem_StatusDetailUsesServiceWords 状态差异文案说"服务"，不说机器。
//
// 旧文案用机器状态名词（"上游状态 运行中 与本地 已停止 不一致"），
// 与用户的操作直觉冲突：他刚关机，界面就说"状态不一致"，看起来像系统没同步。
func TestBuildItem_StatusDetailUsesServiceWords(t *testing.T) {
	row := repository.ReconcileRow{
		ID:                11,
		InstanceID:        "17",
		Status:            "running",
		LocalServiceState: "active",
		UpstreamStatus:    "Suspended",
		StatusAnomaly:     dto.StatusMismatch,
		PriceAnomaly:      "ok",
		ExpireAnomaly:     "ok",
	}
	item := buildItem(row, map[uint64]string{}, map[uint64]repository.ProviderMeta{})
	if !strings.Contains(item.Detail, "服务") {
		t.Errorf("状态差异文案应说明是服务维度，实际 %q", item.Detail)
	}
	if strings.Contains(item.Detail, "运行中") || strings.Contains(item.Detail, "已停止") {
		t.Errorf("状态差异文案不该出现机器电源状态措辞，实际 %q", item.Detail)
	}

	stale := buildItem(repository.ReconcileRow{
		ID:                12,
		Status:            "stopped",
		LocalServiceState: "active",
		UpstreamStatus:    "Deleted",
		StatusAnomaly:     dto.StatusStale,
		PriceAnomaly:      "ok",
		ExpireAnomaly:     "ok",
	}, map[uint64]string{}, map[uint64]repository.ProviderMeta{})
	if !strings.Contains(stale.Detail, "计费") {
		t.Errorf("上游已删除的结论应点明计费风险，实际 %q", stale.Detail)
	}
}

// TestStatusNamesDoNotClaimPowerSemantics 状态差异类型名不得描述机器开关机。
//
// 旧名"上游已暂停本地未同步"把服务暂停说成了机器动作，用户一眼读成
// "我关了机、系统没帮我同步"。名称只描述服务维度即可。
func TestStatusNamesDoNotClaimPowerSemantics(t *testing.T) {
	for kind, name := range map[string]string{
		"status_mismatch": "服务状态两侧不一致",
		"status_stale":    "上游已终止本地仍在服务",
	} {
		if got := labelOf(statusAnomalyNames, kind, ""); got != name {
			t.Errorf("%s 展示名 = %q, want %q", kind, got, name)
		}
	}
}

// TestServiceStateLabel 台账两侧取值都要翻成中文（不露内部码）。
//
// 台账页把 local_value/upstream_value 并排展示为"本地 / 上游"。旧实现直接回显
// 数据库里的取值，于是出现 "active" 对 "Active" 这种机器码对照；价格与到期两类
// 是金额/日期，必须原样通过（不能被误翻）。
func TestServiceStateLabel(t *testing.T) {
	// 本地归一值与上游原文映射到**同一套**中文词。
	groups := [][]string{
		{"active", "Active"},
		{"provisioning", "Pending"},
		{"suspended", "Suspended"},
		{"destroyed", "Deleted"},
		{"cancelled", "Cancelled"},
	}
	want := []string{"服务中", "开通中", "已暂停", "已终止", "已取消"}
	for i, g := range groups {
		for _, v := range g {
			if got := reconmodel.ServiceStateLabel(v); got != want[i] {
				t.Errorf("ServiceStateLabel(%q) = %q, want %q", v, got, want[i])
			}
		}
	}
	// 风控冻结与宽限期只有一侧有，单独钉住。
	if got := reconmodel.ServiceStateLabel("Fraud"); got != "风控冻结" {
		t.Errorf("Fraud = %q, want 风控冻结", got)
	}
	if got := reconmodel.ServiceStateLabel("grace"); got != "宽限期" {
		t.Errorf("grace = %q, want 宽限期", got)
	}
	// 非状态取值原样通过：价格与到期两侧进的是同一套兜底。
	for _, v := range []string{"38.00", "2026-01-01", "42.50", ""} {
		if got := reconmodel.ServiceStateLabel(v); got != v {
			t.Errorf("ServiceStateLabel(%q) 被改写为 %q（金额/日期不得翻译）", v, got)
		}
	}
	// 未登记的上游新状态原样回显（便于发现新状态），不吞不打码。
	if got := reconmodel.ServiceStateLabel("SomeNewStatus"); got != "SomeNewStatus" {
		t.Errorf("未知状态应原样回显，实际 %q", got)
	}
}
