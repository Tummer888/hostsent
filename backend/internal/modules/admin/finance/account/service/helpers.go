package service

import (
	"fmt"
	"math/rand"
	"time"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	accountmodel "hostsent/backend/internal/modules/admin/finance/account/model"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
)

// genTxNo 生成流水号，如 20260908W00001。
func genTxNo() string {
	return fmt.Sprintf("W%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

// buildWalletInfo 构建钱包信息 DTO。
func buildWalletInfo(acc *accountmodel.WalletAccount) *accountdto.WalletInfo {
	return &accountdto.WalletInfo{
		UserID:       acc.UserID,
		Balance:      acc.Balance,
		Frozen:       acc.Frozen,
		TotalIncome:  acc.TotalIncome,
		TotalExpense: acc.TotalExpense,
		Version:      acc.Version,
	}
}

// buildTransactionInfo 构建资金流水 DTO。
func buildTransactionInfo(tx transmodel.WalletTransaction) transdto.TransactionInfo {
	return transdto.TransactionInfo{
		ID:            tx.ID,
		TxNo:          tx.TxNo,
		UserID:        tx.UserID,
		Type:          tx.Type,
		Direction:     tx.Direction,
		Amount:        tx.Amount,
		BalanceBefore: tx.BalanceBefore,
		BalanceAfter:  tx.BalanceAfter,
		OrderID:       tx.OrderID,
		OrderNo:       tx.OrderNo,
		RefNo:         tx.RefNo,
		Remark:        tx.Remark,
		OperatorID:    tx.OperatorID,
		CreatedAt:     tx.CreatedAt.Format(time.RFC3339),
	}
}
