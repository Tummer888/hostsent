// Package repository 提供任务队列的只读聚合查询。
//
// 任务队列不新增表：把已有的三张"动作表"按统一列形状 UNION ALL 成一个虚拟队列，
// 再由外层做筛选 / 分页 / 汇总（迁移 036 已为三表补 created_at 索引）。
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/resource/taskqueue/dto"
)

// TaskRow 归一后的队列行（各来源表 → 统一列形状）。
type TaskRow struct {
	Category      string
	RefID         uint64
	Action        string
	Status        string
	Subject       string
	RefNo         string
	InstanceRef   string
	Detail        string
	ProviderID    uint64
	UserID        uint64
	Attempts      int
	MaxAttempts   int
	Amount        float64
	CreatedAt     time.Time
	FinishedAt    *time.Time
	UpstreamState string
}

// CategoryCount 单类别计数。
type CategoryCount struct {
	Category   string
	Total      int64
	NotReached int64
}

// StatusCount 原始状态计数。
type StatusCount struct {
	Status string
	Total  int64
}

// StateCount 上游到达状态计数。
type StateCount struct {
	UpstreamState string
	Total         int64
}

// Repository 任务队列仓储接口。
type Repository interface {
	List(ctx context.Context, query dto.TaskQueueListQuery, page, pageSize int) ([]TaskRow, int64, error)
	StatusCounts(ctx context.Context, query dto.TaskQueueListQuery) ([]StatusCount, error)
	StateCounts(ctx context.Context, query dto.TaskQueueListQuery) ([]StateCount, error)
	CategoryCounts(ctx context.Context, query dto.TaskQueueListQuery) ([]CategoryCount, error)
	// Usernames 批量取用户名（列表装饰用，避免每行查库）。
	Usernames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// ProviderNames 批量取渠道名（列表装饰用）。
	ProviderNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository 创建任务队列仓储。
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// queueCTE 把三类平台动作归一为统一列形状。
//
// 归一字段：category / ref_id / action / status / subject / ref_no / instance_ref /
// detail / provider_id / user_id / attempts / max_attempts / amount / created_at /
// finished_at / upstream_state。
//
// upstream_state 是「是否到达上游」的唯一判据，按各类别语义在 SQL 中直接算出，
// 保证筛选与展示同源：
//   - provision：success → reached；failed/manual → not_reached（未到达或上游拒绝）。
//   - instance_action：result=success → reached，其余 not_reached。
//   - renewal：pending → pending；cancelled / sync_state=local_only → not_applicable
//     （链路 B 无上游续费接口，本地账期顺延，不算未到达）；
//     sync_state=upstream_ok → reached；sync_state=failed 或 status=failed → not_reached。
//   - sync：上游同步任务的 success → reached，failed → not_reached，其余 pending。
//     sync_tasks 本身即「平台调用上游接口」的记录，故以任务成败直接判定到达与否。
const queueCTE = `
WITH queue AS (
    SELECT
        'provision'::text AS category,
        pt.id AS ref_id,
        'provision'::text AS action,
        pt.status AS status,
        COALESCE(NULLIF(p.name, ''), '商品 #' || pt.product_id::text) AS subject,
        COALESCE(o.order_no, '') AS ref_no,
        COALESCE(i.instance_id, '') AS instance_ref,
        COALESCE(pt.last_error, '') AS detail,
        COALESCE(i.provider_id, 0) AS provider_id,
        pt.user_id AS user_id,
        pt.attempts AS attempts,
        pt.max_attempts AS max_attempts,
        COALESCE(o.final_amount, 0)::float8 AS amount,
        pt.created_at AS created_at,
        CASE WHEN pt.status IN ('success', 'failed', 'manual') THEN pt.completed_at ELSE NULL END AS finished_at,
        CASE pt.status
            WHEN 'success' THEN 'reached'
            WHEN 'failed' THEN 'not_reached'
            WHEN 'manual' THEN 'not_reached'
            WHEN 'skipped' THEN 'skipped'
            ELSE 'pending'
        END::text AS upstream_state
    FROM provision_tasks pt
    LEFT JOIN orders o ON o.id = pt.order_id
    LEFT JOIN products p ON p.id = pt.product_id
    LEFT JOIN LATERAL (
        SELECT instance_id, provider_id FROM instances WHERE order_id = pt.order_id ORDER BY id LIMIT 1
    ) i ON TRUE

    UNION ALL

    SELECT
        'instance_action'::text,
        op.id,
        op.action,
        op.result,
        COALESCE(NULLIF(i.name, ''), NULLIF(op.instance_mark, ''), '实例 #' || op.instance_id::text),
        ''::text,
        COALESCE(NULLIF(op.instance_mark, ''), COALESCE(i.instance_id, '')),
        COALESCE(op.error_message, ''),
        COALESCE(i.provider_id, 0),
        op.user_id,
        0,
        0,
        0::float8,
        op.created_at,
        op.created_at,
        CASE WHEN op.result = 'success' THEN 'reached' ELSE 'not_reached' END::text
    FROM instance_operations op
    LEFT JOIN instances i ON i.id = op.instance_id
    WHERE op.action IN ('power_on', 'power_off', 'hard_off', 'reboot', 'hard_reboot',
                        'sync', 'resize', 'destroy', 'suspend', 'unsuspend')

    UNION ALL

    SELECT
        'renewal'::text,
        r.id,
        'renew'::text,
        r.status,
        COALESCE(NULLIF(r.product_name, ''), '实例 #' || r.instance_id::text),
        r.renewal_no,
        COALESCE(NULLIF(r.instance_mark, ''), COALESCE(i.instance_id, '')),
        COALESCE(NULLIF(r.fail_reason, ''), COALESCE(r.upstream_order_id, '')),
        COALESCE(i.provider_id, 0),
        r.user_id,
        0,
        0,
        r.amount::float8,
        r.created_at,
        CASE WHEN r.status IN ('success', 'failed', 'cancelled') THEN r.updated_at ELSE NULL END,
        CASE
            WHEN r.status = 'pending' THEN 'pending'
            WHEN r.status = 'cancelled' THEN 'not_applicable'
            WHEN r.sync_state = 'local_only' THEN 'not_applicable'
            WHEN r.sync_state = 'upstream_ok' THEN 'reached'
            WHEN r.sync_state = 'failed' THEN 'not_reached'
            WHEN r.status = 'success' THEN 'reached'
            ELSE 'not_reached'
        END::text
    FROM instance_renewals r
    LEFT JOIN instances i ON i.id = r.instance_id
    UNION ALL

    SELECT
        'sync'::text,
        st.id,
        COALESCE(NULLIF(st.task_type, ''), 'catalog'),
        COALESCE(NULLIF(st.status, ''), 'pending'),
        COALESCE(NULLIF(pr.name, ''), '渠道 #' || st.provider_id::text),
        ''::text,
        ''::text,
        COALESCE(st.error_message, ''),
        st.provider_id,
        0,
        0,
        0,
        0::float8,
        st.created_at,
        CASE WHEN st.status IN ('success', 'failed') THEN COALESCE(st.completed_at, st.created_at) ELSE NULL END,
        CASE
            WHEN st.status = 'success' THEN 'reached'
            WHEN st.status = 'failed' THEN 'not_reached'
            ELSE 'pending'
        END::text
    FROM sync_tasks st
    LEFT JOIN resource_providers pr ON pr.id = st.provider_id
)`

// buildWhere 组装外层筛选条件。includeCategory=false 用于页签计数（不受类别筛选影响）。
func buildWhere(query dto.TaskQueueListQuery, includeCategory bool) (string, []any) {
	var conds []string
	var args []any

	if includeCategory && query.Category != "" {
		conds = append(conds, "category = ?")
		args = append(args, query.Category)
	}
	if query.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, query.Status)
	}
	if query.UpstreamState != "" {
		conds = append(conds, "upstream_state = ?")
		args = append(args, query.UpstreamState)
	}
	if query.ProviderID > 0 {
		conds = append(conds, "provider_id = ?")
		args = append(args, query.ProviderID)
	}
	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		conds = append(conds, "(subject ILIKE ? OR ref_no ILIKE ? OR instance_ref ILIKE ? OR detail ILIKE ?)")
		args = append(args, like, like, like, like)
	}
	if from, _, ok := parseTimeParam(query.CreatedFrom); ok {
		conds = append(conds, "created_at >= ?")
		args = append(args, from)
	}
	if to, dateOnly, ok := parseTimeParam(query.CreatedTo); ok {
		if dateOnly {
			to = to.Add(24*time.Hour - time.Second)
		}
		conds = append(conds, "created_at <= ?")
		args = append(args, to)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(conds, " AND "), args
}

