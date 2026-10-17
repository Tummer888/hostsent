//go:build live

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	pkgmodel "hostsent/backend/internal/pkg/model"
)

// TestLivePanelDeletedInstanceShowsTerminated 面板删除后的实例在运维台必须显示「服务已终止」。
//
// 用户口径（第二十三轮）：「魔方云那边进行了删除，你这边肯定也要同步这个状态」。
// 同步侧只落 status=deleted（阶段不动，因为阶段是到期处置的进度），因此**展示层**
// 必须认这个值：否则一台已经不存在、账期还挂在未来的机器会继续显示「服务中」，
// 运营会以为机器还在跑并向它下发开机/关机（上游回 404）。
func TestLivePanelDeletedInstanceShowsTerminated(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveInstanceDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	mark := time.Now().UnixNano() % 1e6
	instanceID := createOpsInstance(t, db, "panel-del-"+strconv.Itoa(int(mark)), string(pkgmodel.InstanceStatusDeleted), 0)

	// 账期仍在未来 + 阶段为空：模拟"同步发现面板侧机器已删除"后的真实行形状。
	future := time.Now().AddDate(0, 0, 30)
	if err := db.Exec(`UPDATE instances SET expire_at = ?, lifecycle_stage = '', suspend_reason = NULL
		WHERE id = ?`, future, instanceID).Error; err != nil {
		t.Fatalf("准备账期失败: %v", err)
	}

	svc := newOpsService(t, db, &fakeControlProvider{})
	detail, err := svc.Detail(ctx, instanceID, false)
	if err != nil {
		t.Fatalf("Detail 失败: %v", err)
	}
	if detail.ServiceState != "destroyed" {
		t.Errorf("面板已删除的实例服务维度应为 destroyed，实得 %q（账期未到就继续显示「服务中」）",
			detail.ServiceState)
	}
	if detail.PowerState != "unknown" {
		t.Errorf("已不存在的机器电源维度应为 unknown，实得 %q", detail.PowerState)
	}
	if detail.Capabilities.Power || detail.Capabilities.Suspend || detail.Capabilities.Destroy {
		t.Errorf("面板已删除的实例必须关闭渠道动作能力位（点下去只会得到上游 404），实得 %+v",
			detail.Capabilities)
	}
	if detail.CapabilityError == "" {
		t.Error("能力位关闭时必须给出原因，否则界面只看到一片灰按钮")
	}
}

// TestLiveStoppedInstanceStillServing 反向确认：关机不是终止。
//
// 这是第 21 轮的核心口径 —— 电源维度与服���维度不能混比。补了"面板删除"判据后，
// 必须确认没有把一台正常关机的机器也算成服务终止。
func TestLiveStoppedInstanceStillServing(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveInstanceDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	mark := time.Now().UnixNano()%1e6 + 1
	instanceID := createOpsInstance(t, db, "stop-serve-"+strconv.Itoa(int(mark)), string(pkgmodel.InstanceStatusStopped), 0)
	future := time.Now().AddDate(0, 0, 30)
	if err := db.Exec(`UPDATE instances SET expire_at = ? WHERE id = ?`, future, instanceID).Error; err != nil {
		t.Fatalf("准备账期失败: %v", err)
	}

	svc := newOpsService(t, db, &fakeControlProvider{})
	detail, err := svc.Detail(ctx, instanceID, false)
	if err != nil {
		t.Fatalf("Detail 失败: %v", err)
	}
	if detail.ServiceState != "active" {
		t.Errorf("关机不影响服务维度，应为 active，实得 %q（这会把「已关机」误报成服务终止）", detail.ServiceState)
	}
	if detail.PowerState != "off" {
		t.Errorf("关机实例电源维度应为 off，实得 %q", detail.PowerState)
	}
}
