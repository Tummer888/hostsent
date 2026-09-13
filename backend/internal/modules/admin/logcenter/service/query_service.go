package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/logcenter/catalog"
	"hostsent/backend/internal/modules/admin/logcenter/dto"
	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
)

// QueryService 日志浏览器（doc92 §8.2）。
//
// 26 个源共用一套页面，靠的就是这里下发的列元数据：前端不写死任何列，
// 全部由 catalog 驱动渲染。
type QueryService interface {
	Catalog(ctx context.Context) (*dto.CatalogResponse, error)
	Stats(ctx context.Context) (*dto.StatsResponse, error)
	Query(ctx context.Context, req dto.QueryRequest) (*dto.QueryResponse, error)
	// QueryWithFilters 同 Query，但额外接受源特有的等值筛选参数（服务层按白名单过滤）。
	QueryWithFilters(ctx context.Context, req dto.QueryRequest, filters map[string]string) (*dto.QueryResponse, error)
	Detail(ctx context.Context, source string, id uint64) (*dto.DetailResponse, error)
	// BuildFilter 把请求参数按源的白名单转换成仓储筛选条件。
	BuildFilter(src *catalog.Source, from, to, keyword string, filters map[string]string) logrepo.Filter
}

type queryService struct {
	repo        logrepo.LogQueryRepository
	policy      PolicyService
	exportRepo  logrepo.ExportRepository
	cleanupRepo logrepo.CleanupRepository
	jobRunRepo  logrepo.JobRunRepository
	metadata    MetadataCounter
	logger      *zap.Logger
}

// MetadataCounter 采集侧计数（由上游写入器提供）。
type MetadataCounter interface {
	Stats() (written, dropped int64)
}

// NewQueryService 创建查询服务。
func NewQueryService(
	repo logrepo.LogQueryRepository,
	policy PolicyService,
	exportRepo logrepo.ExportRepository,
	cleanupRepo logrepo.CleanupRepository,
	jobRunRepo logrepo.JobRunRepository,
	metadata MetadataCounter,
	logger *zap.Logger,
) QueryService {
	return &queryService{
		repo: repo, policy: policy, exportRepo: exportRepo,
		cleanupRepo: cleanupRepo, jobRunRepo: jobRunRepo, metadata: metadata, logger: logger,
	}
}

// Catalog 下发源目录与列元数据（含行数与索引诊断，5 分钟由前端缓存）。
func (s *queryService) Catalog(ctx context.Context) (*dto.CatalogResponse, error) {
	items := make([]dto.SourceInfo, 0, 26)
	for _, entry := range catalog.All() {
		src := catalog.Get(entry.Key)
		cols := make([]dto.ColumnInfo, 0, len(src.SelectColumns))
		for _, c := range src.SelectColumns {
			cols = append(cols, dto.ColumnInfo{
				Key: c.Key, Label: c.Label, Type: c.Type, Width: c.Width,
				Hidden: c.Hidden, Masked: c.Masked, Enum: c.Enum,
			})
		}
		info := dto.SourceInfo{
			Key: src.Key, DisplayName: src.DisplayName, Group: src.Group,
			GroupLabel: catalog.GroupLabel(src.Group), Class: src.Class,
			Cleanable: src.Cleanable, TimeColumn: src.TimeColumn, Columns: cols,
			FilterColumns: src.FilterColumns, Searchable: len(src.SearchColumns) > 0,
			DefaultAction: src.EffectiveAction(), DefaultRetentionDays: src.DefaultRetention(),
			IndependentPage: src.IndependentPage, Remark: src.Remark,
		}
		// 行数失败不阻断目录下发（徽标是装饰信息，不该让整页打不开）。
		if n, err := s.repo.CountRows(ctx, src); err == nil {
			info.Rows = n
		} else {
			s.logger.Warn("logcenter: count source rows failed",
				zap.String("source", src.Key), zap.Error(err))
		}
		before := time.Now().AddDate(0, 0, -30)
		if ok, err := s.repo.UsesIndex(ctx, src, before); err == nil {
			info.HasIndex = ok
		}
		items = append(items, info)
	}

	groups := make([]dto.GroupInfo, 0, 8)
	for _, g := range catalog.GroupOrder() {
		groups = append(groups, dto.GroupInfo{Key: g, Label: catalog.GroupLabel(g)})
	}
	return &dto.CatalogResponse{
		Groups: groups,
		Items:  items,
		Actions: []dto.EnumOption{
			{Value: catalog.ActionClean, Label: catalog.ActionLabel(catalog.ActionClean)},
			{Value: catalog.ActionArchiveOnly, Label: catalog.ActionLabel(catalog.ActionArchiveOnly)},
			{Value: catalog.ActionKeep, Label: catalog.ActionLabel(catalog.ActionKeep)},
		},
		Classes: []dto.EnumOption{
			{Value: catalog.ClassOps, Label: "运维流水（可清理）"},
			{Value: catalog.ClassAudit, Label: "审计留痕（只备份不删）"},
		},
	}, nil
}

