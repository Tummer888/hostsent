//go:build live

package repository

import (
	"context"
	"testing"
)

// TestLiveSubOrderingMatchesProvisionDefault 履约侧 config_groups 的子项顺序，
// 必须与界面/后端"取首个可见值"的口径一致（sort_order asc, id asc）。
//
// 为什么值得一条 live 测试：
//
//	后台代下单弹窗对"没有声明默认值"的配置项（上游商品的网络类型、操作系统就是如此）
//	按候选顺序预选第一项并显式提交；履约侧对"未提交"的项由 zjmf pickConfigSub
//	取首个可见子项。两处顺序若不一致，界面上显示"VPC 网络"，实际开出的却是经典网络
//	—— 没有报错、没有告警，属于静默的配置错发。
//
// ConfigGroupsByProductID 的 Subs preload 必须显式排序（orderSubsBySort），
// 否则 GORM 不保证返回顺序，主键顺序与 sort_order 不同时就会分叉。
//
// 判定口径：config_groups 里每个多值配置项的首个子项，必须等于
// 「按 sort_order,id 升序的第一个可见子项」。两者都取自同一个上游选项 id，
// 因此直接比对子项的上游 id 即可。
func TestLiveSubOrderingMatchesProvisionDefault(t *testing.T) {
	db := openLiveDB(t)
	ctx := context.Background()
	repo := NewProductRepository(db)

	// 履约首项：按界面同款排序取第一个可见子项，按「商品 + 上游选项id」键化。
	type firstRow struct {
		ProductID  uint64
		OptionKey  int64
		SubSkillID int64
	}
	var firsts []firstRow
	if err := db.Raw(`
		SELECT DISTINCT ON (o.product_id, o.upstream_key)
		       o.product_id AS product_id,
		       o.upstream_key AS option_key,
		       s.upstream_key AS sub_skill_id
		FROM product_config_options o
		JOIN product_config_options_sub s ON s.option_id = o.id
		WHERE o.source = 'upstream' AND s.hidden = 0
		  AND (SELECT count(*) FROM product_config_options_sub x
		       WHERE x.option_id = o.id AND x.hidden = 0) > 1
		ORDER BY o.product_id, o.upstream_key, s.sort_order ASC, s.id ASC`).
		Scan(&firsts).Error; err != nil {
		t.Fatalf("查询履约首项失败: %v", err)
	}
	if len(firsts) == 0 {
		t.Skip("库中没有多值上游配置项，无法验证顺序（测试在这种情况下无意义）")
	}

	// 按商品分组，避免对同一商品重复读配置组。
	byProduct := map[uint64][]firstRow{}
	for _, row := range firsts {
		byProduct[row.ProductID] = append(byProduct[row.ProductID], row)
	}

	checked := 0
	for productID, rows := range byProduct {
		groups, err := repo.ConfigGroupsByProductID(ctx, productID)
		if err != nil {
			t.Fatalf("ConfigGroupsByProductID(%d): %v", productID, err)
		}
		opts := extractGroupOptions(groups)
		want := map[int64]int64{}
		for _, row := range rows {
			want[row.OptionKey] = row.SubSkillID
		}
		for _, opt := range opts {
			expect, ok := want[opt.ID]
			if !ok || len(opt.Sub) == 0 {
				continue
			}
			// configSub.ID / UpstreamID 都是上游取值 id（见 buildConfigGroups 的 id 口径）。
			got := opt.Sub[0].ID
			if got == 0 {
				got = opt.Sub[0].UpstreamID
			}
			checked++
			if got != expect {
				t.Errorf("商品 %d 配置项 %d（%s）子项顺序与默认口径不一致："+
					"config_groups 首项=%d，按 sort_order 的首个可见项=%d"+
					"（会导致「界面显示的默认值」≠「实际开出的值」）",
					productID, opt.ID, opt.OptionName, got, expect)
			}
		}
	}
	// 防呆：必须真的比对过，否则测试是空跑的（这个坑真实踩过：
	// 早前的选样查询挑到了全是单值项的商品，循环体全部 continue，测试恒过）。
	if checked == 0 {
		t.Fatalf("未比对任何多值配置项（候选 %d 个商品 %d 个配置项），测试无效", len(byProduct), len(firsts))
	}
	t.Logf("已比对 %d 个多值配置项的子项顺序", checked)
}

// extractGroupOptions 把 ConfigGroupsByProductID 的返回值取成结构化选项列表。
// 返回元素是同包内的 configGroup 结构体（不是 map），断言失败时会打出来便于排查。
func extractGroupOptions(groups []interface{}) []configOption {
	out := make([]configOption, 0)
	for _, raw := range groups {
		switch g := raw.(type) {
		case configGroup:
			out = append(out, g.Options...)
		case *configGroup:
			if g != nil {
				out = append(out, g.Options...)
			}
		}
	}
	return out
}
