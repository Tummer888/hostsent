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

	"hostsent/backend/internal/modules/admin/resource/common/providerkind"
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
        -- 暂停原因分类（迁移 114）：界面据此把「违规暂停」与到期/手动暂停分开，
        -- 对账文案里"本地未暂停"的措辞也依赖它。
        COALESCE(i.suspend_reason, '') AS suspend_reason,
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
        -- 上游状态口径：raw_data.domainstatus 是魔方财务（zjmf）的**服务状态**字段
        -- （Active/Pending/Suspended/Deleted...），可能由「上游推送」写入（push_service
        -- 回写 raw_data）或由定时同步拉取写入。非 zjmf 上游没有该字段，此时
        -- status_anomaly 恒为 ok（不做状态比对，而不是错判成异常——把"没这个概念"
        -- 当"数据缺失"会天天误报）。
        --
        -- **维度提醒（第二十一轮修复的正是这里）**：domainstatus 是服务维度，
        -- 它不是机器电源状态；zjmf 没有电源字段（见 pkg/upstream/capability.go 的
        -- ReportsPowerState=false）。客户在控制台点一次关机（func=off）后，
        -- domainstatus 仍然是 Active —— 服务还在、账单还在，只是机器没通电。
        -- 所以它只能与本地**服务维度**（lifecycle_stage）比对，绝不能拿本地
        -- status（机器维度：running/stopped）去机械地要求相等：
        -- 那会把每一次正常关机都报成"状态不同步"（用户实测误报）。
        CASE WHEN pg_input_is_valid(i.raw_data, 'jsonb')
             THEN i.raw_data::jsonb ->> 'domainstatus' END AS raw_domainstatus,
        i.expire_at
    FROM instances i
    LEFT JOIN products p ON p.id = i.sell_product_id
    LEFT JOIN resource_products rp ON rp.id = i.upstream_product_id
    WHERE i.source_mode = 'upstream'
      -- 终态行没有可对账的上游对象：destroyed 的机器已终止，cancelled 的
      -- 机器从未开通（订单未付款超时）。留着它们只会产生永远无法收敛的
      -- "缺到期/缺成本"预警，把真正要对账的行淹掉。
      AND COALESCE(i.lifecycle_stage, '') NOT IN ('destroyed', 'cancelled')
      -- 待开通的占位行（还没有上游实例号）同样没有比对的另一侧。
      -- 对账页的定位就是"已开通并对接上游的实例"（见页面说明）。
      AND COALESCE(i.provider_instance_id, '') <> ''
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
        END AS upstream_expire,
        -- 本地**服务维度**（与上游 domainstatus 可比的口径）。
        --
        -- 单独算一次而不是在判定里逐个字段现拼：上游 domainstatus 是服务状态，
        -- 本地能与之对应的只有 lifecycle_stage + suspend_reason（服务是否在用、
        -- 是否被停），**不是** status（机器通没通电）。把本地口径先归一成一个
        -- 取值，判定与展示、台账三条路径都读同一个值，避免三处各写一份又分叉。
        --
        -- 取值含义：
        --   suspended    —— 服务被停（阶段为暂停，或落了暂停原因）
        --   destroyed    —— 已终止（阶段 destroyed 或 status=deleted）
        --   cancelled    —— 从未开通（订单未付款超时，服务不存在）
        --   grace        —— 已过期但在宽限期，机器仍可服务
        --   provisioning —— 开通中（上游主机还在创建）
        --   active       —— 其余：服务在用（含历史行 stage 为空但一直在服务的情况）
        CASE
            WHEN COALESCE(n.lifecycle_stage, '') = 'suspended' THEN 'suspended'
            WHEN COALESCE(n.suspend_reason, '') <> '' THEN 'suspended'
            WHEN COALESCE(n.lifecycle_stage, '') = 'destroyed' THEN 'destroyed'
            WHEN n.status = 'deleted' THEN 'destroyed'
            WHEN COALESCE(n.lifecycle_stage, '') = 'cancelled' THEN 'cancelled'
            WHEN n.status = 'cancelled' THEN 'cancelled'
            WHEN COALESCE(n.lifecycle_stage, '') = 'grace' THEN 'grace'
            WHEN n.status = 'creating' THEN 'provisioning'
            ELSE 'active'
        END AS local_service_state
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
             THEN EXTRACT(EPOCH FROM (c.upstream_expire - c.expire_at)) / 86400.0 END AS expire_diff_days,
        -- 状态不同步判据：上游 domainstatus（服务维度） vs 本地 local_service_state
        -- （同为服务维度，见 calc 注释）。**两侧都不含机器电源状态** ——
        -- 客户关一次机（本地 status=stopped、上游 domainstatus 仍 Active）不是差异，
        -- 那台机器只是没通电，服务与账单都还在（用户实测误报的根因）。
        --
        -- 没有 domainstatus 的上游一律 ok：不做比对与"比对不出结果"是两回事。
        --
        -- status_stale（严重）：上游已终止而本地还在服务 → 在为不存在的机器计费。
        -- status_mismatch（预警）：其余两侧对不上号的组合，方向都是"客户的机器还能用，
        --                          但我方账/服务状态没跟上"或反之。
        CASE
            WHEN c.raw_domainstatus IS NULL OR c.raw_domainstatus = '' THEN 'ok'
            -- 上游已终止（删除/取消/终止）而本地还在服务：销毁流程没走完。
            WHEN c.raw_domainstatus IN ('Deleted', 'Cancelled', 'Terminated')
                 AND c.local_service_state NOT IN ('destroyed', 'cancelled') THEN 'status_stale'
            -- 上游暂停（含风控冻结）而本地还在服务：我方还在给客户算钱，
            -- 但机器在上游已被停 —— 客户实际用不了。
            WHEN c.raw_domainstatus IN ('Suspended', 'Fraud')
                 AND c.local_service_state NOT IN ('suspended', 'destroyed', 'cancelled') THEN 'status_mismatch'
            -- 上游在服务而本地已暂停：本地暂停没同步到上游（或上游被谁恢复了）。
            -- 宽限期（grace）与开通中（provisioning）都算"未暂停"，不报：
            -- 前者机器仍可服务，后者上游常仍显示 Active（主机已就绪但账单未结）。
            WHEN c.raw_domainstatus = 'Active'
                 AND c.local_service_state = 'suspended' THEN 'status_mismatch'
            -- 上游还在开通中（Pending）而本地已视为在用：开通流程没走完。
            WHEN c.raw_domainstatus = 'Pending'
                 AND c.local_service_state IN ('active', 'grace') THEN 'status_mismatch'
            ELSE 'ok'
        END AS status_anomaly
    FROM calc c
)
`, ThinMarginRate, ExpireToleranceDays)
}

// severityExpr 异常等级：资金风险最高（低于成本 / 到期错位 / 上游已删本地仍有效），
// 其次数据缺失、薄利与状态漂移。
const severityExpr = `
CASE
    WHEN price_anomaly = 'below_cost'
      OR expire_anomaly IN ('expire_early', 'expire_late')
      OR status_anomaly = 'status_stale' THEN 'danger'
    WHEN price_anomaly <> 'ok' OR expire_anomaly <> 'ok' OR status_anomaly <> 'ok' THEN 'warning'
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
	// StatusAnomaly 状态不同步判据（ok/status_mismatch/status_stale）。
	StatusAnomaly  string
	UpstreamStatus string
	// LocalServiceState 本地服务维度归一值（active/grace/provisioning/suspended/
	// destroyed/cancelled，见 baseCTE 的 calc 注释）。状态比对与文案都以它为准，
	// **不要**退回 Status（那是机器电源维度：running/stopped）—— 两者不可混，
	// 混比会把每一次正常关机报成"状态不同步"（第二十一轮修复的误报）。
	LocalServiceState string
	LocalExpire       *time.Time
	UpstreamExpire    *time.Time
	Severity          string
}

