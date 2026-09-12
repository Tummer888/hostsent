// Package service 提供积分体系业务能力：发放、人工调整、规则与查询。
//
// 边界（docs/实施计划/36 §2.1）：本服务只写 point_* 表，不 import finance/order/payment，
// 与资金账本零耦合；发放由装配层的支付成功钩子以最小入参调用（EarnInput 仅基础类型）。
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/point/dto"
	"hostsent/backend/internal/modules/admin/point/model"
	"hostsent/backend/internal/modules/admin/point/repository"
)

// BillingViewPermission 用户端查看积分所需权限码（复用费用中心口径）。
const BillingViewPermission = "billing:view"

// 场景别名：供装配层引用（model 常量在本包内已可用，导出别名便于跨包取用）。
const (
	SceneOrderPurchase = model.SceneOrderPurchase
	SceneOrderRenewal  = model.SceneOrderRenewal
	SceneBillPayment   = model.SceneBillPayment
	SceneActivity      = model.SceneActivity
)

// EarnInput 积分发放入参。只传基础类型，避免积分模块反向依赖业务模块。
type EarnInput struct {
	UserID uint64
	// BizType 幂等业务标识（order/bill/recharge），与 RefNo 组成唯一键。
	BizType string
	// RefNo 业务单号（订单号/账单号/充值单号）。
	RefNo string
	// Scene 发放场景（order_purchase/order_renewal/bill_payment）。
	Scene string
	// Amount 计提基数（实付金额，元）。
	Amount float64
}

// PointService 积分能力总接口。
type PointService interface {
	// Earn 按规则发放积分（幂等：同 (user, bizType, refNo) 只发一次）。
	// 无匹配规则、金额不达门槛或已发放时静默返回 nil，不阻断支付链路。
	Earn(ctx context.Context, in EarnInput) error
	// Adjust 人工调整积分（正数发放 / 负数扣减，幂等键为单号）。
	Adjust(ctx context.Context, req dto.AdjustRequest, operatorID uint64) (*dto.TransactionInfo, error)

	// —— 管理端 ——
	Accounts(ctx context.Context, q dto.AccountListQuery) (*dto.AccountListResponse, error)
	AccountOf(ctx context.Context, userID uint64) (*dto.AccountInfo, error)
	Transactions(ctx context.Context, q dto.TransactionListQuery) (*dto.TransactionListResponse, error)
	Rules(ctx context.Context, q dto.RuleListQuery) (*dto.RuleListResponse, error)
	CreateRule(ctx context.Context, req dto.RuleSaveRequest) (*dto.RuleInfo, error)
	UpdateRule(ctx context.Context, id uint64, req dto.RuleSaveRequest) (*dto.RuleInfo, error)
	// Overview 全平台积分概览（积分中心首页）：总量 + 生效规则 + 最近流水。
	Overview(ctx context.Context) (*dto.OverviewResponse, error)

	// —— 用户端 ——
	MyOverview(ctx context.Context, userID uint64) (*dto.MyOverview, error)
	MyTransactions(ctx context.Context, userID uint64, q dto.TransactionListQuery) (*dto.TransactionListResponse, error)
}

type pointService struct {
	db   *gorm.DB
	repo repository.PointRepository
}

// NewPointService 创建积分服务。
func NewPointService(db *gorm.DB, repo repository.PointRepository) PointService {
	return &pointService{db: db, repo: repo}
}

// —— 发放 ——

// Earn 按场景规则发放积分。
//
// 容错策略与返现/退款钩子一致：规则缺失、金额不达门槛、重复发放都返回 nil，
// 只有数据库写入失败才返回 error，由调用方记日志（绝不回滚支付/开通）。
func (s *pointService) Earn(ctx context.Context, in EarnInput) error {
	in.BizType = strings.TrimSpace(in.BizType)
	in.RefNo = strings.TrimSpace(in.RefNo)
	if in.UserID == 0 || in.RefNo == "" || in.BizType == "" || in.Amount <= 0 {
		return nil
	}
	rule, err := s.repo.FindRuleByScene(ctx, in.Scene)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 该场景未配置规则 = 不发放
		}
		return err
	}
	if rule.Status != model.RuleStatusEnabled || in.Amount < rule.MinAmount {
		return nil
	}
	points := calcPoints(rule, in.Amount)
	if points <= 0 {
		return nil
	}
	_, err = s.grant(ctx, grantInput{
		UserID:    in.UserID,
		TxType:    earnTxType(in.Scene),
		BizType:   in.BizType,
		RefNo:     in.RefNo,
		Points:    points,
		Remark:    fmt.Sprintf("%s：%.2f 元", rule.Name, in.Amount),
		ValidDays: rule.ValidDays,
	})
	return err
}

