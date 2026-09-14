package service

import (
	"context"
	"strings"

	"hostsent/backend/internal/modules/admin/content/dto"
	"hostsent/backend/internal/modules/admin/content/model"
	"hostsent/backend/internal/modules/admin/content/repository"
	"hostsent/backend/internal/pkg/revalidate"
)

// LinkService 友情链接业务能力。
type LinkService interface {
	List(ctx context.Context, req dto.LinkListQuery) (*dto.LinkListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.LinkInfo, error)
	Create(ctx context.Context, req *dto.LinkSaveRequest) (*dto.LinkInfo, error)
	Update(ctx context.Context, id uint64, req *dto.LinkSaveRequest) (*dto.LinkInfo, error)
	Delete(ctx context.Context, id uint64) error
	// ListActive 启用中的友情链接（门户页脚用），按 sort_order。
	ListActive(ctx context.Context) ([]dto.LinkInfo, error)
}

type linkService struct {
	repo repository.LinkRepository
}

// NewLinkService 创建友情链接业务服务。
func NewLinkService(repo repository.LinkRepository) LinkService {
	return &linkService{repo: repo}
}

func (s *linkService) List(ctx context.Context, req dto.LinkListQuery) (*dto.LinkListResponse, error) {
	items, total, err := s.repo.List(ctx, repository.LinkQuery{
		Status:   strings.TrimSpace(req.Status),
		Keyword:  req.Keyword,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*dto.LinkInfo, 0, len(items))
	for _, it := range items {
		info := toLinkInfo(it)
		list = append(list, &info)
	}
	page, pageSize := normalizePage(req.Page, req.PageSize)
	return &dto.LinkListResponse{Total: total, List: list, Page: page, PageSize: pageSize}, nil
}

func (s *linkService) FindByID(ctx context.Context, id uint64) (*dto.LinkInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrLinkNotFound
	}
	info := toLinkInfo(*item)
	return &info, nil
}

func (s *linkService) Create(ctx context.Context, req *dto.LinkSaveRequest) (*dto.LinkInfo, error) {
	url := strings.TrimSpace(req.URL)
	if url == "" {
		return nil, ErrLinkURLRequired
	}
	item := &model.FriendlyLink{
		Name:        strings.TrimSpace(req.Name),
		URL:         url,
		Logo:        strings.TrimSpace(req.Logo),
		Description: strings.TrimSpace(req.Description),
		SortOrder:   req.SortOrder,
		OpenInNew:   req.OpenInNew == nil || *req.OpenInNew,
		Status:      defaultCategoryStatus(req.Status),
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	revalidate.Notify(ctx, revalidate.KeyFriendlyLink)
	return s.FindByID(ctx, item.ID)
}

func (s *linkService) Update(ctx context.Context, id uint64, req *dto.LinkSaveRequest) (*dto.LinkInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrLinkNotFound
	}
	url := strings.TrimSpace(req.URL)
	if url == "" {
		return nil, ErrLinkURLRequired
	}
	item.Name = strings.TrimSpace(req.Name)
	item.URL = url
	item.Logo = strings.TrimSpace(req.Logo)
	item.Description = strings.TrimSpace(req.Description)
	item.SortOrder = req.SortOrder
	item.Status = defaultCategoryStatus(req.Status)
	if req.OpenInNew != nil {
		item.OpenInNew = *req.OpenInNew
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	revalidate.Notify(ctx, revalidate.KeyFriendlyLink)
	return s.FindByID(ctx, item.ID)
}

func (s *linkService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return ErrLinkNotFound
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// 友情链接渲染在门户页脚（每页都有），删掉后不清缓存等于全站留着死链。
	revalidate.Notify(ctx, revalidate.KeyFriendlyLink)
	return nil
}

// ListActive 门户页脚用的启用链接。
func (s *linkService) ListActive(ctx context.Context) ([]dto.LinkInfo, error) {
	items, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.LinkInfo, 0, len(items))
	for _, it := range items {
		out = append(out, toLinkInfo(it))
	}
	return out, nil
}

func toLinkInfo(l model.FriendlyLink) dto.LinkInfo {
	return dto.LinkInfo{
		ID:          l.ID,
		Name:        l.Name,
		URL:         l.URL,
		Logo:        l.Logo,
		Description: l.Description,
		SortOrder:   l.SortOrder,
		OpenInNew:   l.OpenInNew,
		Status:      l.Status,
		CreatedAt:   l.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   l.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
