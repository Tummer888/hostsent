// Package service 提供任务队列业务编排：聚合四类平台动作，
// 并给每行标注「是否到达上游」与队列汇总。
package service

import (
	"context"
	"errors"
	"time"

	"hostsent/backend/internal/modules/admin/resource/taskqueue/dto"
	"hostsent/backend/internal/modules/admin/resource/taskqueue/repository"
)

// Repository 任务队列仓储能力（便于装配层注入与测试替身）。
type Repository = repository.Repository

// TaskQueueService 任务队列业务能力。
type TaskQueueService interface {
	List(ctx context.Context, query dto.TaskQueueListQuery) (*dto.TaskQueueListResponse, error)
	// Categories 返回类别元信息与全量计数（无筛选），供前端页签渲染。
	Categories(ctx context.Context) ([]dto.TaskQueueCategoryCount, error)
}

type taskQueueService struct {
	repo repository.Repository
}

// NewTaskQueueService 创建任务队列服务。
func NewTaskQueueService(repo repository.Repository) TaskQueueService {
	return &taskQueueService{repo: repo}
}

// categoryNames 类别展示名。
var categoryNames = map[string]string{
	dto.CategoryProvision:      "开通履约",
	dto.CategoryInstanceAction: "实例动作",
	dto.CategoryRenewal:        "续费",
	dto.CategorySync:           "上游同步",
}

// categoryOrder 页签固定顺序，与前端展示顺序一致。
var categoryOrder = []string{
	dto.CategoryProvision,
	dto.CategoryInstanceAction,
	dto.CategoryRenewal,
	dto.CategorySync,
}

// actionNames 动作展示名。同步类动作码为 sync_tasks.task_type。
var actionNames = map[string]string{
	"provision": "开通实例",
	"renew":     "续费",
	"power_on":  "开机",
	"power_off": "关机",
	"hard_off":  "强制关机",
	"reboot":    "重启",
	// hard_reboot 上游语义等同重启，展示名统一，避免运营区分两套术语。
	"hard_reboot": "重启",
	"resize":      "变配",
	"destroy":     "销毁",
	"suspend":     "暂停",
	"unsuspend":   "恢复",
	"sync":        "回源刷新",
	// 上游同步 scope
	"catalog":  "商品目录同步",
	"product":  "商品同步",
	"price":    "价格同步",
	"pool":     "资源池同步",
	"region":   "区域同步",
	"instance": "实例同步",
}

// statusNames 原始状态展示名。
var statusNames = map[string]string{
	"pending":   "排队中",
	"running":   "执行中",
	"success":   "成功",
	"failed":    "失败",
	"manual":    "待人工",
	"skipped":   "已跳过",
	"cancelled": "已取消",
}

// upstreamStateNames 上游到达状态展示名。
var upstreamStateNames = map[string]string{
	dto.UpstreamReached:       "已到达上游",
	dto.UpstreamNotReached:    "未到达上游",
	dto.UpstreamPending:       "待执行",
	dto.UpstreamNotApplicable: "无需上游",
	dto.UpstreamSkipped:       "渠道不支持",
}

// ErrInvalidCategory 类别不在允许列表内。
var ErrInvalidCategory = errors.New("未知的任务类别")

func (s *taskQueueService) Categories(ctx context.Context) ([]dto.TaskQueueCategoryCount, error) {
	counts, err := s.repo.CategoryCounts(ctx, dto.TaskQueueListQuery{})
	if err != nil {
		return nil, err
	}
	return mergeCategoryCounts(counts), nil
}

