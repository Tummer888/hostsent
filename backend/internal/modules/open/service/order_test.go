package service

import (
	"context"
	"sync"
	"testing"
	"time"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/open/dto"
	openmodel "hostsent/backend/internal/modules/open/model"
	openrepo "hostsent/backend/internal/modules/open/repository"
	ucorderdto "hostsent/backend/internal/modules/uc/order/dto"
	apperrors "hostsent/backend/internal/pkg/errors"
)

// stubUcOrders 记录 ChannelMeta 的下单执行器。
type stubUcOrders struct {
	mu      sync.Mutex
	calls   int
	lastReq ucorderdto.CreateRequest
	lastUID uint64
	result  *ucorderdto.OrderInfo
	err     error
}

func (s *stubUcOrders) Create(_ context.Context, userID, actorID uint64, req ucorderdto.CreateRequest) (*ucorderdto.OrderInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastReq = req
	s.lastUID = userID
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &ucorderdto.OrderInfo{ID: 101, OrderNo: "NO101", Status: ordermodel.OrderStatusPaid, FinalAmount: 57.8}, nil
}

// stubRequestRepo 内存幂等仓储。
type stubRequestRepo struct {
	mu   sync.Mutex
	rows map[string]*openmodel.OpenRequest
}

func newStubRequestRepo() *stubRequestRepo {
	return &stubRequestRepo{rows: map[string]*openmodel.OpenRequest{}}
}

func key(appID uint64, cid string) string { return string(rune(appID)) + ":" + cid }

func (s *stubRequestRepo) Claim(_ context.Context, appID uint64, clientRequestID, method, path string) (*openrepo.ClaimResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[key(appID, clientRequestID)]
	if !ok {
		row = &openmodel.OpenRequest{
			ID: uint64(len(s.rows) + 1), AppID: appID, ClientRequestID: clientRequestID,
			Method: method, Path: path, Status: openmodel.OpenRequestProcessing,
			UpdatedAt: time.Now(),
		}
		s.rows[key(appID, clientRequestID)] = row
		return &openrepo.ClaimResult{Request: row, Claimed: true}, nil
	}
	// 与真实仓储 classify 一致：completed 重放；processing 未停滞冲突、停滞可接管。
	if row.Status == openmodel.OpenRequestCompleted {
		return &openrepo.ClaimResult{Request: row, Claimed: false}, nil
	}
	if time.Since(row.UpdatedAt) > openrepo.StaleProcessingAfter {
		row.UpdatedAt = time.Now()
		return &openrepo.ClaimResult{Request: row, Claimed: true}, nil
	}
	return &openrepo.ClaimResult{Request: row, Claimed: false}, nil
}

func (s *stubRequestRepo) Complete(_ context.Context, id uint64, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range s.rows {
		if row.ID == id {
			bodyStr := string(body)
			row.ResponseBody = &bodyStr
			row.Status = openmodel.OpenRequestCompleted
			return nil
		}
	}
	return nil
}

func (s *stubRequestRepo) get(appID uint64, cid string) *openmodel.OpenRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rows[key(appID, cid)]
}

func appFixture() *openrepo.ResolvedApp {
	return openrepo.NewResolvedApp(
		openmodel.OpenApp{ID: 7, AppID: "app_x", Status: 1, OwnerUserID: 42},
		[]openmodel.OpenAppScope{{AppID: 7, Scope: "order:create"}}, nil,
	)
}

