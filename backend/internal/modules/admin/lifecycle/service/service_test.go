// Package service 生命周期模块单元测试（doc60 §11 重点用例）：
// 状态推导边界、续费周期计算、策略参数解析、阶段筛选窗口。
package service

import (
	"testing"
	"time"

	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
	lifecyclerepo "hostsent/backend/internal/modules/admin/lifecycle/repository"
)

// testPolicy 测试策略：宽限期 7 天、销毁保留期 30 天。
var testPolicy = &lifecyclemodel.LifecyclePolicy{
	RemindDays:      "7,3,1",
	GraceDays:       7,
	DestroyKeepDays: 30,
}

// testNow 固定基准时间，避免用例受当前时间影响。
var testNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

// TestDeriveStage 生命周期派生状态边界（含到期当天、宽限期最后一天、保留期边界）。
func TestDeriveStage(t *testing.T) {
	cases := []struct {
		name     string
		expireAt time.Time
		want     string
	}{
		{"未到期-active", testNow.Add(24 * time.Hour), lifecyclemodel.StageActive},
		{"到期当天（now==expire_at）进入宽限期", testNow, lifecyclemodel.StageGrace},
		{"宽限期内", testNow.Add(-6 * 24 * time.Hour), lifecyclemodel.StageGrace},
		{"宽限期最后一天（now==expire+7d）转为暂停", testNow.Add(-7 * 24 * time.Hour), lifecyclemodel.StageSuspended},
		{"保留期内", testNow.Add(-15 * 24 * time.Hour), lifecyclemodel.StageSuspended},
		{"保留期最后一天（now==expire+37d）转为销毁", testNow.Add(-37 * 24 * time.Hour), lifecyclemodel.StageDestroyed},
		{"保留期结束已久", testNow.Add(-100 * 24 * time.Hour), lifecyclemodel.StageDestroyed},
	}
	svc := &lifecycleService{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := svc.DeriveStage(tc.expireAt, testNow, testPolicy); got != tc.want {
				t.Errorf("DeriveStage(expire=%s) = %s, want %s", tc.expireAt.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

// TestDeriveStageForUser 用户端派生阶段与管理端同规则。
func TestDeriveStageForUser(t *testing.T) {
	cases := []struct {
		expireAt time.Time
		want     string
	}{
		{testNow.Add(1 * time.Hour), lifecyclemodel.StageActive},
		{testNow.Add(-3 * 24 * time.Hour), lifecyclemodel.StageGrace},
		{testNow.Add(-20 * 24 * time.Hour), lifecyclemodel.StageSuspended},
		{testNow.Add(-50 * 24 * time.Hour), lifecyclemodel.StageDestroyed},
	}
	for _, tc := range cases {
		if got := deriveStageForUser(tc.expireAt, testNow, testPolicy); got != tc.want {
			t.Errorf("deriveStageForUser(expire=%s) = %s, want %s", tc.expireAt, got, tc.want)
		}
	}
}

// TestNextExpireAt 续费周期计算：按计费模式延长；已过期实例从当前时间起算。
func TestNextExpireAt(t *testing.T) {
	svc := &renewalService{}
	base := testNow.Add(10 * 24 * time.Hour) // 未到期实例

	cases := []struct {
		name        string
		billingMode string
		periodCount int
		want        time.Time
	}{
		{"月付1周期", "monthly", 1, base.AddDate(0, 1, 0)},
		{"月付3周期", "monthly", 3, base.AddDate(0, 3, 0)},
		{"季付2周期", "quarter", 2, base.AddDate(0, 6, 0)},
		{"年付1周期", "yearly", 1, base.AddDate(12, 0, 0)},
		{"日付5周期", "daily", 5, base.AddDate(0, 0, 5)},
		{"未知模式默认按月", "hourly", 2, base.AddDate(0, 2, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.nextExpireAt(base, tc.billingMode, tc.periodCount)
			if got == nil {
				t.Fatal("nextExpireAt returned nil")
			}
			if !got.Equal(tc.want) {
				t.Errorf("nextExpireAt(%s,%d) = %s, want %s", tc.billingMode, tc.periodCount, got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}

	// 已过期实例：结果必须不早于当前时间 + 周期（从 now 起算，不落在过去）
	expiredBase := testNow.Add(-20 * 24 * time.Hour)
	got := svc.nextExpireAt(expiredBase, "monthly", 1)
	if got == nil {
		t.Fatal("nextExpireAt returned nil")
	}
	minExpected := time.Now().AddDate(0, 1, 0).Add(-time.Minute)
	if got.Before(minExpected) {
		t.Errorf("过期实例续费应从 now 起算: got %s, 期望 >= %s", got.Format(time.RFC3339), minExpected.Format(time.RFC3339))
	}
}

// TestStageWindow 阶段筛选窗口与派生规则一致（grace 不混入 suspended）。
func TestStageWindow(t *testing.T) {
	graceEnd := testNow.AddDate(0, 0, -testPolicy.GraceDays)                 // expire_at 下界（grace）
	destroyEnd := graceEnd.AddDate(0, 0, -testPolicy.DestroyKeepDays)        // expire_at 下界（suspended）
	thirtyDays := testNow.AddDate(0, 0, 30)

	cases := []struct {
		stage        string
		expireAfter  *time.Time
		expireBefore *time.Time
	}{
		{"active", &testNow, nil},
		{"expiring", &testNow, &thirtyDays},
		{"grace", &graceEnd, &testNow},
		{"suspended", &destroyEnd, &graceEnd},
		{"destroyed", nil, &destroyEnd},
		{"all", nil, nil},
	}
	for _, tc := range cases {
		t.Run("stage="+tc.stage, func(t *testing.T) {
			w := stageWindow(tc.stage, testNow, testPolicy)
			if tc.expireAfter == nil && tc.expireBefore == nil {
				if w != nil {
					t.Fatalf("stage=%s 期望 nil 窗口, got %+v", tc.stage, w)
				}
				return
			}
			if w == nil {
				t.Fatalf("stage=%s 期望窗口, got nil", tc.stage)
			}
			if !sameTimePtr(w.ExpireAfter, tc.expireAfter) || !sameTimePtr(w.ExpireBefore, tc.expireBefore) {
				t.Errorf("stage=%s 窗口 = [%v, %v], want [%v, %v]",
					tc.stage, fmtTimePtr(w.ExpireAfter), fmtTimePtr(w.ExpireBefore), fmtTimePtr(tc.expireAfter), fmtTimePtr(tc.expireBefore))
			}
		})
	}
}

// TestStageWindowDisjoint 验证 grace 与 suspended 窗口不重叠。
func TestStageWindowDisjoint(t *testing.T) {
	grace := stageWindow(lifecyclemodel.StageGrace, testNow, testPolicy)
	susp := stageWindow(lifecyclemodel.StageSuspended, testNow, testPolicy)
	if grace == nil || susp == nil || grace.ExpireAfter == nil || susp.ExpireBefore == nil {
		t.Fatal("窗口缺失")
	}
	// grace 的 expire_at 下界（graceEnd）必须晚于或等于 suspended 的上界
	if grace.ExpireAfter.Before(*susp.ExpireBefore) {
		t.Errorf("grace 与 suspended 窗口重叠: grace 下界 %s < suspended 上界 %s",
			grace.ExpireAfter.Format(time.RFC3339), susp.ExpireBefore.Format(time.RFC3339))
	}
}

// TestParseRemindDays 提醒天数配置解析。
func TestParseRemindDays(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{"默认配置", "7,3,1", []int{7, 3, 1}, false},
		{"含空格", "7, 3", []int{7, 3}, false},
		{"空串", "", nil, false},
		{"非数字", "a,b", nil, true},
		{"超范围", "400", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRemindDays(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseRemindDays(%q) err = %v, wantErr %v", tc.raw, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseRemindDays(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseRemindDays(%q)[%d] = %d, want %d", tc.raw, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestDaysLeft 剩余天数向上取整。
func TestDaysLeft(t *testing.T) {
	cases := []struct {
		expireAt time.Time
		want     int
	}{
		{testNow.Add(3 * 24 * time.Hour), 3},
		{testNow.Add(3 * 24 * time.Hour).Add(time.Hour), 4}, // 向上取整
		{testNow, 0},
		{testNow.Add(-3 * 24 * time.Hour), -3},
	}
	for _, tc := range cases {
		if got := daysLeft(tc.expireAt, testNow); got != tc.want {
			t.Errorf("daysLeft(%s) = %d, want %d", tc.expireAt.Format(time.RFC3339), got, tc.want)
		}
	}
}

// TestListExpiringWindow 编译期引用 lifecyclerepo.StageWindow，保证导入有效。
func TestListExpiringWindow(t *testing.T) {
	var _ *lifecyclerepo.StageWindow = stageWindow("active", testNow, testPolicy)
}

func sameTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return "<nil>"
	}
	return t.Format(time.RFC3339)
}