// SummaryRow 对账汇总（单行聚合结果）。
type SummaryRow struct {
	Total         int64
	Danger        int64
	Warning       int64
	OK            int64
	BelowCost     int64
	ThinMargin    int64
	MissingPrice  int64
	ExpireEarly   int64
	ExpireLate    int64
	MissingExpire int64
	// StatusMismatch / StatusStale 状态不同步计数。
	StatusMismatch int64
	StatusStale    int64
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
	// ProviderMeta 批量取渠道名与链路（列表装饰用；kind 供前端打链路标签）。
	ProviderMeta(ctx context.Context, ids []uint64) (map[uint64]ProviderMeta, error)
}

// ProviderMeta 渠道展示元信息。
type ProviderMeta struct {
	Name string
	Kind string
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
	// 链路过滤（upstream/compute）：口径集中一份，见 providerkind 包。
	if cond, condArgs := providerkind.Condition(query.ProviderKind, "provider_id"); cond != "" {
		conds = append(conds, cond)
		args = append(args, condArgs...)
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
	case "status_mismatch":
		conds = append(conds, "status_anomaly = 'status_mismatch'")
	case "status_stale":
		conds = append(conds, "status_anomaly = 'status_stale'")
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
               status_anomaly, COALESCE(raw_domainstatus, '') AS upstream_status,
               local_service_state,
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
            COUNT(*) FILTER (WHERE status_anomaly = 'status_mismatch')          AS status_mismatch,
            COUNT(*) FILTER (WHERE status_anomaly = 'status_stale')             AS status_stale,
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

func (r *repository) ProviderMeta(ctx context.Context, ids []uint64) (map[uint64]ProviderMeta, error) {
	out := make(map[uint64]ProviderMeta, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   uint64
		Name string
		Kind string
	}
	if err := r.db.WithContext(ctx).Table("resource_providers").
		Select("id, name, kind").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = ProviderMeta{Name: row.Name, Kind: row.Kind}
	}
	return out, nil
}
