// Package repository 提供实例对账的只读比对查询。
//
// 对账不新增表：直接以 instances 为事实表，左连本地售出商品（products，取售价）
// 与上游资源商品（resource_products，取成本价），并从 instances.raw_data（上游原始 JSON）
// 透取 nextduedate / upstream_cost 作为上游侧的权威口径。
//
// 价格与到期的异常判定全部在 SQL 内一次算出（marked CTE），筛选与汇总都基于同一份结果，
// 避免「列表按 A 判、汇总按 B 判」的口径分叉。
package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/resource/reconcile/dto"
)

// 对账判据阈值（单一来源：既用于拼 SQL，也回传给前端展示口径）。
const (
	// ExpireToleranceDays 到期时间容差：相差在 1 天以内视为一致（上游时间戳取整带来的误差）。
	ExpireToleranceDays = 1
	// ThinMarginRate 毛利预警阈值（%）：毛利率低于该值即计入预警，与「低于成本」区分。
	ThinMarginRate = 5.0
)

// baseCTE 逐层收敛：base（取数）→ norm（原始字符串安全转数值）→ calc（算上游成本与到期）
// → marked（判定异常与等级）。
//
// 上游到期口径说明：instances.expire_at 是本地账期，正是被比对的本地值；
// 上游到期只能来自 raw_data.nextduedate。该字段为 Unix 时间戳，
// 魔方财务返回秒级（10 位），个别上游会返回毫秒级（13 位），故按量级自动换算。
var baseCTE = buildBaseCTE()

func buildBaseCTE() string {
	return fmt.Sprintf(`
WITH base AS (
    SELECT
        i.id,
        i.instance_id,
        i.name,
        COALESCE(NULLIF(i.status, ''), 'unknown') AS status,
        COALESCE(i.lifecycle_stage, '') AS lifecycle_stage,
        i.provider_id,
        i.user_id,
        COALESCE(i.sell_product_id, 0) AS sell_product_id,
        COALESCE(p.name, '') AS sell_product_name,
        p.price AS local_price,
        COALESCE(i.upstream_product_id, 0) AS upstream_product_id,
        COALESCE(rp.name, '') AS upstream_product_name,
        COALESCE(rp.upstream_id, '') AS upstream_sku,
        rp.cost_price AS resource_cost,
        CASE WHEN pg_input_is_valid(i.raw_data, 'jsonb')
             THEN i.raw_data::jsonb ->> 'nextduedate' END AS raw_nextduedate,
        CASE WHEN pg_input_is_valid(i.raw_data, 'jsonb')
             THEN i.raw_data::jsonb ->> 'upstream_cost' END AS raw_upstream_cost,
        i.expire_at
    FROM instances i
    LEFT JOIN products p ON p.id = i.sell_product_id
    LEFT JOIN resource_products rp ON rp.id = i.upstream_product_id
    WHERE i.source_mode = 'upstream'
      AND COALESCE(i.lifecycle_stage, '') <> 'destroyed'
),
norm AS (
    SELECT
        b.*,
        -- pg_input_is_valid 校验后再转型：raw_data 来自上游，可能为空串或非数值，
        -- 直接 ::numeric 会整条查询报错。
        -- 注意：此处不可用正则写法——正则里的问号会被 gorm 当成绑定占位符，导致参数错位。
        CASE WHEN pg_input_is_valid(COALESCE(b.raw_upstream_cost, ''), 'numeric')
             THEN b.raw_upstream_cost::numeric END AS raw_cost,
        CASE WHEN pg_input_is_valid(COALESCE(b.raw_nextduedate, ''), 'bigint')
             THEN b.raw_nextduedate::bigint END AS raw_nextdue
    FROM base b
),
calc AS (
    SELECT
        n.*,
        COALESCE(n.raw_cost, n.resource_cost) AS upstream_cost,
        CASE
            WHEN n.raw_nextdue IS NULL THEN NULL
            WHEN n.raw_nextdue > 100000000000 THEN to_timestamp(n.raw_nextdue / 1000.0)
            ELSE to_timestamp(n.raw_nextdue)
        END AS upstream_expire
    FROM norm n
),
marked AS (
    SELECT
        c.*,
        c.local_price - c.upstream_cost AS margin_amount,
        CASE WHEN c.upstream_cost > 0
             THEN (c.local_price - c.upstream_cost) / c.upstream_cost * 100 END AS margin_rate,
        CASE
            WHEN c.local_price IS NULL THEN 'missing_local_price'
            WHEN c.upstream_cost IS NULL THEN 'missing_cost'
            WHEN c.local_price < c.upstream_cost THEN 'below_cost'
            WHEN c.upstream_cost > 0
                 AND (c.local_price - c.upstream_cost) / c.upstream_cost * 100 < %[1]g THEN 'thin_margin'
            ELSE 'ok'
        END AS price_anomaly,
        CASE
            WHEN c.expire_at IS NULL THEN 'missing_local_expire'
            WHEN c.upstream_expire IS NULL THEN 'missing_upstream_expire'
            WHEN c.upstream_expire - c.expire_at > make_interval(days => %[2]d) THEN 'expire_early'
            WHEN c.expire_at - c.upstream_expire > make_interval(days => %[2]d) THEN 'expire_late'
            ELSE 'ok'
        END AS expire_anomaly,
        CASE WHEN c.upstream_expire IS NOT NULL AND c.expire_at IS NOT NULL
             THEN EXTRACT(EPOCH FROM (c.upstream_expire - c.expire_at)) / 86400.0 END AS expire_diff_days
    FROM calc c
)
`, ThinMarginRate, ExpireToleranceDays)
}

