package services

import (
	"context"
	"strings"

	"github.com/joshuarp/withdraw-api/internal/domain"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
)

type BalanceWithdrawRepository interface {
	WithdrawWalletBalanceByUserID(ctx context.Context, userID string, amountMinor int64, chainID string) (domain.WalletBalance, error)
}

type InquiryWithdrawBalanceService struct {
	repository BalanceWithdrawRepository
}

func NewInquiryWithdrawBalanceService(repository BalanceWithdrawRepository) *InquiryWithdrawBalanceService {
	return &InquiryWithdrawBalanceService{repository: repository}
}

func (s *InquiryWithdrawBalanceService) WithdrawBalance(ctx context.Context, userID string, amountMinor int64, chainID string) (vo.WalletWithdrawal, error) {
	if strings.TrimSpace(userID) == "" {
		return vo.WalletWithdrawal{}, vo.ErrWalletNotFound
	}

	if amountMinor <= 0 {
		return vo.WalletWithdrawal{}, vo.ErrInvalidAmount
	}

	balance, err := s.repository.WithdrawWalletBalanceByUserID(ctx, userID, amountMinor, chainID)
	if err != nil {
		return vo.WalletWithdrawal{}, err
	}

	return vo.WalletWithdrawal{
		UserID:       balance.UserID,
		AmountMinor:  amountMinor,
		BalanceMinor: balance.BalanceMinor,
		Currency:     balance.Currency,
		ChainID:      chainID,
		UpdatedAt:    balance.UpdatedAt,
	}, nil
}
