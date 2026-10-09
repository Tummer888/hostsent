package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	accountmodel "hostsent/backend/internal/modules/admin/finance/account/model"
	accountrepo "hostsent/backend/internal/modules/admin/finance/account/repository"
	settingsmodel "hostsent/backend/internal/modules/admin/finance/settings/model"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	transrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	"hostsent/backend/internal/pkg/money"
)

// ChangeRequest 通用资金变动请求。biz 幂等键由调用方提供（如 ref_no + type）。
type ChangeRequest struct {
	UserID     uint64
	Type       string
	Direction  int
	Amount     float64
	OrderID    uint64
	OrderNo    string
	RefNo      string
	BizType    string
	Remark     string
	OperatorID uint64
}

// FreezeRequest 冻结/解冻/结算冻结资金请求。
type FreezeRequest struct {
	UserID     uint64
	Amount     float64
	RefNo      string
	BizType    string
	TxType     string
	Remark     string
	OperatorID uint64
}

// WalletService 账务核心能力：所有余额变动必须经由本服务（唯一资金入口）。
type WalletService interface {
	// Balance 查询用户可用/冻结余额
	Balance(ctx context.Context, userID uint64) (*accountdto.WalletInfo, error)
	// Change 通用资金变动（收入/支出），事务 + 行锁 + 幂等
	Change(ctx context.Context, req ChangeRequest) (*transmodel.WalletTransaction, error)
	// ListTransactions 资金流水分页（含筛选口径汇总）
	ListTransactions(ctx context.Context, q transdto.TransactionListQuery) (*transdto.TransactionListResponse, error)
	// ExportTransactions 导出筛选后的流水明细（按时间正序，limit 条封顶）
	ExportTransactions(ctx context.Context, q transdto.TransactionListQuery, limit int) ([]transdto.TransactionInfo, error)
	// Adjust 人工调账（赠送/扣减）
	Adjust(ctx context.Context, req accountdto.AdjustRequest, operatorID uint64) (*transdto.TransactionInfo, error)
	// Freeze 冻结：可用余额 → 冻结余额（提现申请/预授权），不计入累计支出
	Freeze(ctx context.Context, req FreezeRequest) (*transmodel.WalletTransaction, error)
	// Unfreeze 解冻：冻结余额 → 可用余额（提现驳回/打款失败）
	Unfreeze(ctx context.Context, req FreezeRequest) (*transmodel.WalletTransaction, error)
	// SettleFrozen 结算冻结：冻结余额 → 真实支出（打款成功），计入累计支出
	SettleFrozen(ctx context.Context, req FreezeRequest) (*transmodel.WalletTransaction, error)
	// SetConfigReader 注入配置读取器（finance.adjust_enabled）：装配层接线
	SetConfigReader(reader ConfigReader)
}

// ConfigReader 读取 system_configs 原文（键不存在返回 ok=false，不报错，沿用装配层注入约定）。
type ConfigReader func(ctx context.Context, key string) (string, bool, error)

// ConfigKeyAdjustEnabled 人工调账开关（键定义见 settings/model，唯一定义处）。
const ConfigKeyAdjustEnabled = settingsmodel.ConfigKeyAdjustEnabled

type walletService struct {
	db         *gorm.DB
	walletRepo accountrepo.WalletRepository
	txRepo     transrepo.TransactionRepository
	// configReader 读财务配置；未注入时按「允许调账」处理。
	configReader ConfigReader
}

// NewWalletService 创建账务核心服务。
func NewWalletService(db *gorm.DB, walletRepo accountrepo.WalletRepository, txRepo transrepo.TransactionRepository) WalletService {
	return &walletService{db: db, walletRepo: walletRepo, txRepo: txRepo}
}

// SetConfigReader 注入配置读取器（装配层提供，避免账务核心 import 系统配置模块）。
func (s *walletService) SetConfigReader(reader ConfigReader) {
	s.configReader = reader
}

func (s *walletService) Balance(ctx context.Context, userID uint64) (*accountdto.WalletInfo, error) {
	acc, err := s.walletRepo.FindByUser(s.db, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 尚无账户时按 0 返回，避免查询 404
			return &accountdto.WalletInfo{UserID: userID}, nil
		}
		return nil, err
	}
	return buildWalletInfo(acc), nil
}