func (s *taskQueueService) List(ctx context.Context, query dto.TaskQueueListQuery) (*dto.TaskQueueListResponse, error) {
	if query.Category != "" {
		if _, ok := categoryNames[query.Category]; !ok {
			return nil, ErrInvalidCategory
		}
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	rows, total, err := s.repo.List(ctx, query, page, pageSize)
	if err != nil {
		return nil, err
	}
	statusCounts, err := s.repo.StatusCounts(ctx, query)
	if err != nil {
		return nil, err
	}
	stateCounts, err := s.repo.StateCounts(ctx, query)
	if err != nil {
		return nil, err
	}
	catCounts, err := s.repo.CategoryCounts(ctx, query)
	if err != nil {
		return nil, err
	}

	// 用户名批量装饰（失败不阻塞列表）。
	userIDs := make([]uint64, 0, len(rows))
	seen := make(map[uint64]struct{}, len(rows))
	for i := range rows {
		if rows[i].UserID == 0 {
			continue
		}
		if _, ok := seen[rows[i].UserID]; ok {
			continue
		}
		seen[rows[i].UserID] = struct{}{}
		userIDs = append(userIDs, rows[i].UserID)
	}
	usernames, uerr := s.repo.Usernames(ctx, userIDs)
	if uerr != nil {
		usernames = map[uint64]string{}
	}

	// 渠道名批量装饰（失败不阻塞列表）。
	providerIDs := make([]uint64, 0, len(rows))
	seenProvider := make(map[uint64]struct{}, len(rows))
	for i := range rows {
		if rows[i].ProviderID == 0 {
			continue
		}
		if _, ok := seenProvider[rows[i].ProviderID]; ok {
			continue
		}
		seenProvider[rows[i].ProviderID] = struct{}{}
		providerIDs = append(providerIDs, rows[i].ProviderID)
	}
	providerNames, perr := s.repo.ProviderNames(ctx, providerIDs)
	if perr != nil {
		providerNames = map[uint64]string{}
	}

	items := make([]dto.TaskQueueItem, 0, len(rows))
	for i := range rows {
		row := rows[i]
		item := dto.TaskQueueItem{
			ID:                row.Category + ":" + itoa(row.RefID),
			Category:          row.Category,
			CategoryName:      categoryNames[row.Category],
			Action:            row.Action,
			ActionName:        actionName(row.Action, row.Category),
			Subject:           row.Subject,
			RefNo:             row.RefNo,
			InstanceRef:       row.InstanceRef,
			Status:            row.Status,
			StatusName:        statusName(row.Status),
			UpstreamState:     row.UpstreamState,
			UpstreamStateName: stateName(row.UpstreamState),
			UpstreamDetail:    row.Detail,
			ProviderID:        row.ProviderID,
			ProviderName:      providerNames[row.ProviderID],
			UserID:            row.UserID,
			Username:          usernames[row.UserID],
			Attempts:          row.Attempts,
			MaxAttempts:       row.MaxAttempts,
			Amount:            row.Amount,
			CreatedAt:         row.CreatedAt.Format(time.RFC3339),
		}
		if row.FinishedAt != nil {
			val := row.FinishedAt.Format(time.RFC3339)
			item.FinishedAt = &val
			if d := row.FinishedAt.Sub(row.CreatedAt).Milliseconds(); d > 0 {
				item.DurationMs = d
			}
		}
		items = append(items, item)
	}

	return &dto.TaskQueueListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
		Summary: dto.TaskQueueSummary{
			Total:       total,
			Pending:     pickStatus(statusCounts, "pending"),
			Running:     pickStatus(statusCounts, "running"),
			Success:     pickStatus(statusCounts, "success"),
			Failed:      pickStatus(statusCounts, "failed"),
			Manual:      pickStatus(statusCounts, "manual"),
			Reached:     pickState(stateCounts, dto.UpstreamReached),
			NotReached:  pickState(stateCounts, dto.UpstreamNotReached),
			ReachedRate: reachedRate(pickState(stateCounts, dto.UpstreamReached), pickState(stateCounts, dto.UpstreamNotReached)),
			Categories:  mergeCategoryCounts(catCounts),
		},
	}, nil
}

// mergeCategoryCounts 把仓储计数对齐到固定类别顺序，缺失补 0（页签不消失）。
func mergeCategoryCounts(counts []repository.CategoryCount) []dto.TaskQueueCategoryCount {
	byCat := make(map[string]repository.CategoryCount, len(counts))
	for _, c := range counts {
		byCat[c.Category] = c
	}
	out := make([]dto.TaskQueueCategoryCount, 0, len(categoryOrder))
	for _, cat := range categoryOrder {
		entry := byCat[cat]
		out = append(out, dto.TaskQueueCategoryCount{
			Category:     cat,
			CategoryName: categoryNames[cat],
			Total:        entry.Total,
			NotReached:   entry.NotReached,
		})
	}
	return out
}

func pickStatus(counts []repository.StatusCount, status string) int64 {
	for _, c := range counts {
		if c.Status == status {
			return c.Total
		}
	}
	return 0
}

func pickState(counts []repository.StateCount, state string) int64 {
	for _, c := range counts {
		if c.UpstreamState == state {
			return c.Total
		}
	}
	return 0
}

// reachedRate 到达率：仅以「已到达 + 未到达」为分母，排队中与无需上游不拉低指标。
func reachedRate(reached, notReached int64) float64 {
	denom := reached + notReached
	if denom == 0 {
		return 0
	}
	rate := float64(reached) / float64(denom) * 100
	return float64(int(rate*10+0.5)) / 10
}

func actionName(action, category string) string {
	if name, ok := actionNames[action]; ok {
		return name
	}
	if name, ok := categoryNames[category]; ok {
		return name
	}
	return action
}

func statusName(status string) string {
	if name, ok := statusNames[status]; ok {
		return name
	}
	if status == "" {
		return "未知"
	}
	return status
}

func stateName(state string) string {
	if name, ok := upstreamStateNames[state]; ok {
		return name
	}
	if state == "" {
		return "未知"
	}
	return state
}

// itoa 无依赖的无符号整数转字符串（避免为一行拼接引入 strconv）。
func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
