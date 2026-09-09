// Package service 提供用户中心订单模块业务编排：余额支付下单 → 触发上游开通。
package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	orderdto "hostsent/backend/internal/modules/admin/order/dto"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"

	"hostsent/backend/internal/modules/uc/order/dto"
)

var (
	// ErrProductOffline 商品下架不可购买
	ErrProductOffline = errors.New("商品已下架，不可购买")
	// ErrInsufficientBalance 余额不足
	ErrInsufficientBalance = errors.New("余额不足，请先充值")
	// ErrProvisionFailed 开通上游失败（已扣款，可稍后在订单页重试）
	ErrProvisionFailed = errors.New("开通资源失败，已扣款，可稍后重试")
)

// 以下为装配层注入的最小暴露接口（仅声明 uc 真正用到的方法，避免依赖 admin 的 service 层）。
type (
	// productReader 商品读取（catalog.FindByID）。
	productReader interface {
		FindByID(ctx context.Context, id uint64) (*catalogdto.ProductInfo, error)
	}
	// walletPort 账务调整（余额扣款）。
	walletPort interface {
		Adjust(ctx context.Context, req accountdto.AdjustRequest, operatorID uint64) (*transdto.TransactionInfo, error)
	}
	// orderRepoPort 订单仓储读写。
	orderRepoPort interface {
		Create(ctx context.Context, item *ordermodel.Order) error
		List(ctx context.Context, query orderdto.OrderListQuery) ([]ordermodel.Order, int64, error)
	}
	// orderActivator 订单履约开通（Activate）。
	orderActivator interface {
		Activate(ctx context.Context, id uint64) error
	}
)

// OrderService 用户中心订单业务能力。
type OrderService interface {
	// Create 用户下单（余额支付 + 开通上游）。
	Create(ctx context.Context, userID uint64, req dto.CreateRequest) (*dto.OrderInfo, error)
	// List 我的订单。
	List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error)
}

type orderService struct {
	catalog        productReader
	wallet         walletPort
	orderRepo      orderRepoPort
	orderSvc       orderActivator
	ensureOpenable func(ctx context.Context, productID uint64) error // 校验商品是否可即时开通（避免误扣款）
	isBalanceErr   func(error) bool                                  // 余额不足判定（由装配层提供，避免依赖 admin 错误变量）
}

// NewOrderService 创建用户中心订单服务。
func NewOrderService(
	catalog productReader,
	wallet walletPort,
	orderRepo orderRepoPort,
	orderSvc orderActivator,
	ensureOpenable func(ctx context.Context, productID uint64) error,
	isBalanceErr func(error) bool,
) OrderService {
	return &orderService{catalog: catalog, wallet: wallet, orderRepo: orderRepo, orderSvc: orderSvc, ensureOpenable: ensureOpenable, isBalanceErr: isBalanceErr}
}

// Create 用户下单：校验商品 → 余额扣款 → 创建已支付订单 → 触发上游开通。
func (s *orderService) Create(ctx context.Context, userID uint64, req dto.CreateRequest) (*dto.OrderInfo, error) {
	if req.ProductID == 0 {
		return nil, errors.New("商品不能为空")
	}
	product, err := s.catalog.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Status != 1 {
		return nil, ErrProductOffline
	}
	if product.Price <= 0 {
		return nil, errors.New("商品定价无效，暂不可购买")
	}
	// 校验商品是否可即时开通（财务型上游等不支持直连开通，避免误扣款）
	if s.ensureOpenable != nil {
		if err := s.ensureOpenable(ctx, req.ProductID); err != nil {
			return nil, err
		}
	}
	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}
	amount := product.Price * float64(qty)

	// 1) 余额扣款（支出方向 -1），返回资金流水
	if _, err := s.wallet.Adjust(ctx, accountdto.AdjustRequest{
		UserID:    userID,
		Type:      "order",
		Direction: -1, // 支出
		Amount:    amount,
		BizKey:    fmt.Sprintf("uc-order-%d-%d", userID, time.Now().UnixNano()),
		Remark:    fmt.Sprintf("购买云主机：%s", product.Name),
	}, userID); err != nil {
		if s.isBalanceErr != nil && s.isBalanceErr(err) {
			return nil, ErrInsufficientBalance
		}
		return nil, err
	}

	// 2) 创建已支付订单
	now := time.Now()
	order := &ordermodel.Order{
		OrderNo:     genOrderNo(userID),
		UserID:      userID,
		ProductID:   product.ID,
		ProductName: product.Name,
		Specs:       product.Specs,
		Quantity:    qty,
		PriceModel:  product.PriceModel,
		TotalAmount: amount,
		PaidAmount:  amount,
		Status:      ordermodel.OrderStatusPaid,
		PayMethod:   "balance",
		PayTime:     &now,
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}

	// 3) 触发上游开通（orderService.Activate：paid → 创建实例 → active）
	if err := s.orderSvc.Activate(ctx, order.ID); err != nil {
		// 已扣款，订单保持 paid，允许用户稍后重试
		info := fromAdminOrder(*order)
		return &info, ErrProvisionFailed
	}
	info := fromAdminOrder(*order)
	info.Status = ordermodel.OrderStatusActive
	return &info, nil
}

// List 我的订单。
func (s *orderService) List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.orderRepo.List(ctx, orderdto.OrderListQuery{
		UserID: userID, Status: query.Status, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	resp := &dto.ListResponse{Page: page, PageSize: pageSize, Total: total, Items: make([]dto.OrderInfo, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, fromAdminOrder(it))
	}
	return resp, nil
}

// fromAdminOrder 映射订单为用户可见字段。
func fromAdminOrder(o ordermodel.Order) dto.OrderInfo {
	return dto.OrderInfo{
		ID:          o.ID,
		OrderNo:     o.OrderNo,
		ProductID:   o.ProductID,
		ProductName: o.ProductName,
		Specs:       o.Specs,
		PriceModel:  o.PriceModel,
		TotalAmount: o.TotalAmount,
		PaidAmount:  o.PaidAmount,
		Status:      o.Status,
		PayMethod:   o.PayMethod,
		CreatedAt:   o.CreatedAt.Format(time.RFC3339),
	}
}

// genOrderNo 生成订单号：时间戳 + 随机。
func genOrderNo(userID uint64) string {
	return fmt.Sprintf("UC%d%d%04d", userID, time.Now().Unix(), rand.Intn(10000))
}