// parseTimeParam 解析时间筛选：支持 RFC3339、YYYY-MM-DD 与 DB 时间戳（YYYY-MM-DD HH:MM:SS）。
//
// isDateOnly 为 true 时表示入参只给到日期，调用方据此把区间终点顺延到当天 23:59:59，
// 否则「结束日期=今天」会退化成「今天 00:00:00 之前」而漏掉当天数据。
func parseTimeParam(raw string) (t time.Time, isDateOnly bool, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false, false
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, false, true
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, true, true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local); err == nil {
		return t, false, true
	}
	return time.Time{}, false, false
}

func (r *repository) List(ctx context.Context, query dto.TaskQueueListQuery, page, pageSize int) ([]TaskRow, int64, error) {
	where, args := buildWhere(query, true)

	var total int64
	countSQL := queueCTE + " SELECT COUNT(*) FROM queue WHERE 1=1" + where
	if err := r.db.WithContext(ctx).Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	listSQL := queueCTE + `
        SELECT category, ref_id, action, status, subject, ref_no, instance_ref, detail,
               provider_id, user_id, attempts, max_attempts, amount, created_at, finished_at, upstream_state
          FROM queue
         WHERE 1=1` + where + `
         ORDER BY created_at DESC, ref_id DESC
         LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)

	var rows []TaskRow
	if err := r.db.WithContext(ctx).Raw(listSQL, listArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *repository) StatusCounts(ctx context.Context, query dto.TaskQueueListQuery) ([]StatusCount, error) {
	where, args := buildWhere(query, true)
	sql := queueCTE + " SELECT status, COUNT(*) AS total FROM queue WHERE 1=1" + where + " GROUP BY status"
	var rows []StatusCount
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) StateCounts(ctx context.Context, query dto.TaskQueueListQuery) ([]StateCount, error) {
	where, args := buildWhere(query, true)
	sql := queueCTE + " SELECT upstream_state, COUNT(*) AS total FROM queue WHERE 1=1" + where + " GROUP BY upstream_state"
	var rows []StateCount
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CategoryCounts 各类别计数：刻意不带 category 筛选，供页签角标始终显示全量。
func (r *repository) CategoryCounts(ctx context.Context, query dto.TaskQueueListQuery) ([]CategoryCount, error) {
	where, args := buildWhere(query, false)
	sql := queueCTE + `
        SELECT category,
               COUNT(*) AS total,
               COUNT(*) FILTER (WHERE upstream_state = 'not_reached') AS not_reached
          FROM queue WHERE 1=1` + where + ` GROUP BY category`
	var rows []CategoryCount
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
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
