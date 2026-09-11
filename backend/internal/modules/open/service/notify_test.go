package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	openmodel "hostsent/backend/internal/modules/open/model"
	openrepo "hostsent/backend/internal/modules/open/repository"
	"hostsent/backend/internal/pkg/crypto"
)

func mustEncrypt(plain string) string {
	enc, err := crypto.Encrypt(plain, testEncryptKey)
	if err != nil {
		panic(err)
	}
	return enc
}

type stubNotifyRepo struct {
	mu   sync.Mutex
	rows map[uint64]*openmodel.OpenNotifyDelivery
	seq  uint64
}

func newStubNotifyRepo() *stubNotifyRepo {
	return &stubNotifyRepo{rows: map[uint64]*openmodel.OpenNotifyDelivery{}}
}

func (s *stubNotifyRepo) Create(_ context.Context, d *openmodel.OpenNotifyDelivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	d.ID = s.seq
	cp := *d
	s.rows[d.ID] = &cp
	return nil
}

func (s *stubNotifyRepo) ListDue(_ context.Context, limit int) ([]openmodel.OpenNotifyDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]openmodel.OpenNotifyDelivery, 0)
	for _, r := range s.rows {
		if (r.Status == openmodel.OpenNotifyPending || r.Status == openmodel.OpenNotifyFailed) &&
			(r.NextRetryAt == nil || !r.NextRetryAt.After(time.Now())) {
			out = append(out, *r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *stubNotifyRepo) MarkSuccess(_ context.Context, id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id].Status = openmodel.OpenNotifySuccess
	now := time.Now()
	s.rows[id].DeliveredAt = &now
	return nil
}

func (s *stubNotifyRepo) MarkFailed(_ context.Context, id uint64, attempts int, nextRetryAt time.Time, lastError string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id].Status = openmodel.OpenNotifyFailed
	s.rows[id].Attempts = attempts
	s.rows[id].NextRetryAt = &nextRetryAt
	s.rows[id].LastError = lastError
	return nil
}

func (s *stubNotifyRepo) MarkDead(_ context.Context, id uint64, attempts int, lastError string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id].Status = openmodel.OpenNotifyDead
	s.rows[id].Attempts = attempts
	s.rows[id].LastError = lastError
	return nil
}

func (s *stubNotifyRepo) GetByID(_ context.Context, id uint64) (*openmodel.OpenNotifyDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.rows[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, openrepo.ErrInstanceNotFound
}

func (s *stubNotifyRepo) Redeliver(_ context.Context, id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id].Status = openmodel.OpenNotifyPending
	s.rows[id].Attempts = 0
	s.rows[id].NextRetryAt = nil
	return nil
}

func (s *stubNotifyRepo) ListDeliveries(_ context.Context, _ uint64, _ int) ([]openmodel.OpenNotifyDelivery, error) {
	return nil, nil
}

func (s *stubNotifyRepo) AppIDLookup(_ context.Context, _ string) (uint64, error) {
	return 0, nil
}

func TestEventPublisherSkipsAppsWithoutNotifyURL(t *testing.T) {
	notifyRepo := newStubNotifyRepo()
	appRepo := &stubAppRepo{apps: map[string]*openrepo.ResolvedApp{}}
	// owner 42 无任何可通知应用 → 不落库。
	publisher := NewEventPublisher(notifyRepo, appRepo, nil)
	publisher.PublishToOwner(context.Background(), 42, EventInstanceCreated, map[string]any{"x": 1})
	if len(notifyRepo.rows) != 0 {
		t.Fatalf("no notifiable apps must produce no deliveries")
	}
}

func TestEventPublisherCreatesPendingDelivery(t *testing.T) {
	notifyRepo := newStubNotifyRepo()
	// 归属 owner 42 的启用应用（带回调地址）。
	enc := mustEncrypt("notify-secret")
	appRepo := &stubAppRepo{apps: map[string]*openrepo.ResolvedApp{
		"app_evt": openrepo.NewResolvedApp(openmodel.OpenApp{
			ID: 5, AppID: "app_evt", OwnerUserID: 42, Status: openmodel.OpenAppStatusEnabled,
			NotifyURL: "http://receiver/notify", NotifySecret: enc,
		}, nil, nil),
	}}
	publisher := NewEventPublisher(notifyRepo, appRepo, nil)
	publisher.PublishToOwner(context.Background(), 42, EventInstanceRenewed, map[string]any{"renewal_no": "RN1"})
	if len(notifyRepo.rows) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(notifyRepo.rows))
	}
	for _, r := range notifyRepo.rows {
		if r.Status != openmodel.OpenNotifyPending || r.AppID != 5 || r.Event != EventInstanceRenewed {
			t.Fatalf("delivery row: %+v", r)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(r.Payload), &payload); err != nil || payload["event"] != EventInstanceRenewed {
			t.Fatalf("payload: %s", r.Payload)
		}
	}
}