// severityExpr 异常等级：资金风险最高（低于成本 / 到期错位），其次数据缺失与薄利。
const severityExpr = `
CASE
    WHEN price_anomaly = 'below_cost' OR expire_anomaly IN ('expire_early', 'expire_late') THEN 'danger'
    WHEN price_anomaly <> 'ok' OR expire_anomaly <> 'ok' THEN 'warning'
    ELSE 'ok'
END`

// ReconcileRow 对账行（SQL 结果直接映射）。
type ReconcileRow struct {
	ID                  uint64
	InstanceID          string
	Name                string
	Status              string
	LifecycleStage      string
	ProviderID          uint64
	UserID              uint64
	SellProductID       uint64
	SellProductName     string
	LocalPrice          *float64
	UpstreamProductID   uint64
	UpstreamProductName string
	UpstreamSKU         string
	UpstreamCost        *float64
	MarginAmount        *float64
	MarginRate          *float64
	PriceAnomaly        string
	ExpireAnomaly       string
	ExpireDiffDays      *float64
	LocalExpire         *time.Time
	UpstreamExpire      *time.Time
	Severity            string
}

// SummaryRow 对账汇总（单行聚合结果）。
type SummaryRow struct {
	Total          int64
	Danger         int64
	Warning        int64
	OK             int64
	BelowCost      int64
	ThinMargin     int64
	MissingPrice   int64
	ExpireEarly    int64
	ExpireLate     int64
	MissingExpire  int64
	NegativeMargin int64
	TotalMargin    *float64
	AvgMarginRate  *float64
}

