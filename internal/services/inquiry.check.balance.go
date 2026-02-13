package services

import (
	"context"
	"errors"
	"strings"

	"github.com/joshuarp/withdraw-api/internal/domain"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
)

type BalanceInquiryRepository interface {
	GetWalletBalanceByUserID(ctx context.Context, userID string) (domain.WalletBalance, error)
}

type InquiryCheckBalanceService struct {
	repository BalanceInquiryRepository
}

func NewInquiryCheckBalanceService(repository BalanceInquiryRepository) *InquiryCheckBalanceService {
	return &InquiryCheckBalanceService{repository: repository}
}

func (s *InquiryCheckBalanceService) CheckBalance(ctx context.Context, userID string) (vo.BalanceInquiry, error) {
	if strings.TrimSpace(userID) == "" {
		return vo.BalanceInquiry{}, errors.New("user_id is required")
	}

	balance, err := s.repository.GetWalletBalanceByUserID(ctx, userID)
	if err != nil {
		return vo.BalanceInquiry{}, err
	}

	return vo.BalanceInquiry{
		UserID:       balance.UserID,
		BalanceMinor: balance.BalanceMinor,
		Currency:     balance.Currency,
		UpdatedAt:    balance.UpdatedAt,
	}, nil
}
