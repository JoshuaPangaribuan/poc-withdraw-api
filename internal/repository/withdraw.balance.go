package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/joshuarp/withdraw-api/internal/domain"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
	sharedsqlc "github.com/joshuarp/withdraw-api/internal/shared/sqlc"
)

type WithdrawBalanceRepository struct {
	db      *sqlx.DB
	queries *sharedsqlc.Queries
}

func NewWithdrawBalanceRepository(db *sqlx.DB) *WithdrawBalanceRepository {
	return &WithdrawBalanceRepository{db: db, queries: sharedsqlc.New(db.DB)}
}

func (r *WithdrawBalanceRepository) WithdrawWalletBalanceByUserID(ctx context.Context, userID string, amountMinor int64, chainID string) (domain.WalletBalance, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return domain.WalletBalance{}, fmt.Errorf("repository: invalid user_id: %w", err)
	}

	if amountMinor <= 0 {
		return domain.WalletBalance{}, vo.ErrInvalidAmount
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.WalletBalance{}, fmt.Errorf("repository: failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	queriesWithTx := r.queries.WithTx(tx.Tx)
	withdrawnWallet, err := queriesWithTx.WithdrawWalletBalanceByUserID(ctx, sharedsqlc.WithdrawWalletBalanceByUserIDParams{
		AmountMinor: amountMinor,
		UserID:      parsedUserID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			exists, existsErr := queriesWithTx.HasWalletByUserID(ctx, parsedUserID)
			if existsErr != nil {
				return domain.WalletBalance{}, fmt.Errorf("repository: failed to check wallet existence: %w", existsErr)
			}

			if !exists {
				return domain.WalletBalance{}, vo.ErrWalletNotFound
			}

			return domain.WalletBalance{}, vo.ErrInsufficientBalance
		}

		return domain.WalletBalance{}, fmt.Errorf("repository: failed to withdraw wallet balance: %w", err)
	}

	ledgerParams := sharedsqlc.InsertWalletLedgerParams{
		WalletID:          withdrawnWallet.WalletID,
		EntryType:         "withdrawal",
		AmountMinor:       -amountMinor,
		BalanceAfterMinor: withdrawnWallet.BalanceMinor,
	}

	if chainID != "" {
		ledgerParams.ChainID = sql.NullString{String: chainID, Valid: true}
	}

	if err := queriesWithTx.InsertWalletLedger(ctx, ledgerParams); err != nil {
		return domain.WalletBalance{}, fmt.Errorf("repository: failed to insert wallet ledger: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.WalletBalance{}, fmt.Errorf("repository: failed to commit transaction: %w", err)
	}

	return domain.WalletBalance{
		UserID:       withdrawnWallet.UserID,
		BalanceMinor: withdrawnWallet.BalanceMinor,
		Currency:     withdrawnWallet.Currency,
		UpdatedAt:    withdrawnWallet.UpdatedAt,
	}, nil
}