// grantInput 发放/扣减的统一入参（内部使用）。
type grantInput struct {
	UserID     uint64
	TxType     string
	BizType    string
	RefNo      string
	Points     int64 // 正数=获得，负数=消耗
	Remark     string
	OperatorID uint64
	ValidDays  int
}

// grant 事务内完成「幂等检查 → 账户加锁 → 记流水 → 更新余额」。
func (s *pointService) grant(ctx context.Context, in grantInput) (*model.PointTransaction, error) {
	var result *model.PointTransaction
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, err := s.repo.FindTxByBiz(tx, in.UserID, in.BizType, in.RefNo); err == nil && existing != nil {
			result = existing // 幂等：已发放，直接返回原流水
			return nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		acc, err := s.repo.EnsureAccount(tx, in.UserID)
		if err != nil {
			return err
		}
		before := acc.Balance
		after := before + in.Points
		if after < 0 {
			return ErrInsufficientPoints
		}
		var expireAt *time.Time
		if in.Points > 0 && in.ValidDays > 0 {
			t := time.Now().AddDate(0, 0, in.ValidDays)
			expireAt = &t
		}
		direction := model.DirectionEarn
		if in.Points < 0 {
			direction = model.DirectionSpend
		}
		record := &model.PointTransaction{
			TxNo:          genTxNo(),
			UserID:        in.UserID,
			Type:          in.TxType,
			Direction:     direction,
			Points:        abs(in.Points),
			BalanceBefore: before,
			BalanceAfter:  after,
			BizType:       in.BizType,
			RefNo:         in.RefNo,
			Remark:        in.Remark,
			OperatorID:    in.OperatorID,
			ExpireAt:      expireAt,
		}
		if err := s.repo.CreateTx(tx, record); err != nil {
			return err
		}
		acc.Balance = after
		if in.Points > 0 {
			acc.TotalEarned += in.Points
		} else {
			acc.TotalSpent += -in.Points
		}
		acc.Version++
		if err := s.repo.SaveAccount(tx, acc); err != nil {
			return err
		}
		result = record
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Adjust 人工调整积分：正数发放、负数扣减，幂等键为人工单号。
func (s *pointService) Adjust(ctx context.Context, req dto.AdjustRequest, operatorID uint64) (*dto.TransactionInfo, error) {
	if req.UserID == 0 || req.Points == 0 {
		return nil, ErrInvalidPoints
	}
	txType := model.TxTypeAdjust
	tx, err := s.grant(ctx, grantInput{
		UserID:     req.UserID,
		TxType:     txType,
		BizType:    "adjust",
		RefNo:      genAdjustNo(),
		Points:     req.Points,
		Remark:     req.Remark,
		OperatorID: operatorID,
	})
	if err != nil {
		return nil, err
	}
	info := buildTransactionInfo(*tx, "")
	return &info, nil
}

// —— 管理端查询 ——

func (s *pointService) Accounts(ctx context.Context, q dto.AccountListQuery) (*dto.AccountListResponse, error) {
	rows, total, err := s.repo.ListAccounts(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]dto.AccountInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildAccountInfo(row))
	}
	return &dto.AccountListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}

func (s *pointService) AccountOf(ctx context.Context, userID uint64) (*dto.AccountInfo, error) {
	acc, err := s.repo.FindAccount(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &dto.AccountInfo{UserID: userID}, nil // 无账户按 0 返回，避免 404
		}
		return nil, err
	}
	info := buildAccountInfo(model.PointAccountRow{PointAccount: *acc})
	return &info, nil
}

func (s *pointService) Transactions(ctx context.Context, q dto.TransactionListQuery) (*dto.TransactionListResponse, error) {
	rows, total, err := s.repo.ListTransactions(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]dto.TransactionInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildTransactionInfo(row.PointTransaction, row.Username))
	}
	return &dto.TransactionListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}

func (s *pointService) Rules(ctx context.Context, q dto.RuleListQuery) (*dto.RuleListResponse, error) {
	items, total, err := s.repo.ListRules(ctx, q)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.RuleInfo, 0, len(items))
	for _, r := range items {
		resp = append(resp, buildRuleInfo(r))
	}
	return &dto.RuleListResponse{
		Items: resp,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}