// Stats 汇总统计。
func (s *queryService) Stats(ctx context.Context) (*dto.StatsResponse, error) {
	resp := &dto.StatsResponse{
		RowsByGroup:  map[string]int64{},
		RowsBySource: map[string]int64{},
	}
	for _, item := range catalog.All() {
		src := catalog.Get(item.Key)
		n, err := s.repo.CountRows(ctx, src)
		if err != nil {
			// 单源失败跳过，不让一个坏源毁掉整页统计。
			s.logger.Warn("logcenter: stats count failed", zap.String("source", src.Key), zap.Error(err))
			continue
		}
		resp.RowsBySource[src.Key] = n
		resp.RowsByGroup[src.Group] += n
		resp.TotalRows += n
	}
	if s.metadata != nil {
		resp.UpstreamRecorded, resp.UpstreamDropped = s.metadata.Stats()
	}
	if s.cleanupRepo != nil {
		if active, err := s.cleanupRepo.HasActive(ctx); err == nil && active {
			// 取最近一条非终态任务的 ID 供前端提示。
			if jobs, _, err := s.cleanupRepo.List(ctx, logrepo.CleanupListQuery{
				Status: "", Page: 1, PageSize: 20,
			}); err == nil {
				for _, j := range jobs {
					if logmodel.IsActiveStatus(j.Status) {
						resp.ActiveCleanupJob = j.ID
						break
					}
				}
			}
		}
	}
	if s.exportRepo != nil {
		files, _, err := s.exportRepo.List(ctx, logrepo.ExportListQuery{Page: 1, PageSize: 500})
		if err == nil {
			resp.ExportFiles = int64(len(files))
			for i := range files {
				resp.ExportTotalSize += files[i].FileSize
			}
		}
	}
	return resp, nil
}

// Query 分页查询某源。
func (s *queryService) Query(ctx context.Context, req dto.QueryRequest) (*dto.QueryResponse, error) {
	return s.QueryWithFilters(ctx, req, nil)
}

// QueryWithFilters 分页查询某源（带等值筛选）。
func (s *queryService) QueryWithFilters(ctx context.Context, req dto.QueryRequest, filters map[string]string) (*dto.QueryResponse, error) {
	src := catalog.Get(req.Source)
	if src == nil {
		return nil, ErrPolicyRejected
	}
	page, pageSize := normalizePage(req.Page, req.PageSize)
	f := s.BuildFilter(src, req.From, req.To, req.Keyword, filters)
	items, err := s.repo.Query(ctx, src, f, page, pageSize)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, src, f)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []map[string]any{}
	}
	return &dto.QueryResponse{
		Source: src.Key, Items: items, Total: total, Page: page, PageSize: pageSize,
		From: req.From, To: req.To,
	}, nil
}

// Detail 取单行详情。
func (s *queryService) Detail(ctx context.Context, source string, id uint64) (*dto.DetailResponse, error) {
	src := catalog.Get(source)
	if src == nil {
		return nil, ErrPolicyRejected
	}
	item, err := s.repo.FindByID(ctx, src, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return &dto.DetailResponse{Source: src.Key, Item: item}, nil
}

// BuildFilter 按源的白名单构造筛选条件。
//
// 等值筛选只接受 FilterColumns 里声明的参数名，并且再次确认目标列在该源的
// 可选列白名单内 —— 参数名与列名都来自代码常量（doc92 §0.3 硬约束 1）。
func (s *queryService) BuildFilter(src *catalog.Source, from, to, keyword string, filters map[string]string) logrepo.Filter {
	f := logrepo.Filter{Keyword: keyword}
	if t, err := logrepo.ParseTime(from); err == nil && from != "" {
		f.From = t
	}
	if t, err := logrepo.ParseTime(to); err == nil && to != "" {
		f.To = t
	}
	if len(filters) == 0 {
		return f
	}
	allowed := map[string]bool{}
	for _, c := range src.SelectColumns {
		allowed[c.Key] = true
	}
	equals := map[string]string{}
	for param, value := range filters {
		column, ok := src.FilterColumns[param]
		if !ok || !allowed[column] || strings.TrimSpace(value) == "" {
			continue
		}
		equals[column] = strings.TrimSpace(value)
	}
	if len(equals) > 0 {
		f.Equals = equals
	}
	return f
}

// normalizePage 收敛分页参数。
func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

// parseUint 宽松解析 uint64（前端可能传空串）。
func parseUint(raw string) uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