func TestOpenOrderCreateInjectsChannelMeta(t *testing.T) {
	uc := &stubUcOrders{}
	svc := NewOpenOrderService(OrderDeps{Orders: uc, Requests: newStubRequestRepo()})
	result, err := svc.Create(context.Background(), appFixture(), dto.OpenOrderRequest{
		ProductID: 1, SpecCode: "2c4g", Quantity: 1, CustomerRef: "cust-001", ClientRequestID: "req-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if uc.calls != 1 || uc.lastUID != 42 {
		t.Fatalf("uc must be called once for owner 42: calls=%d uid=%d", uc.calls, uc.lastUID)
	}
	meta := uc.lastReq.ChannelMeta
	if meta.Channel != ordermodel.OrderChannelOpen || meta.OpenAppID != 7 || meta.ChannelCustomerRef != "cust-001" {
		t.Fatalf("channel meta: %+v", meta)
	}
	if result.Order.OrderNo != "NO101" || result.CustomerRef != "cust-001" || result.IdempotentlyReplayed {
		t.Fatalf("result: %+v", result)
	}
}

func TestOpenOrderReplayReturnsFirstResult(t *testing.T) {
	uc := &stubUcOrders{}
	repo := newStubRequestRepo()
	svc := NewOpenOrderService(OrderDeps{Orders: uc, Requests: repo})
	req := dto.OpenOrderRequest{ProductID: 1, CustomerRef: "c1", ClientRequestID: "req-1"}

	r1, err := svc.Create(context.Background(), appFixture(), req)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	r2, err := svc.Create(context.Background(), appFixture(), req)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if uc.calls != 1 {
		t.Fatalf("replay must not re-execute business, uc calls=%d", uc.calls)
	}
	if !r2.IdempotentlyReplayed || r1.Order.OrderNo != r2.Order.OrderNo {
		t.Fatalf("replay must return same order marked replayed: %+v", r2)
	}
}

func TestOpenOrderFailureStoredAndReplayed(t *testing.T) {
	uc := &stubUcOrders{err: apperrors.New(30001, "余额不足")}
	repo := newStubRequestRepo()
	svc := NewOpenOrderService(OrderDeps{Orders: uc, Requests: repo})
	req := dto.OpenOrderRequest{ProductID: 1, ClientRequestID: "req-err"}

	if _, err := svc.Create(context.Background(), appFixture(), req); err == nil {
		t.Fatalf("business error must surface")
	}
	// 重放同 key：返回同样的失败（不得再次执行业务扣款）。
	_, err := svc.Create(context.Background(), appFixture(), req)
	ae, ok := err.(*apperrors.AppError)
	if !ok || ae.Code != 30001 {
		t.Fatalf("replay must return stored error, got %v", err)
	}
	if uc.calls != 1 {
		t.Fatalf("failed request replay must not re-execute, calls=%d", uc.calls)
	}
}

func TestOpenOrderProcessingConflict(t *testing.T) {
	uc := &stubUcOrders{}
	repo := newStubRequestRepo()
	svc := NewOpenOrderService(OrderDeps{Orders: uc, Requests: repo})
	// 直接占位一个未完成的 processing 行。
	req := dto.OpenOrderRequest{ProductID: 1, ClientRequestID: "req-live"}
	claim, _ := repo.Claim(context.Background(), 7, req.ClientRequestID, "POST", "/open/v1/orders")
	if !claim.Claimed {
		t.Fatalf("fresh claim must succeed")
	}
	// 模拟另一请求持有该键（业务未完成、未停滞）。
	repo.mu.Lock()
	row := repo.rows[key(7, req.ClientRequestID)]
	row.UpdatedAt = time.Now()
	repo.mu.Unlock()

	_, err := svc.Create(context.Background(), appFixture(), req)
	ae, ok := err.(*apperrors.AppError)
	if !ok || ae.Code != CodeOpenConflict {
		t.Fatalf("processing conflict must be 40003, got %v", err)
	}
	if uc.calls != 0 {
		t.Fatalf("conflict must not execute business")
	}
}

func TestOpenOrderStaleProcessingRetakes(t *testing.T) {
	uc := &stubUcOrders{}
	repo := newStubRequestRepo()
	svc := NewOpenOrderService(OrderDeps{Orders: uc, Requests: repo})
	// 预置一条已停滞的 processing 行。
	repo.Claim(context.Background(), 7, "req-old", "POST", "/open/v1/orders")
	repo.mu.Lock()
	repo.rows[key(7, "req-old")].UpdatedAt = time.Now().Add(-2 * openrepo.StaleProcessingAfter)
	repo.mu.Unlock()

	result, err := svc.Create(context.Background(), appFixture(), dto.OpenOrderRequest{ProductID: 1, ClientRequestID: "req-old"})
	if err != nil {
		t.Fatalf("stale retake: %v", err)
	}
	if uc.calls != 1 || result.IdempotentlyReplayed {
		t.Fatalf("stale claim must re-execute: %+v", result)
	}
}

func TestOpenOrderParamValidation(t *testing.T) {
	svc := NewOpenOrderService(OrderDeps{Orders: &stubUcOrders{}, Requests: newStubRequestRepo()})
	if _, err := svc.Create(context.Background(), appFixture(), dto.OpenOrderRequest{ProductID: 1}); err == nil {
		t.Fatalf("missing client request id must fail")
	}
	if _, err := svc.Create(context.Background(), appFixture(), dto.OpenOrderRequest{ClientRequestID: "r"}); err == nil {
		t.Fatalf("product_id=0 must fail")
	}
}
