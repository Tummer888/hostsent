package service

import (
	"context"
	"encoding/json"
	"testing"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/model"
)

// fakeSpecTemplateRepo 记录写入的模型，用于断言落库前的取值形态。
type fakeSpecTemplateRepo struct {
	created *model.SpecTemplate
	updated *model.SpecTemplate
}

func (f *fakeSpecTemplateRepo) List(context.Context, dto.SpecTemplateQuery) ([]model.SpecTemplate, int64, error) {
	return nil, 0, nil
}
func (f *fakeSpecTemplateRepo) FindByID(context.Context, uint64) (*model.SpecTemplate, error) {
	if f.created != nil {
		return f.created, nil
	}
	return f.updated, nil
}
func (f *fakeSpecTemplateRepo) Create(_ context.Context, item *model.SpecTemplate) error {
	f.created = item
	return nil
}
func (f *fakeSpecTemplateRepo) Update(_ context.Context, item *model.SpecTemplate) error {
	f.updated = item
	return nil
}
func (f *fakeSpecTemplateRepo) Delete(context.Context, uint64) error { return nil }

// assertJSONBText 断言写入 jsonb 列的取值形态。
//
// 核心约束：**绝不能是空串**。spec_values / platform_params / option_selections 三列都是
// jsonb，Postgres 对空串直接报 22P02（invalid input syntax for type json），表现为接口
// 返回 50001 且带着 SQL 原文。缺省必须是 nil（落 NULL）。
//
// 这是历史缺陷的回归锁：`POST /product/spec/templates` 省略任一可选 JSON 字段时报错。
func assertJSONBText(t *testing.T, label string, v *string) {
	t.Helper()
	if v == nil {
		return
	}
	if *v == "" {
		t.Fatalf("%s 落库值为空串：空串不是合法 jsonb，Postgres 会报 22P02；缺省必须是 nil", label)
	}
	if *v == "null" {
		t.Fatalf("%s 落库值为字符串 \"null\"：应落 SQL NULL（nil），不是 JSON null 文本", label)
	}
	if !json.Valid([]byte(*v)) {
		t.Fatalf("%s 落库值不是合法 JSON：%q", label, *v)
	}
}

// TestCreateWithOmittedJSONFields 省略三个可选 JSON 字段时应落 NULL 而不是空串。
// 对应缺陷：省略字段的创建请求返回 `ERROR: invalid input syntax for type json`。
func TestCreateWithOmittedJSONFields(t *testing.T) {
	repo := &fakeSpecTemplateRepo{}
	svc := NewSpecTemplateService(repo)

	// 只给最小必填（名称），三个 JSON 字段全部省略——正是直接调 API 时的报错场景。
	if _, err := svc.Create(context.Background(), dto.SpecTemplateRequest{
		Name:         "省略JSON字段",
		ProviderType: "dcimcloud",
		CPU:          2,
		Memory:       4,
		Disk:         40,
	}); err != nil {
		t.Fatalf("省略 JSON 字段不应报错，实际：%v", err)
	}
	if repo.created == nil {
		t.Fatal("未写入")
	}
	// 三列都必须落 NULL：给错一个就会复现 22P02。
	assertJSONBText(t, "spec_values", repo.created.SpecValues)
	assertJSONBText(t, "platform_params", repo.created.PlatformParams)
	assertJSONBText(t, "option_selections", repo.created.OptionSelections)

	// 顺带确认"未标注机型"不会被误写成 vps（存量档位兼容路径）。
	if repo.created.MachineKind != model.MachineKindUnset {
		t.Fatalf("未传机型应落未标注，实际 %q", repo.created.MachineKind)
	}
}

// TestCreateNormalizesEmptyAndNullJSON 显式传空串 / JSON null / 空对象时的归一行为。
func TestCreateNormalizesEmptyAndNullJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want bool // true=应落 NULL
	}{
		{name: "缺省（nil）", raw: nil, want: true},
		{name: "显式 JSON null", raw: json.RawMessage("null"), want: true},
		{name: "空白串", raw: json.RawMessage("   "), want: true},
		{name: "空对象", raw: json.RawMessage("{}"), want: false},
		{name: "正常对象", raw: json.RawMessage(`{"area":"1"}`), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeSpecTemplateRepo{}
			svc := NewSpecTemplateService(repo)
			if _, err := svc.Create(context.Background(), dto.SpecTemplateRequest{
				Name: "归一测试", ProviderType: "dcimcloud", CPU: 1, Memory: 1, Disk: 40,
				PlatformParams: tc.raw,
			}); err != nil {
				t.Fatalf("创建失败：%v", err)
			}
			got := repo.created.PlatformParams
			assertJSONBText(t, "platform_params", got)
			if tc.want && got != nil {
				t.Fatalf("应落 NULL，实际 %q", *got)
			}
			if !tc.want && got == nil {
				t.Fatal("应保留取值，实际落了 NULL")
			}
		})
	}
}

// TestUpdateWithOmittedJSONFields 更新路径同样不能写入空串（同一处 normalizeJSON）。
func TestUpdateWithOmittedJSONFields(t *testing.T) {
	existing := &model.SpecTemplate{ID: 7, Name: "旧档位", CPU: 2, Memory: 4, Disk: 40}
	repo := &fakeSpecTemplateRepo{updated: existing}
	svc := NewSpecTemplateService(repo)

	// 更新时省略字段被定义为"显式清空"（表单始终回传完整 JSON），
	// 清空同样必须落 NULL，不能落空串。
	if _, err := svc.Update(context.Background(), 7, dto.SpecTemplateRequest{
		Name: "新档位", ProviderType: "dcimcloud", CPU: 4, Memory: 8, Disk: 60,
	}); err != nil {
		t.Fatalf("更新不应报错，实际：%v", err)
	}
	assertJSONBText(t, "spec_values", existing.SpecValues)
	assertJSONBText(t, "platform_params", existing.PlatformParams)
	assertJSONBText(t, "option_selections", existing.OptionSelections)
	if existing.SpecValues != nil {
		t.Fatalf("省略字段应清空为 NULL，实际 %q", *existing.SpecValues)
	}
}

// TestCreateKeepsProvidedJSON 提供了取值时必须原样保留（不能因归一化丢内容）。
func TestCreateKeepsProvidedJSON(t *testing.T) {
	repo := &fakeSpecTemplateRepo{}
	svc := NewSpecTemplateService(repo)
	params := `{"area":"1","node":"1","ip_num":0,"nat_acl_limit":0}`
	if _, err := svc.Create(context.Background(), dto.SpecTemplateRequest{
		Name: "NAT 档位", ProviderType: "dcimcloud", MachineKind: "nat",
		CPU: 1, Memory: 1, Disk: 20,
		PlatformParams:   json.RawMessage(params),
		OptionSelections: json.RawMessage(`{"bw":{"range":[1,100],"default":"5"}}`),
	}); err != nil {
		t.Fatalf("创建失败：%v", err)
	}
	if repo.created.PlatformParams == nil || *repo.created.PlatformParams != params {
		t.Fatalf("平台参数应原样保留，实际 %v", repo.created.PlatformParams)
	}
	// ip_num=0 必须留在 JSON 里（NAT 机型靠它表达"不分配独立 IP"）。
	var parsed map[string]any
	if err := json.Unmarshal([]byte(*repo.created.PlatformParams), &parsed); err != nil {
		t.Fatalf("平台参数解析失败：%v", err)
	}
	if v, ok := parsed["ip_num"]; !ok || v != float64(0) {
		t.Fatalf("ip_num 应为 0，实际 %v（存在=%v）", v, ok)
	}
}
