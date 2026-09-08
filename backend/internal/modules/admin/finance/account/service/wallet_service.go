package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	accountmodel "hostsent/backend/internal/modules/admin/finance/account/model"
	accountrepo "hostsent/backend/internal/modules/admin/finance/account/repository"
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

// WalletService 账务核心能力：所有余额变动必须经由本服务（唯一资金入口）。
type WalletService interface {
	// Balance 查询用户可用/冻结余额
	Balance(ctx context.Context, userID uint64) (*accountdto.WalletInfo, error)
	// Change 通用资金变动（收入/支出），事务 + 行锁 + 幂等
	Change(ctx context.Context, req ChangeRequest) (*transmodel.WalletTransaction, error)
	// ListTransactions 资金流水分页
	ListTransactions(ctx context.Context, q transdto.TransactionListQuery) (*transdto.TransactionListResponse, error)
	// Adjust 人工调账（赠送/扣减）
	Adjust(ctx context.Context, req accountdto.AdjustRequest, operatorID uint64) (*transdto.TransactionInfo, error)
}

type walletService struct {
	db         *gorm.DB
	walletRepo accountrepo.WalletRepository
	txRepo     transrepo.TransactionRepository
}

// NewWalletService 创建账务核心服务。
func NewWalletService(db *gorm.DB, walletRepo accountrepo.WalletRepository, txRepo transrepo.TransactionRepository) WalletService {
	return &walletService{db: db, walletRepo: walletRepo, txRepo: txRepo}
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
		return nil, ErrInsufficientBalance
	}
	if req.Direction != transmodel.DirectionIncome && req.Direction != transmodel.DirectionExpense {
		return nil, ErrStatusConflict
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

func (s *walletService) ListTransactions(ctx context.Context, q transdto.TransactionListQuery) (*transdto.TransactionListResponse, error) {
	items, total, err := s.txRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := normalizePage(q.Page)
	pageSize := normalizePageSize(q.PageSize)
	resp := make([]transdto.TransactionInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildTransactionInfo(item))
	}
	return &transdto.TransactionListResponse{Items: resp, Meta: transdto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *walletService) Adjust(ctx context.Context, req accountdto.AdjustRequest, operatorID uint64) (*transdto.TransactionInfo, error) {
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
	info := buildTransactionInfo(*tr)
	return &info, nil
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