// Change 在数据库事务内完成：账户加锁 → 校验 → 更新余额 → 写流水 → 同步 users.balance。
// 同一 (user_id, biz_type, ref_no) 来源只记一次账，重复调用返回已有流水（幂等）。
func (s *walletService) Change(ctx context.Context, req ChangeRequest) (*transmodel.WalletTransaction, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if req.Direction != transmodel.DirectionIncome && req.Direction != transmodel.DirectionExpense {
		return nil, ErrInvalidDirection
	}

	var result *transmodel.WalletTransaction
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 行锁账户（防止并发超扣）
		acc, err := s.walletRepo.LockByUser(tx, req.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				acc = &accountmodel.WalletAccount{UserID: req.UserID}
				if cerr := s.walletRepo.Create(tx, acc); cerr != nil {
					return cerr
				}
			} else {
				return err
			}
		}

		// 2. 幂等：同业务同来源已记账则跳过
		existing, err := s.txRepo.FindByBiz(tx, req.UserID, req.BizType, req.RefNo)
		if err == nil {
			result = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 3. 余额校验（支出方向不可超余额）
		before := acc.Balance
		if req.Direction < 0 && req.Amount > before {
			return ErrInsufficientBalance
		}

		// 4. 更新余额
		after := money.Round2(before + float64(req.Direction)*req.Amount)
		acc.Balance = after
		if req.Direction > 0 {
			acc.TotalIncome = money.Round2(acc.TotalIncome + req.Amount)
		} else {
			acc.TotalExpense = money.Round2(acc.TotalExpense + req.Amount)
		}
		acc.Version++
		if err := s.walletRepo.Update(tx, acc); err != nil {
			return err
		}

		// 5. 同一事务同步 users.balance，避免两处余额漂移
		if err := s.walletRepo.SyncUsersBalance(tx, req.UserID, after); err != nil {
			return err
		}

		// 6. 写流水（只增不改）
		record := &transmodel.WalletTransaction{
			TxNo:          genTxNo(),
			UserID:        req.UserID,
			Type:          req.Type,
			Direction:     req.Direction,
			Amount:        money.Round2(req.Amount),
			BalanceBefore: before,
			BalanceAfter:  after,
			OrderID:       req.OrderID,
			OrderNo:       req.OrderNo,
			RefNo:         req.RefNo,
			BizType:       req.BizType,
			Remark:        req.Remark,
			OperatorID:    req.OperatorID,
		}
		if err := s.txRepo.Create(tx, record); err != nil {
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

// Freeze 冻结：可用余额 → 冻结余额。不计入累计支出，流水方向按支出侧记 0 额
// 变动金额仍记冻结额，但 direction 记为 0 会破坏既有约束，故按「支出方向 + 冻结标记」
// 记账：Amount 记冻结额、余额不变，仅冻结列增减。
func (s *walletService) Freeze(ctx context.Context, req FreezeRequest) (*transmodel.WalletTransaction, error) {
	return s.moveFrozen(ctx, req, "freeze")
}

// Unfreeze 解冻：冻结余额 → 可用余额。
func (s *walletService) Unfreeze(ctx context.Context, req FreezeRequest) (*transmodel.WalletTransaction, error) {
	return s.moveFrozen(ctx, req, "unfreeze")
}

// SettleFrozen 结算冻结：冻结余额 → 真实支出（打款成功）。从冻结列扣减，
// 余额不再次扣减（冻结时已从可用余额划出），累计支出累加。
func (s *walletService) SettleFrozen(ctx context.Context, req FreezeRequest) (*transmodel.WalletTransaction, error) {
	return s.moveFrozen(ctx, req, "settle")
}

// moveFrozen 冻结资金状态迁移的统一实现（事务 + 行锁 + 幂等）。
//
//	freeze  ：available -= amount, frozen += amount
//	unfreeze：available += amount, frozen -= amount
//	settle  ：frozen    -= amount（available 不变，累计支出 += amount）
func (s *walletService) moveFrozen(ctx context.Context, req FreezeRequest, action string) (*transmodel.WalletTransaction, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}
	bizType := req.BizType
	if bizType == "" {
		bizType = "freeze"
	}
	txType := req.TxType
	if txType == "" {
		switch action {
		case "freeze":
			txType = transmodel.TxTypeFreeze
		case "unfreeze":
			txType = transmodel.TxTypeUnfreeze
		default:
			txType = transmodel.TxTypeSettlement
		}
	}
	var result *transmodel.WalletTransaction
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.walletRepo.LockByUser(tx, req.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				acc = &accountmodel.WalletAccount{UserID: req.UserID}
				if cerr := s.walletRepo.Create(tx, acc); cerr != nil {
					return cerr
				}
			} else {
				return err
			}
		}
		// 幂等：同业务同来源只处理一次
		if existing, err := s.txRepo.FindByBiz(tx, req.UserID, bizType, req.RefNo); err == nil {
			result = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		amount := money.Round2(req.Amount)
		available := acc.Balance
		frozen := acc.Frozen
		switch action {
		case "freeze":
			if amount > available {
				return ErrInsufficientBalance
			}
			acc.Balance = money.Round2(available - amount)
			acc.Frozen = money.Round2(frozen + amount)
		case "unfreeze":
			if amount > frozen {
				return ErrFrozenInsufficient
			}
			acc.Balance = money.Round2(available + amount)
			acc.Frozen = money.Round2(frozen - amount)
		case "settle":
			if amount > frozen {
				return ErrFrozenInsufficient
			}
			acc.Frozen = money.Round2(frozen - amount)
			acc.TotalExpense = money.Round2(acc.TotalExpense + amount)
		}
		acc.Version++
		if err := s.walletRepo.Update(tx, acc); err != nil {
			return err
		}
		if err := s.walletRepo.SyncUsersBalance(tx, req.UserID, acc.Balance); err != nil {
			return err
		}
		dir := transmodel.DirectionExpense
		if action == "unfreeze" {
			dir = transmodel.DirectionIncome
		}
		record := &transmodel.WalletTransaction{
			TxNo:          genTxNo(),
			UserID:        req.UserID,
			Type:          txType,
			Direction:     dir,
			Amount:        money.Round2(req.Amount),
			BalanceBefore: available,
			BalanceAfter:  acc.Balance,
			RefNo:         req.RefNo,
			BizType:       bizType,
			Remark:        req.Remark,
			OperatorID:    req.OperatorID,
		}
		if err := s.txRepo.Create(tx, record); err != nil {
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

func (s *walletService) ListTransactions(ctx context.Context, q transdto.TransactionListQuery) (*transdto.TransactionListResponse, error) {
	items, total, err := s.txRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	// 汇总与列表同口径（同一组筛选条件、全量、非当前页），页面汇总条因此不会与明细互相矛盾。
	summary, err := s.txRepo.Summary(ctx, q)
	if err != nil {
		return nil, err
	}
	page := normalizePage(q.Page)
	pageSize := normalizePageSize(q.PageSize)
	resp := make([]transdto.TransactionInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildTransactionInfo(item.WalletTransaction, item.Username))
	}
	return &transdto.TransactionListResponse{
		Items:   resp,
		Meta:    transdto.ListMeta{Page: page, PageSize: pageSize, Total: total},
		Summary: buildTransactionSummary(*summary),
	}, nil
}

// ExportTransactions 导出筛选后的流水明细（按时间正序、条数封顶），供财务留档与外部对账。
func (s *walletService) ExportTransactions(ctx context.Context, q transdto.TransactionListQuery, limit int) ([]transdto.TransactionInfo, error) {
	rows, err := s.txRepo.ListForExport(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	resp := make([]transdto.TransactionInfo, 0, len(rows))
	for _, item := range rows {
		resp = append(resp, buildTransactionInfo(item.WalletTransaction, item.Username))
	}
	return resp, nil
}

func buildTransactionSummary(row transrepo.TransactionSummaryRow) transdto.TransactionSummary {
	return transdto.TransactionSummary{
		IncomeTotal: money.Round2(row.Income), ExpenseTotal: money.Round2(row.Expense),
		NetTotal:      money.Round2(row.Income - row.Expense),
		TxCount:       row.Count,
		InternalCount: row.InternalCount,
	}
}

func (s *walletService) Adjust(ctx context.Context, req accountdto.AdjustRequest, operatorID uint64) (*transdto.TransactionInfo, error) {
	if !s.adjustEnabled(ctx) {
		return nil, ErrAdjustDisabled
	}
	txType := req.Type
	if txType == "" {
		txType = transmodel.TxTypeAdjust
	}
	tr, err := s.Change(ctx, ChangeRequest{
		UserID:     req.UserID,
		Type:       txType,
		Direction:  req.Direction,
		Amount:     req.Amount,
		RefNo:      req.BizKey,
		BizType:    transmodel.TxTypeAdjust,
		Remark:     req.Remark,
		OperatorID: operatorID,
	})
	if err != nil {
		return nil, err
	}
	info := buildTransactionInfo(*tr, "")
	return &info, nil
}

// adjustEnabled 人工调账开关：读财务配置（finance.adjust_enabled），
// 键缺失、读取失败或取值非法时按「允许」处理，避免配置问题把账务入口锁死。
func (s *walletService) adjustEnabled(ctx context.Context) bool {
	if s.configReader == nil {
		return true
	}
	raw, ok, err := s.configReader(ctx, ConfigKeyAdjustEnabled)
	if err != nil || !ok {
		return true
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return true
	}
	return enabled
}

// normalizePage/normalizePageSize 复用 repository 中的分页归一化逻辑。
func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return 10
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}
