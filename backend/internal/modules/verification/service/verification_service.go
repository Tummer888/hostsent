// Package service 提供实名认证模块的业务逻辑实现。
package service

import (
	"context"

	"hostsent/backend/internal/modules/verification/dto"
	"hostsent/backend/internal/modules/verification/model"
	"hostsent/backend/internal/modules/verification/repository"
)

// VerificationService 定义实名认证列表业务能力。
type VerificationService interface {
	ListPending(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error)
	ListApproved(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error)
	ListRejected(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error)
}

// verificationService 是 VerificationService 的默认实现。
type verificationService struct {
	repo repository.VerificationRepository
}

// normalizeMeta 规范分页参数。
func normalizeMeta(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// toVerificationInfo 将认证模型转换为返回结构。
func toVerificationInfo(item model.VerificationApplication) dto.VerificationInfo {
	return dto.VerificationInfo{
		ID:               item.ID,
		UserID:           item.UserID,
		Username:         item.Username,
		VerificationType: item.VerificationType,
		Status:           item.Status,
		RealName:         item.RealName,
		SubjectName:      item.SubjectName,
		IDType:           item.IDType,
		IDNumberMasked:   item.IDNumberMasked,
		MobileMasked:     item.MobileMasked,
		RiskFlags:        item.RiskFlags,
		SubmittedAt:      item.SubmittedAt,
		ReviewedAt:       item.ReviewedAt,
		ReviewedBy:       item.ReviewedBy,
		ReviewerName:     item.ReviewerName,
		RejectReasonCode: item.RejectReasonCode,
		RejectReason:     item.RejectReason,
		ReviewNote:       item.ReviewNote,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

// NewVerificationService 创建实名认证业务服务。
func NewVerificationService(repo repository.VerificationRepository) VerificationService {
	return &verificationService{repo: repo}
}

// ListPending 查询待审核实名认证列表。
func (s *verificationService) ListPending(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error) {
	return s.listByStatus(ctx, "pending", query)
}

// ListApproved 查询已通过实名认证列表。
func (s *verificationService) ListApproved(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error) {
	return s.listByStatus(ctx, "approved", query)
}

// ListRejected 查询已拒绝实名认证列表。
func (s *verificationService) ListRejected(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error) {
	return s.listByStatus(ctx, "rejected", query)
}

// listByStatus 按状态查询实名认证列表。
func (s *verificationService) listByStatus(ctx context.Context, status string, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.repo.ListByStatus(ctx, status, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.VerificationInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toVerificationInfo(item))
	}
	return &dto.ListResponse[dto.VerificationInfo]{
		Items: respItems,
		Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

