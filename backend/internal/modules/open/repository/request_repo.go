package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	openmodel "hostsent/backend/internal/modules/open/model"
)

// StaleProcessingAfter processing 记录视为停滞的时长：超过后可被同 key 请求安全接管
// （接管意味着业务层未确认完成，重执行可能产生重复业务，依赖业务自身幂等兜底）。
const StaleProcessingAfter = 10 * time.Minute

// ClaimResult 幂等键领取结果。
type ClaimResult struct {
	Request *openmodel.OpenRequest
	// Claimed=true 新领取（或接管停滞记录），调用方执行业务并回写响应；
	// Claimed=false 已有终态记录，直接重放 Request.ResponseBody。
	Claimed bool
}

// OpenRequestRepository 写接口幂等仓储（doc16 §8.5）。
type OpenRequestRepository interface {
	// Claim 按 (app_id, client_request_id) 领取幂等键。
	Claim(ctx context.Context, appID uint64, clientRequestID, method, path string) (*ClaimResult, error)
	// Complete 回写响应快照（成功与业务错误都存，重放返回首次结果）。
	Complete(ctx context.Context, id uint64, responseBody []byte) error
}

type openRequestRepository struct {
	db *gorm.DB
}

// NewOpenRequestRepository 构造幂等仓储。
func NewOpenRequestRepository(db *gorm.DB) OpenRequestRepository {
	return &openRequestRepository{db: db}
}

func (r *openRequestRepository) Claim(ctx context.Context, appID uint64, clientRequestID, method, path string) (*ClaimResult, error) {
	// 先查：命中终态直接重放。
	existing, err := r.findByKey(ctx, appID, clientRequestID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return r.classify(ctx, existing)
	}

	// 首次：插入 processing 占位。
	row := &openmodel.OpenRequest{
		AppID: appID, ClientRequestID: clientRequestID,
		Method: method, Path: path, Status: openmodel.OpenRequestProcessing,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		// 并发竞争同 key：另一请求先插入，转重放判定。
		existing, ferr := r.findByKey(ctx, appID, clientRequestID)
		if ferr != nil || existing == nil {
			return nil, ferr
		}
		return r.classify(ctx, existing)
	}
	return &ClaimResult{Request: row, Claimed: true}, nil
}

func (r *openRequestRepository) classify(ctx context.Context, row *openmodel.OpenRequest) (*ClaimResult, error) {
	if row.Status == openmodel.OpenRequestCompleted {
		return &ClaimResult{Request: row, Claimed: false}, nil
	}
	// processing：未停滞则冲突；停滞超时可接管重执行。
	stale := time.Since(row.UpdatedAt) > StaleProcessingAfter
	if !stale {
		return &ClaimResult{Request: row, Claimed: false}, nil
	}
	res := r.db.WithContext(ctx).Model(&openmodel.OpenRequest{}).
		Where("id = ? AND status = ? AND updated_at <= ?", row.ID, openmodel.OpenRequestProcessing, time.Now().Add(-StaleProcessingAfter)).
		Update("updated_at", time.Now())
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		// 已被其他请求接管。
		row.UpdatedAt = time.Now()
		return &ClaimResult{Request: row, Claimed: false}, nil
	}
	return &ClaimResult{Request: row, Claimed: true}, nil
}

func (r *openRequestRepository) findByKey(ctx context.Context, appID uint64, clientRequestID string) (*openmodel.OpenRequest, error) {
	var row openmodel.OpenRequest
	err := r.db.WithContext(ctx).
		Where("app_id = ? AND client_request_id = ?", appID, clientRequestID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *openRequestRepository) Complete(ctx context.Context, id uint64, responseBody []byte) error {
	body := string(responseBody)
	return r.db.WithContext(ctx).Model(&openmodel.OpenRequest{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":          openmodel.OpenRequestCompleted,
			"response_status": 200,
			"response_body":   body,
			"updated_at":      time.Now(),
		}).Error
}
