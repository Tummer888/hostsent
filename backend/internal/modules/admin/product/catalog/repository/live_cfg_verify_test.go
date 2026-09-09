//go:build live

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
)

// TestLiveConfigGroupsCircular 针对真实库验证：BuildProvisionRequest 子表优先路径读到的
// config_groups，与 raw_specs 惰性解析提取的 config_groups 在 (option.upstream_id, sub.upstream_id)
// 集合上一致，从而保证下游 buildCartConfigOption 的 cart 映射不回归。
// 仅在带 -tags live 时编译/运行；常规 `go test ./...` 跳过。
func TestLiveConfigGroupsCircular(t *testing.T) {
	db := openLiveDB(t)
	ctx := context.Background()
	repo := NewProductRepository(db)

	// 任取一个有 source_product_id 且子表已回填的产品。
	var products []struct {
		ID              uint64
		SourceProductID uint64
	}
	if err := db.Raw("SELECT id, source_product_id FROM products WHERE source_product_id > 0 LIMIT 5").Scan(&products).Error; err != nil {
		t.Fatalf("查询 products 失败: %v", err)
	}
	if len(products) == 0 {
		t.Fatal("无带 source_product_id 的产品")
	}
	for _, p := range products {
		// 子表路径
		rebuilt, err := repo.ConfigGroupsByProductID(ctx, p.ID)
		if err != nil {
			t.Fatalf("ConfigGroupsByProductID(%d) 失败: %v", p.ID, err)
		}
		builtSet := flattenPairs(t, rebuilt)
		if len(builtSet) == 0 {
			t.Logf("product %d 子表重建为空，跳过", p.ID)
			continue
		}

		// raw_specs 惰性路径
		var rp resourceproductmodel.ResourceProduct
		if err := db.First(&rp, p.SourceProductID).Error; err != nil {
			t.Fatalf("读取 resource_product %d 失败: %v", p.SourceProductID, err)
		}
		lazySet := flattenPairs(t, extractLazyConfigGroups(rp.RawSpecs))
		if len(lazySet) == 0 {
			t.Logf("product %d raw_specs 无 config_groups，跳过", p.ID)
			continue
		}
		t.Logf("product %d 子表=%d 对，惰性=%d 对", p.ID, len(builtSet), len(lazySet))
		for k := range lazySet {
			if _, ok := builtSet[k]; !ok {
				t.Errorf("product %d 缺失候选对: %s", p.ID, k)
			}
		}
		// 反向校验：子表不应额外引入惰性路径不存在的子项（避免画蛇添足）。
		for k := range builtSet {
			if _, ok := lazySet[k]; !ok {
				t.Errorf("product %d 多余候选对: %s", p.ID, k)
			}
		}

		// 写路径验证：用读到的配置组回写 SaveConfigOptions（净零变更），再读比对 upstream_id 对。
		if err := repo.SaveConfigOptions(ctx, p.ID, rebuilt); err != nil {
			t.Fatalf("SaveConfigOptions(%d) 失败: %v", p.ID, err)
		}
		after, err := repo.ConfigGroupsByProductID(ctx, p.ID)
		if err != nil {
			t.Fatalf("SaveConfigOptions 后 ConfigGroupsByProductID(%d) 失败: %v", p.ID, err)
		}
		afterSet := flattenPairs(t, after)
		for k := range afterSet {
			if _, ok := builtSet[k]; !ok {
				t.Errorf("product %d 回写后多余候选对: %s", p.ID, k)
			}
		}
		for k := range builtSet {
			if _, ok := afterSet[k]; !ok {
				t.Errorf("product %d 回写后缺失候选对: %s", p.ID, k)
			}
		}
	}
}

// flattenPairs 提取 (option.upstream_id, sub.upstream_id) 全部候选对集合（key=optID|subID），
// 即 buildCartConfigOption 可消费的候选池。
func flattenPairs(t *testing.T, groups []interface{}) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	if len(groups) == 0 {
		return out
	}
	b, err := json.Marshal(groups)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var parsed []struct {
		Options []struct {
			UpstreamID int64 `json:"upstream_id"`
			Sub        []struct {
				UpstreamID int64 `json:"upstream_id"`
			} `json:"sub"`
		} `json:"options"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	for _, g := range parsed {
		for _, o := range g.Options {
			if o.UpstreamID == 0 {
				continue
			}
			for _, s := range o.Sub {
				if s.UpstreamID == 0 {
					continue
				}
				out[fmt.Sprintf("%d|%d", o.UpstreamID, s.UpstreamID)] = true
			}
		}
	}
	return out
}

// extractLazyConfigGroups 复刻 service.extractConfigGroups。
func extractLazyConfigGroups(rawSpecs string) []interface{} {
	if rawSpecs == "" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(rawSpecs), &m); err != nil {
		return nil
	}
	raw, ok := m["config_groups"]
	if !ok || raw == nil {
		return nil
	}
	if arr, ok := raw.([]interface{}); ok {
		return arr
	}
	return nil
}

func openLiveDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	return db
}