// Repository 实例对账仓储接口。
type Repository interface {
	List(ctx context.Context, query dto.ReconcileListQuery, page, pageSize int) ([]ReconcileRow, int64, error)
	Summary(ctx context.Context, query dto.ReconcileListQuery) (*SummaryRow, error)
	// Usernames 批量取用户名（列表装饰用）。
	Usernames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// ProviderNames 批量取渠道名（列表装饰用）。
	ProviderNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository 创建实例对账仓储。
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// buildWhere 组装筛选条件。
func buildWhere(query dto.ReconcileListQuery) (string, []any) {
	var conds []string
	var args []any

	if query.ProviderID > 0 {
		conds = append(conds, "provider_id = ?")
		args = append(args, query.ProviderID)
	}
	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		conds = append(conds, `(instance_id ILIKE ? OR name ILIKE ? OR sell_product_name ILIKE ? OR upstream_product_name ILIKE ? OR upstream_sku ILIKE ?)`)
		args = append(args, like, like, like, like, like)
	}
	switch strings.TrimSpace(query.Anomaly) {
	case "danger":
		conds = append(conds, "("+severityExpr+" = 'danger')")
	case "warning":
		conds = append(conds, "("+severityExpr+" = 'warning')")
	case "ok":
		conds = append(conds, "("+severityExpr+" = 'ok')")
	case "below_cost":
		conds = append(conds, "price_anomaly = 'below_cost'")
	case "thin_margin":
		conds = append(conds, "price_anomaly = 'thin_margin'")
	case "expire_early":
		conds = append(conds, "expire_anomaly = 'expire_early'")
	case "expire_late":
		conds = append(conds, "expire_anomaly = 'expire_late'")
	case "missing":
		conds = append(conds, `(price_anomaly IN ('missing_local_price', 'missing_cost')
		                        OR expire_anomaly IN ('missing_local_expire', 'missing_upstream_expire'))`)
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(conds, " AND "), args
}

func (r *repository) List(ctx context.Context, query dto.ReconcileListQuery, page, pageSize int) ([]ReconcileRow, int64, error) {
	where, args := buildWhere(query)

	var total int64
	countSQL := baseCTE + " SELECT COUNT(*) FROM marked WHERE 1=1" + where
	if err := r.db.WithContext(ctx).Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	listSQL := baseCTE + `
        SELECT id, instance_id, name, status, lifecycle_stage, provider_id, user_id,
               sell_product_id, sell_product_name, local_price,
               upstream_product_id, upstream_product_name, upstream_sku, upstream_cost,
               margin_amount, margin_rate, price_anomaly, expire_anomaly, expire_diff_days,
               expire_at AS local_expire, upstream_expire,
               ` + severityExpr + ` AS severity
          FROM marked
         WHERE 1=1` + where + `
         ORDER BY CASE ` + severityExpr + ` WHEN 'danger' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END,
                  margin_rate ASC NULLS LAST,
                  id DESC
         LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)

	var rows []ReconcileRow
	if err := r.db.WithContext(ctx).Raw(listSQL, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *repository) Summary(ctx context.Context, query dto.ReconcileListQuery) (*SummaryRow, error) {
	where, args := buildWhere(query)
	sql := baseCTE + `
        SELECT
            COUNT(*) FILTER (WHERE TRUE)                                        AS total,
            COUNT(*) FILTER (WHERE ` + severityExpr + ` = 'danger')             AS danger,
            COUNT(*) FILTER (WHERE ` + severityExpr + ` = 'warning')            AS warning,
            COUNT(*) FILTER (WHERE ` + severityExpr + ` = 'ok')                 AS ok,
            COUNT(*) FILTER (WHERE price_anomaly = 'below_cost')                AS below_cost,
            COUNT(*) FILTER (WHERE price_anomaly = 'thin_margin')               AS thin_margin,
            COUNT(*) FILTER (WHERE price_anomaly IN ('missing_local_price', 'missing_cost')) AS missing_price,
            COUNT(*) FILTER (WHERE expire_anomaly = 'expire_early')             AS expire_early,
            COUNT(*) FILTER (WHERE expire_anomaly = 'expire_late')              AS expire_late,
            COUNT(*) FILTER (WHERE expire_anomaly IN ('missing_local_expire', 'missing_upstream_expire')) AS missing_expire,
            COUNT(*) FILTER (WHERE margin_amount < 0)                           AS negative_margin,
            SUM(margin_amount)                                                  AS total_margin,
            AVG(margin_rate)                                                    AS avg_margin_rate
          FROM marked
         WHERE 1=1` + where

	var row SummaryRow
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *repository) Usernames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID       uint64
		Username string
	}
	if err := r.db.WithContext(ctx).Table("users").
		Select("id, username").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Username
	}
	return out, nil
}

func (r *repository) ProviderNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   uint64
		Name string
	}
	if err := r.db.WithContext(ctx).Table("resource_providers").
		Select("id, name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}
