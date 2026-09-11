package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "hostsent/backend/internal/pkg/errors"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/open/dto"
	openmodel "hostsent/backend/internal/modules/open/model"
	openrepo "hostsent/backend/internal/modules/open/repository"
	ucorderdto "hostsent/backend/internal/modules/uc/order/dto"
	ucorderservice "hostsent/backend/internal/modules/uc/order/service"
)

// UcOrderCreator UC 下单能力（开放下单复用同一条 SKU/算价/扣款/履约管线，不写第二套）。
type UcOrderCreator interface {
	Create(ctx context.Context, userID, actorID uint64, req ucorderdto.CreateRequest) (*ucorderdto.OrderInfo, error)
}

// OrderDeps 开放下单服务依赖。
type OrderDeps struct {
	Orders   UcOrderCreator
	Requests openrepo.OpenRequestRepository
}

// OpenOrderService 代客下单服务（T6.3）。
type OpenOrderService struct {
	deps OrderDeps
}

// NewOpenOrderService 构造开放下单服务。
func NewOpenOrderService(deps OrderDeps) *OpenOrderService {
	return &OpenOrderService{deps: deps}
}

// Create 代客下单：
//  1. 幂等键领取（X-Client-Request-Id），命中终态直接重放首次结果；
//  2. 复用 UC 下单管线（余额扣 owner_user_id、owner 用户组折扣定价 D3、投递开通任务 T5.1），
//     并注入渠道三列（channel='open'、open_app_id、channel_customer_ref）；
//  3. 回写响应快照，业务错误同样存储（重放返回首次结果，不得隐式重执行扣款类操作）。
func (s *OpenOrderService) Create(ctx context.Context, app *openrepo.ResolvedApp, req dto.OpenOrderRequest) (*dto.OpenOrderResult, error) {
	if req.ClientRequestID == "" {
		return nil, apperrors.New(CodeOpenParam, "缺少幂等键 X-Client-Request-Id")
	}
	if req.ProductID == 0 {
		return nil, apperrors.New(CodeOpenParam, "product_id 必填")
	}
	if app == nil || app.App.ID == 0 {
		return nil, apperrors.New(CodeOpenMissingAuth, "应用未鉴权")
	}

	claim, err := s.deps.Requests.Claim(ctx, app.App.ID, req.ClientRequestID, "POST", "/open/v1/orders")
	if err != nil {
		return nil, err
	}
	if !claim.Claimed {
		return s.replay(claim.Request)
	}

	// userID=owner_user_id（订单与余额归属），actorID 同 owner（操作人）。
	orderInfo, err := s.deps.Orders.Create(ctx, app.App.OwnerUserID, app.App.OwnerUserID, ucorderdto.CreateRequest{
		ProductID: req.ProductID,
		SpecCode:  req.SpecCode,
		Quantity:  req.Quantity,
		ChannelMeta: ucorderdto.ChannelMeta{
			Channel:            ordermodel.OrderChannelOpen,
			OpenAppID:          app.App.ID,
			ChannelCustomerRef: req.CustomerRef,
		},
	})
	if err != nil {
		// 业务失败也固化快照：同幂等键重放返回同样的失败结果，避免隐式重试扣款。
		mapped := mapOrderCreateErr(err)
		s.completeWithError(claim.Request.ID, mapped)
		return nil, mapped
	}

	result := &dto.OpenOrderResult{
		Order:       *orderInfo,
		CustomerRef: req.CustomerRef,
	}
	if err := s.deps.Requests.Complete(ctx, claim.Request.ID, mustJSON(result)); err != nil {
		// 快照回写失败不影响本请求响应；该幂等键会在停滞超时后允许接管（见 request_repo）。
		return result, nil
	}
	return result, nil
}

// replay 解析已存快照：错误快照还原为 AppError，成功快照还原为结果；
// 快照缺失（历史脏数据）按冲突处理。
func (s *OpenOrderService) replay(row *openmodel.OpenRequest) (*dto.OpenOrderResult, error) {
	if row.ResponseBody == nil || *row.ResponseBody == "" {
		return nil, apperrors.New(CodeOpenConflict, "请求正在处理中，请稍后用对账接口核对")
	}
	var envelope struct {
		// error 非空表示首次执行失败，重放还原同样的错误（幂等失败语义）。
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Order json.RawMessage `json:"order"`
	}
	if err := json.Unmarshal([]byte(*row.ResponseBody), &envelope); err != nil {
		return nil, apperrors.New(CodeOpenConflict, "请求正在处理中，请稍后用对账接口核对")
	}
	if envelope.Error != nil {
		return nil, apperrors.New(envelope.Error.Code, envelope.Error.Message)
	}
	if len(envelope.Order) == 0 {
		return nil, apperrors.New(CodeOpenConflict, "请求正在处理中，请稍后用对账接口核对")
	}
	var result dto.OpenOrderResult
	if err := json.Unmarshal([]byte(*row.ResponseBody), &result); err != nil {
		return nil, apperrors.New(CodeOpenConflict, "请求正在处理中，请稍后用对账接口核对")
	}
	result.IdempotentlyReplayed = true
	return &result, nil
}

func (s *OpenOrderService) completeWithError(requestID uint64, bizErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{"code": bizCodeOf(bizErr), "message": bizErr.Error()},
	})
	_ = s.deps.Requests.Complete(ctx, requestID, body)
}

// bizCodeOf 从业务错误提取开放错误码；非 AppError 统一内部错误。
func bizCodeOf(err error) int {
	if ae, ok := err.(*apperrors.AppError); ok {
		return ae.Code
	}
	return CodeOpenInternal
}

// mapOrderCreateErr 下单业务错误归一为开放错误码：
// 商品不存在/下架 → 40404，余额不足 → 30001，其余按 50000 透传消息。
func mapOrderCreateErr(err error) error {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*apperrors.AppError); ok {
		return ae
	}
	switch {
	case errors.Is(err, ucorderservice.ErrProductOffline), errors.Is(err, gorm.ErrRecordNotFound):
		return apperrors.New(CodeOpenNotFound, "商品不存在或未上架")
	case errors.Is(err, ucorderservice.ErrInsufficientBalance):
		return apperrors.New(CodeOpenInsufficientBalance, "归属账户余额不足")
	default:
		return apperrors.New(CodeOpenInternal, err.Error())
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