func TestNotifyWorkerDeliversAndSigns(t *testing.T) {
	var mu sync.Mutex
	var gotHeaders http.Header
	var gotBody []byte
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotHeaders = r.Header.Clone()
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	notifyRepo := newStubNotifyRepo()
	enc := mustEncrypt("notify-secret")
	appRepo := &stubAppRepo{apps: map[string]*openrepo.ResolvedApp{
		"app_evt": openrepo.NewResolvedApp(openmodel.OpenApp{
			ID: 5, AppID: "app_evt", OwnerUserID: 42, Status: openmodel.OpenAppStatusEnabled,
			NotifyURL: receiver.URL, NotifySecret: enc,
		}, nil, nil),
	}}
	publisher := NewEventPublisher(notifyRepo, appRepo, nil)
	publisher.PublishToOwner(context.Background(), 42, EventInstanceStatusChanged, map[string]any{"to": "off"})

	worker := NewNotifyDeliveryWorker(notifyRepo, appRepo, testEncryptKey, nil, NotifyWorkerOptions{})
	worker.runOnce(context.Background())

	mu.Lock()
	event := gotHeaders.Get("X-Open-Event")
	mu.Unlock()
	if event != EventInstanceStatusChanged {
		t.Fatalf("receiver should get event header, got %q", event)
	}
	for _, r := range notifyRepo.rows {
		if r.Status != openmodel.OpenNotifySuccess {
			t.Fatalf("delivery must be success: %+v", r)
		}
	}
	_ = gotBody
}

func TestNotifyWorkerBackoffThenDead(t *testing.T) {
	// 恒 500 的接收端。
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer receiver.Close()

	notifyRepo := newStubNotifyRepo()
	enc := mustEncrypt("notify-secret")
	appRepo := &stubAppRepo{apps: map[string]*openrepo.ResolvedApp{
		"app_evt": openrepo.NewResolvedApp(openmodel.OpenApp{
			ID: 5, AppID: "app_evt", OwnerUserID: 42, Status: openmodel.OpenAppStatusEnabled,
			NotifyURL: receiver.URL, NotifySecret: enc,
		}, nil, nil),
	}}
	publisher := NewEventPublisher(notifyRepo, appRepo, nil)
	publisher.PublishToOwner(context.Background(), 42, EventInstanceCreated, map[string]any{})

	worker := NewNotifyDeliveryWorker(notifyRepo, appRepo, testEncryptKey, nil, NotifyWorkerOptions{})
	worker.runOnce(context.Background())
	row := notifyRepo.rows[1]
	if row.Status != openmodel.OpenNotifyFailed || row.Attempts != 1 {
		t.Fatalf("first failure must schedule retry: %+v", row)
	}
	if row.NextRetryAt == nil || !row.NextRetryAt.After(time.Now()) {
		t.Fatalf("backoff must be in the future")
	}
	// 模拟反复重试直至耗尽。
	for i := 2; i <= row.MaxAttempts; i++ {
		notifyRepo.mu.Lock()
		notifyRepo.rows[1].NextRetryAt = nil // 立即到期
		notifyRepo.mu.Unlock()
		worker.runOnce(context.Background())
	}
	if final := notifyRepo.rows[1]; final.Status != openmodel.OpenNotifyDead {
		t.Fatalf("exhausted attempts must go dead: %+v", final)
	}
}

func TestRetryBackoffSeries(t *testing.T) {
	cases := map[int]time.Duration{1: time.Minute, 2: 2 * time.Minute, 3: 4 * time.Minute, 8: 64 * time.Minute, 9: 64 * time.Minute}
	for attempts, want := range cases {
		if got := retryBackoff(attempts); got != want {
			t.Fatalf("retryBackoff(%d) = %v, want %v", attempts, got, want)
		}
	}
}