// CreateRule 新建规则：编码唯一，参数需自洽。
func (s *pointService) CreateRule(ctx context.Context, req dto.RuleSaveRequest) (*dto.RuleInfo, error) {
	rule, err := buildRuleFromRequest(req)
	if err != nil {
		return nil, err
	}
	if rule.Code == "" {
		rule.Code = rule.Scene + "_" + fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	}
	if _, err := s.repo.FindRuleByCode(ctx, rule.Code); err == nil {
		return nil, ErrRuleCodeExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return nil, err
	}
	info := buildRuleInfo(*rule)
	return &info, nil
}

// UpdateRule 更新规则（编码不可改，避免破坏既有幂等语义）。
func (s *pointService) UpdateRule(ctx context.Context, id uint64, req dto.RuleSaveRequest) (*dto.RuleInfo, error) {
	existing, err := s.repo.FindRuleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRuleNotFound
		}
		return nil, err
	}
	updated, err := buildRuleFromRequest(req)
	if err != nil {
		return nil, err
	}
	updated.ID = existing.ID
	updated.Code = existing.Code
	if err := s.repo.UpdateRule(ctx, updated); err != nil {
		return nil, err
	}
	info := buildRuleInfo(*updated)
	return &info, nil
}

// —— 用户端 ——

// MyOverview 我的积分概览：账户 + 生效规则说明 + 最近流水。
func (s *pointService) MyOverview(ctx context.Context, userID uint64) (*dto.MyOverview, error) {
	overview := &dto.MyOverview{UserID: userID, Rules: []dto.RuleInfo{}, Recent: []dto.TransactionInfo{}}
	if acc, err := s.repo.FindAccount(ctx, userID); err == nil {
		overview.Balance = acc.Balance
		overview.Frozen = acc.Frozen
		overview.TotalEarned = acc.TotalEarned
		overview.TotalSpent = acc.TotalSpent
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	enabled := model.RuleStatusEnabled
	if rules, _, err := s.repo.ListRules(ctx, dto.RuleListQuery{Status: &enabled, Page: 1, PageSize: 50}); err == nil {
		for _, r := range rules {
			overview.Rules = append(overview.Rules, buildRuleInfo(r))
		}
	}
	if rows, _, err := s.repo.ListTransactions(ctx, dto.TransactionListQuery{UserID: userID, Page: 1, PageSize: 5}); err == nil {
		for _, row := range rows {
			overview.Recent = append(overview.Recent, buildTransactionInfo(row.PointTransaction, row.Username))
		}
	}
	return overview, nil
}

// MyTransactions 我的积分流水（强制按 userID 过滤，防越权）。
func (s *pointService) MyTransactions(ctx context.Context, userID uint64, q dto.TransactionListQuery) (*dto.TransactionListResponse, error) {
	q.UserID = userID
	return s.Transactions(ctx, q)
}

// Overview 全平台积分概览：余额/累计发放/累计消耗 + 生效规则 + 最近流水。
func (s *pointService) Overview(ctx context.Context) (*dto.OverviewResponse, error) {
	resp := &dto.OverviewResponse{Rules: []dto.RuleInfo{}, Recent: []dto.TransactionInfo{}}
	balance, earned, spent, err := s.repo.SumBalance(ctx)
	if err != nil {
		return nil, err
	}
	resp.TotalBalance, resp.TotalEarned, resp.TotalSpent = balance, earned, spent
	// 账户数（非零余额）与规则数用于概览卡片，失败不阻断页面。
	if _, total, aerr := s.repo.ListAccounts(ctx, dto.AccountListQuery{Page: 1, PageSize: 1}); aerr == nil {
		resp.AccountCount = total
	}
	enabled := model.RuleStatusEnabled
	if rules, _, rerr := s.repo.ListRules(ctx, dto.RuleListQuery{Status: &enabled, Page: 1, PageSize: 20}); rerr == nil {
		for _, r := range rules {
			resp.Rules = append(resp.Rules, buildRuleInfo(r))
		}
	}
	if rows, _, terr := s.repo.ListTransactions(ctx, dto.TransactionListQuery{Page: 1, PageSize: 10}); terr == nil {
		for _, row := range rows {
			resp.Recent = append(resp.Recent, buildTransactionInfo(row.PointTransaction, row.Username))
		}
	}
	return resp, nil
}

// —— 构建器 ——

func buildAccountInfo(row model.PointAccountRow) dto.AccountInfo {
	return dto.AccountInfo{
		ID:          row.ID,
		UserID:      row.UserID,
		Username:    firstNonEmpty(row.Username, fmt.Sprintf("用户%d", row.UserID)),
		Email:       row.Email,
		Balance:     row.Balance,
		Frozen:      row.Frozen,
		TotalEarned: row.TotalEarned,
		TotalSpent:  row.TotalSpent,
		UpdatedAt:   row.UpdatedAt.Format(time.RFC3339),
	}
}

func buildTransactionInfo(tx model.PointTransaction, username string) dto.TransactionInfo {
	return dto.TransactionInfo{
		ID:            tx.ID,
		TxNo:          tx.TxNo,
		UserID:        tx.UserID,
		Username:      username,
		Type:          tx.Type,
		Direction:     tx.Direction,
		Points:        tx.Points,
		BalanceBefore: tx.BalanceBefore,
		BalanceAfter:  tx.BalanceAfter,
		BizType:       tx.BizType,
		RefNo:         tx.RefNo,
		Remark:        tx.Remark,
		OperatorID:    tx.OperatorID,
		ExpireAt:      formatTime(tx.ExpireAt),
		CreatedAt:     tx.CreatedAt.Format(time.RFC3339),
	}
}

func buildRuleInfo(r model.PointRule) dto.RuleInfo {
	return dto.RuleInfo{
		ID:                r.ID,
		Code:              r.Code,
		Name:              r.Name,
		Scene:             r.Scene,
		EarnMode:          r.EarnMode,
		FixedPoints:       r.FixedPoints,
		PointsPerYuan:     r.PointsPerYuan,
		MinAmount:         r.MinAmount,
		MaxPointsPerOrder: r.MaxPointsPerOrder,
		ValidDays:         r.ValidDays,
		Status:            r.Status,
		SortOrder:         r.SortOrder,
		Remark:            r.Remark,
		CreatedAt:         r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         r.UpdatedAt.Format(time.RFC3339),
	}
}

// buildRuleFromRequest 校验并转换规则入参。
func buildRuleFromRequest(req dto.RuleSaveRequest) (*model.PointRule, error) {
	scene := strings.TrimSpace(req.Scene)
	name := strings.TrimSpace(req.Name)
	if scene == "" || name == "" {
		return nil, ErrInvalidRule
	}
	mode := strings.TrimSpace(req.EarnMode)
	if mode != model.EarnModeRate && mode != model.EarnModeFixed {
		return nil, ErrInvalidRule
	}
	if mode == model.EarnModeRate && req.PointsPerYuan <= 0 {
		return nil, ErrInvalidRule
	}
	if mode == model.EarnModeFixed && req.FixedPoints <= 0 {
		return nil, ErrInvalidRule
	}
	status := model.RuleStatusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	return &model.PointRule{
		Code:              strings.TrimSpace(req.Code),
		Name:              name,
		Scene:             scene,
		EarnMode:          mode,
		FixedPoints:       req.FixedPoints,
		PointsPerYuan:     req.PointsPerYuan,
		MinAmount:         req.MinAmount,
		MaxPointsPerOrder: req.MaxPointsPerOrder,
		ValidDays:         req.ValidDays,
		Status:            status,
		SortOrder:         req.SortOrder,
		Remark:            req.Remark,
	}, nil
}

// calcPoints 按规则计算应发积分：rate 模式向下取整，fixed 模式固定值；均受单笔上限约束。
func calcPoints(rule *model.PointRule, amount float64) int64 {
	var points int64
	switch rule.EarnMode {
	case model.EarnModeFixed:
		points = rule.FixedPoints
	default:
		points = int64(math.Floor(amount * rule.PointsPerYuan))
	}
	if rule.MaxPointsPerOrder > 0 && points > rule.MaxPointsPerOrder {
		points = rule.MaxPointsPerOrder
	}
	return points
}

// earnTxType 场景 → 流水类型。
func earnTxType(scene string) string {
	switch scene {
	case model.SceneOrderRenewal:
		return model.TxTypeEarnRenewal
	case model.SceneActivity:
		return model.TxTypeEarnActivity
	default:
		return model.TxTypeEarnPayment
	}
}

func genTxNo() string {
	return fmt.Sprintf("PT%s%04d", time.Now().Format("20060102150405"), rand.Intn(10000))
}

func genAdjustNo() string {
	return fmt.Sprintf("PADJ%s%04d", time.Now().Format("20060102150405"), rand.Intn(10000))
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(size int) int {
	if size <= 0 {
		return 20
	}
	if size > 100 {
		return 100
	}
	return size
}
