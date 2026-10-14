// 退款策略的跨域适配器：把商品目录与商品分类的能力投影成订单域声明的最小端口。
//
// CONTRIBUTING §3：admin/order 不 import 商品模块；转换职责落在装配层。
package server

import (
	"context"

	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	categoryservice "hostsent/backend/internal/modules/admin/product/category/service"
)

// productPolicyInfoReader 提供商品来源与分类（退款策略匹配的输入维度）。
type productPolicyInfoReader struct {
	svc catalogservice.ProductService
}

func (r productPolicyInfoReader) ProductPolicyInfo(ctx context.Context, productID uint64) (string, uint64, error) {
	product, err := r.svc.FindByID(ctx, productID)
	if err != nil {
		return "", 0, err
	}
	if product == nil {
		return "", 0, nil
	}
	return product.SourceMode, product.CategoryID, nil
}

// categoryChainReader 提供分类祖先链（策略按「具体分类 → 上级分类」逐级匹配）。
//
// 自底向上收集，越靠前越具体（商品自己的分类优先于其上级）：
// 例如「云主机/香港」的策略应覆盖「云主机」的全局口径。
type categoryChainReader struct {
	svc categoryservice.CategoryService
}

func (r categoryChainReader) CategoryChain(ctx context.Context, categoryID uint64) ([]uint64, error) {
	chain := make([]uint64, 0, 4)
	seen := map[uint64]bool{}
	cur := categoryID
	// 上限兜底：分类树理论不限深，但脏数据可能构成环（A 的父是 B、B 的父是 A），
	// 加个步数上限避免死循环拖垮请求。
	for steps := 0; cur != 0 && steps < 32; steps++ {
		if seen[cur] {
			break
		}
		seen[cur] = true
		chain = append(chain, cur)
		info, err := r.svc.FindByID(ctx, cur)
		if err != nil || info == nil {
			break
		}
		cur = info.ParentID
	}
	return chain, nil
}
