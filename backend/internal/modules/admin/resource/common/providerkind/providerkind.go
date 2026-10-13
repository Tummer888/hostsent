// Package providerkind 提供「按渠道链路（upstream/compute）过滤」的公共查询条件。
//
// 目录形态（CONTRIBUTING §1 例外登记）：resource 是分组式域，本包是域内
// 各子模块（sync/reconcile/taskqueue）共享的纯函数工具包，无 model/表，
// 故不采用五件套形态，挂在 resource/common/ 下供域内复用。
//
// 双链路重构后，同步任务、同步日志、差异记录、调价事件、调度行、出站动作队列、
// 实例对账这些表都只存 provider_id，链路（上游转售 / 自营平台）在 resource_providers.kind 上。
// 各页要按链路分开看，就必须在服务端把过滤下推到 SQL——放到前端筛会把分页口径带偏
// （当前页里恰好属于该链路的几条，而总数仍是两条链路之和）。
//
// 这里集中一份子查询，避免七处各写一遍、日后改口径漏掉某处。
package providerkind

import (
	"strings"

	"gorm.io/gorm"
)

// 链路取值（与 resource/provider/model、pkg/upstream 的 Kind* 常量同值）。
const (
	Upstream = "upstream" // 上游转售
	Compute  = "compute"  // 自营平台（算力平台，我方执行器）
)

// kindSubquery 渠道链路子查询。
//
// 带 deleted_at IS NULL：resource_providers 是软删除表，已删渠道不该再把历史行
// 捞进任何链路视图（否则「自营」列表里会冒出早已删除的渠道的历史任务）。
const kindSubquery = `SELECT id FROM resource_providers WHERE kind = ? AND deleted_at IS NULL`

// Apply 给 GORM 查询追加链路过滤（空 kind 表示不过滤，原样返回）。
func Apply(db *gorm.DB, kind string) *gorm.DB {
	kind = Normalize(kind)
	if kind == "" {
		return db
	}
	return db.Where("provider_id IN ("+kindSubquery+")", kind)
}

// Condition 返回裸 SQL 条件片段与参数（供手写 SQL 的仓储使用）。
// column 为所在表里指向渠道的列名；空/非法 kind 返回空串，调用方直接跳过拼接。
func Condition(kind, column string) (string, []any) {
	kind = Normalize(kind)
	if kind == "" {
		return "", nil
	}
	if column == "" {
		column = "provider_id"
	}
	return column + " IN (" + kindSubquery + ")", []any{kind}
}

// Normalize 归一链路入参：非 upstream/compute 一律视为「不过滤」。
//
// 前端传空、传错值时不应被理解成「只看某个不存在的链路」而返回空列表——
// 静默空列表比报错更难排查。
func Normalize(kind string) string {
	switch strings.TrimSpace(kind) {
	case Upstream, Compute:
		return strings.TrimSpace(kind)
	default:
		return ""
	}
}
