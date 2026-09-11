package service

import (
	"context"
	"testing"
	"time"

	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	lifecycledto "hostsent/backend/internal/modules/admin/lifecycle/dto"
	lifecycleservice "hostsent/backend/internal/modules/admin/lifecycle/service"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/modules/open/dto"
	openrepo "hostsent/backend/internal/modules/open/repository"
	apperrors "hostsent/backend/internal/pkg/errors"
)

type stubInstanceRepo struct {
	rows map[uint64]*syncmodel.Instance
	list []syncmodel.Instance
}

func (s *stubInstanceRepo) ListByUser(_ context.Context, userID uint64, _ string, _, _ int) ([]syncmodel.Instance, int64, error) {
	out := make([]syncmodel.Instance, 0)
	for _, r := range s.rows {
		if r.UserID == userID {
			out = append(out, *r)
		}
	}
	return out, int64(len(out)), nil
}

func (s *stubInstanceRepo) FindByIDForUser(_ context.Context, id, userID uint64) (*syncmodel.Instance, error) {
	if row, ok := s.rows[id]; ok && row.UserID == userID {
		return row, nil
	}
	return nil, openrepo.ErrInstanceNotFound
}

type stubOps struct {
	powerActions []string
	suspendIDs   []uint64
	unsuspendIDs []uint64
	err          error
	lastOperator instanceservice.Operator
}

func (s *stubOps) Power(_ context.Context, op instanceservice.Operator, id uint64, action string) error {
	s.lastOperator = op
	s.powerActions = append(s.powerActions, action)
	_ = id
	return s.err
}
func (s *stubOps) Suspend(_ context.Context, op instanceservice.Operator, id uint64, _ string) error {
	s.lastOperator = op
	s.suspendIDs = append(s.suspendIDs, id)
	return s.err
}
func (s *stubOps) Unsuspend(_ context.Context, op instanceservice.Operator, id uint64) error {
	s.lastOperator = op
	s.unsuspendIDs = append(s.unsuspendIDs, id)
	return s.err
}

type stubRenewals struct {
	calls   int
	lastUID uint64
	result  *lifecycledto.RenewalCreatedResponse
	err     error
}

func (s *stubRenewals) UserRenew(_ context.Context, userID, _ uint64, _ *lifecycledto.UserRenewRequest) (*lifecycledto.RenewalCreatedResponse, error) {
	s.calls++
	s.lastUID = userID
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func instanceFixture() (*OpenInstanceService, *stubInstanceRepo, *stubOps, *stubRenewals) {
	repo := &stubInstanceRepo{rows: map[uint64]*syncmodel.Instance{
		9: {ID: 9, InstanceID: "vm-9", UserID: 42, Name: "开放实例", Status: "active", CPU: 2, Memory: 4096,
			ExpireAt: ptrTime(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))},
		10: {ID: 10, InstanceID: "vm-10", UserID: 43, Name: "他人实例", Status: "active"},
	}}
	ops := &stubOps{}
	renewals := &stubRenewals{result: &lifecycledto.RenewalCreatedResponse{}}
	svc := NewOpenInstanceService(InstanceDeps{Repo: repo, Ops: ops, Renewals: renewals})
	return svc, repo, ops, renewals
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestInstanceListScopedByOwner(t *testing.T) {
	svc, _, _, _ := instanceFixture()
	resp, err := svc.List(context.Background(), 42, "", 1, 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.Total != 1 || resp.Items[0].ID != 9 || resp.Items[0].SourceMode != "" {
		t.Fatalf("owner scoping broken: %+v", resp)
	}
	if resp.Items[0].ExpireAt != "2027-01-01T00:00:00Z" {
		t.Fatalf("expire format: %s", resp.Items[0].ExpireAt)
	}
}

func TestGetInstanceRejectsForeignInstance(t *testing.T) {
	svc, _, _, _ := instanceFixture()
	if _, err := svc.Get(context.Background(), 42, 10); err == nil {
		t.Fatalf("foreign instance must 404")
	} else if ae, ok := err.(*apperrors.AppError); !ok || ae.Code != CodeOpenNotFound {
		t.Fatalf("expect 40404, got %v", err)
	}
}

func TestInstancePowerValidatesAction(t *testing.T) {
	svc, _, ops, _ := instanceFixture()
	if err := svc.Power(context.Background(), appFixture(), 9, "destroy"); err == nil {
		t.Fatalf("invalid action must fail")
	}
	if err := svc.Power(context.Background(), appFixture(), 9, "off"); err != nil {
		t.Fatalf("Power off: %v", err)
	}
	if len(ops.powerActions) != 1 || ops.powerActions[0] != "off" {
		t.Fatalf("ops not called: %+v", ops.powerActions)
	}
	if ops.lastOperator.Name != "open-app:app_x" || ops.lastOperator.Type != "system" {
		t.Fatalf("operator identity: %+v", ops.lastOperator)
	}
}

func TestInstanceOpsEnforceOwnership(t *testing.T) {
	svc, _, ops, renewals := instanceFixture()
	_ = ops
	app := appFixture()
	if err := svc.Suspend(context.Background(), app, 10, "test"); err == nil {
		t.Fatalf("suspend foreign instance must fail")
	}
	if err := svc.Unsuspend(context.Background(), app, 10); err == nil {
		t.Fatalf("unsuspend foreign instance must fail")
	}
	if _, err := svc.Renew(context.Background(), app, 10, dto.RenewInstanceRequest{PeriodCount: 1}); err == nil {
		t.Fatalf("renew foreign instance must fail")
	}
	if renewals.calls != 0 {
		t.Fatalf("renew must not reach lifecycle for foreign instance")
	}
}

func TestInstanceRenewDelegatesToLifecycle(t *testing.T) {
	svc, _, _, renewals := instanceFixture()
	if _, err := svc.Renew(context.Background(), appFixture(), 9, dto.RenewInstanceRequest{PeriodCount: 2}); err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if renewals.calls != 1 || renewals.lastUID != 42 {
		t.Fatalf("renew must delegate with owner uid: calls=%d uid=%d", renewals.calls, renewals.lastUID)
	}
}

func TestInstanceRenewBalanceMapped(t *testing.T) {
	svc, _, _, renewals := instanceFixture()
	renewals.err = lifecycleservice.ErrInsufficientBalance
	if _, err := svc.Renew(context.Background(), appFixture(), 9, dto.RenewInstanceRequest{}); err == nil {
		t.Fatalf("expect error")
	} else if ae, ok := err.(*apperrors.AppError); !ok || ae.Code != CodeOpenInsufficientBalance {
		t.Fatalf("expect 30001, got %v", err)
	}
}

func TestDestroyExplicitlyUnsupported(t *testing.T) {
	svc, _, _, _ := instanceFixture()
	err := svc.Destroy()
	ae, ok := err.(*apperrors.AppError)
	if !ok || ae.Code != CodeOpenUnsupported {
		t.Fatalf("destroy must be fixed 40009, got %v", err)
	}
}
