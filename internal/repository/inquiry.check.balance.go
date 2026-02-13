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

type InquiryCheckBalanceRepository struct {
	db      *sqlx.DB
	queries *sharedsqlc.Queries
}

func NewInquiryCheckBalanceRepository(db *sqlx.DB) *InquiryCheckBalanceRepository {
	return &InquiryCheckBalanceRepository{db: db, queries: sharedsqlc.New(db.DB)}
}

func (r *InquiryCheckBalanceRepository) GetWalletBalanceByUserID(ctx context.Context, userID string) (domain.WalletBalance, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return domain.WalletBalance{}, fmt.Errorf("repository: invalid user_id: %w", err)
	}

	balanceRow, err := r.queries.GetWalletBalanceByUserID(ctx, parsedUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.WalletBalance{}, vo.ErrWalletNotFound
		}
		return domain.WalletBalance{}, fmt.Errorf("repository: get wallet balance by user_id failed: %w", err)
	}

	return domain.WalletBalance{
		UserID:       balanceRow.UserID,
		BalanceMinor: balanceRow.BalanceMinor,
		Currency:     balanceRow.Currency,
		UpdatedAt:    balanceRow.UpdatedAt,
	}, nil
}
