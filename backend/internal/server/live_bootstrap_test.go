//go:build live

package server

// CI 引导入口：live 测试需要一个已建表、已种子的库。SQL 迁移目录里的
// 001~003 是历史快照（在全新库上会撞自增序列，生产路径不依赖它们——
// 真实部署走 Go 侧 AutoMigrate + Seed），所以 CI 与本地联调统一用 Go 种子
// 引导：先建表、再种子，后续用例拿到的就是与生产启动等价的库。
//
// 用 LIVE_BOOTSTRAP=1 显式开启；连接目标与其它 live 用例同约定（LIVE_DB_DSN
// 优先，否则默认本机 hostsent 库）。本地已有库时不要设该变量（AutoMigrate
// 幂等，但会与正在运行的开发库并发写，没必要）。

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"hostsent/backend/internal/pkg/db"
)

func TestLiveBootstrapSchema(t *testing.T) {
	if os.Getenv("LIVE_BOOTSTRAP") == "" {
		t.Skip("本地已有库，跳过引导（CI 设 LIVE_BOOTSTRAP=1）")
	}
	conn, err := gorm.Open(postgres.Open(liveDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	if err := db.AutoMigrate(conn); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	if err := db.Seed(conn); err != nil {
		t.Fatalf("Seed 失败: %v", err)
	}
}
