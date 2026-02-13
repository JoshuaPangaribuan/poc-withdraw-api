package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/joshuarp/withdraw-api/internal/domain"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
	servicemocks "github.com/joshuarp/withdraw-api/internal/mock/services"
	hashmocks "github.com/joshuarp/withdraw-api/internal/mock/shared/hash"
	jwtmocks "github.com/joshuarp/withdraw-api/internal/mock/shared/jwt"
	sharedjwt "github.com/joshuarp/withdraw-api/internal/shared/jwt"
)

type AuthLoginServiceSuite struct {
	suite.Suite

	repository   *servicemocks.AuthLoginRepository
	hasher       *hashmocks.Hasher
	tokenManager *jwtmocks.TokenManager
	service      *AuthLoginService
}

func (s *AuthLoginServiceSuite) SetupTest() {
	s.repository = servicemocks.NewAuthLoginRepository(s.T())
	s.hasher = hashmocks.NewHasher(s.T())
	s.tokenManager = jwtmocks.NewTokenManager(s.T())
	s.service = NewAuthLoginService(s.repository, s.hasher, s.tokenManager)
}

func (s *AuthLoginServiceSuite) TestLogin_TableDriven() {
	repoErr := errors.New("repository failure")
	signErr := errors.New("sign failed")

	tests := []struct {
		name      string
		email     string
		password  string
		setupMock func()
		assertion func(vo.AuthLogin, error)
	}{
		{
			name:     "invalid when email empty",
			email:    "   ",
			password: "secret",
			assertion: func(result vo.AuthLogin, err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInvalidCredentials)
				assert.Equal(s.T(), vo.AuthLogin{}, result)
			},
		},
		{
			name:     "invalid when password empty",
			email:    "user@example.com",
			password: "   ",
			assertion: func(result vo.AuthLogin, err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInvalidCredentials)
				assert.Equal(s.T(), vo.AuthLogin{}, result)
			},
		},
		{
			name:     "propagate repository error",
			email:    "USER@EXAMPLE.COM",
			password: "secret",
			setupMock: func() {
				s.repository.EXPECT().
					GetUserAuthByEmail(mock.Anything, "user@example.com").
					Return(domain.UserAuth{}, repoErr)
			},
			assertion: func(result vo.AuthLogin, err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, repoErr)
				assert.Equal(s.T(), vo.AuthLogin{}, result)
			},
		},
		{
			name:     "invalid when password mismatch",
			email:    "user@example.com",
			password: "wrong-password",
			setupMock: func() {
				user := domain.UserAuth{ID: "user-1", PasswordHash: "hashed"}
				s.repository.EXPECT().
					GetUserAuthByEmail(mock.Anything, "user@example.com").
					Return(user, nil)
				s.hasher.EXPECT().
					Compare(mock.Anything, "hashed", "wrong-password").
					Return(errors.New("mismatch"))
			},
			assertion: func(result vo.AuthLogin, err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInvalidCredentials)
				assert.Equal(s.T(), vo.AuthLogin{}, result)
			},
		},
		{
			name:     "returns wrapped error when token signing fails",
			email:    "user@example.com",
			password: "secret",
			setupMock: func() {
				user := domain.UserAuth{ID: "user-1", PasswordHash: "hashed"}
				s.repository.EXPECT().
					GetUserAuthByEmail(mock.Anything, "user@example.com").
					Return(user, nil)
				s.hasher.EXPECT().
					Compare(mock.Anything, "hashed", "secret").
					Return(nil)
				s.tokenManager.EXPECT().
					Sign(mock.Anything, mock.MatchedBy(func(claims sharedjwt.Claims) bool {
						return claims.Subject == "user-1"
					})).
					Return("", signErr)
			},
			assertion: func(result vo.AuthLogin, err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "failed to issue token")
				assert.ErrorIs(s.T(), err, signErr)
				assert.Equal(s.T(), vo.AuthLogin{}, result)
			},
		},
		{
			name:     "success",
			email:    " user@example.com ",
			password: "secret",
			setupMock: func() {
				user := domain.UserAuth{ID: "user-1", PasswordHash: "hashed"}
				s.repository.EXPECT().
					GetUserAuthByEmail(mock.Anything, "user@example.com").
					Return(user, nil)
				s.hasher.EXPECT().
					Compare(mock.Anything, "hashed", "secret").
					Return(nil)
				s.tokenManager.EXPECT().Sign(mock.Anything, mock.Anything).Return("signed-token", nil)
			},
			assertion: func(result vo.AuthLogin, err error) {
				require.NoError(s.T(), err)
				assert.Equal(s.T(), "signed-token", result.AccessToken)
				assert.Equal(s.T(), "Bearer", result.TokenType)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setupMock != nil {
				tc.setupMock()
			}

			result, err := s.service.Login(context.Background(), tc.email, tc.password)
			tc.assertion(result, err)
		})
	}
}

func TestAuthLoginServiceSuite(t *testing.T) {
	suite.Run(t, new(AuthLoginServiceSuite))
}

type InquiryCheckBalanceServiceSuite struct {
	suite.Suite

	repository *servicemocks.BalanceInquiryRepository
	service    *InquiryCheckBalanceService
}

func (s *InquiryCheckBalanceServiceSuite) SetupTest() {
	s.repository = servicemocks.NewBalanceInquiryRepository(s.T())
	s.service = NewInquiryCheckBalanceService(s.repository)
}

func (s *InquiryCheckBalanceServiceSuite) TestCheckBalance_TableDriven() {
	repoErr := errors.New("db down")
	now := time.Now().UTC()

	tests := []struct {
		name      string
		userID    string
		setupMock func()
		assertion func(vo.BalanceInquiry, error)
	}{
		{
			name:   "invalid when user id empty",
			userID: "   ",
			assertion: func(result vo.BalanceInquiry, err error) {
				require.Error(s.T(), err)
				assert.EqualError(s.T(), err, "user_id is required")
				assert.Equal(s.T(), vo.BalanceInquiry{}, result)
			},
		},
		{
			name:   "propagates repository error",
			userID: "user-1",
			setupMock: func() {
				s.repository.EXPECT().
					GetWalletBalanceByUserID(mock.Anything, "user-1").
					Return(domain.WalletBalance{}, repoErr)
			},
			assertion: func(result vo.BalanceInquiry, err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, repoErr)
				assert.Equal(s.T(), vo.BalanceInquiry{}, result)
			},
		},
		{
			name:   "success",
			userID: "user-1",
			setupMock: func() {
				s.repository.EXPECT().
					GetWalletBalanceByUserID(mock.Anything, "user-1").
					Return(domain.WalletBalance{UserID: "user-1", BalanceMinor: 1500, Currency: "IDR", UpdatedAt: now}, nil)
			},
			assertion: func(result vo.BalanceInquiry, err error) {
				require.NoError(s.T(), err)
				assert.Equal(s.T(), vo.BalanceInquiry{UserID: "user-1", BalanceMinor: 1500, Currency: "IDR", UpdatedAt: now}, result)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setupMock != nil {
				tc.setupMock()
			}

			result, err := s.service.CheckBalance(context.Background(), tc.userID)
			tc.assertion(result, err)
		})
	}
}

func TestInquiryCheckBalanceServiceSuite(t *testing.T) {
	suite.Run(t, new(InquiryCheckBalanceServiceSuite))
}

type InquiryWithdrawBalanceServiceSuite struct {
	suite.Suite

	repository *servicemocks.BalanceWithdrawRepository
	service    *InquiryWithdrawBalanceService
}

func (s *InquiryWithdrawBalanceServiceSuite) SetupTest() {
	s.repository = servicemocks.NewBalanceWithdrawRepository(s.T())
	s.service = NewInquiryWithdrawBalanceService(s.repository)
}

func (s *InquiryWithdrawBalanceServiceSuite) TestWithdrawBalance_TableDriven() {
	repoErr := errors.New("repository failure")
	now := time.Now().UTC()

	tests := []struct {
		name      string
		userID    string
		amount    int64
		chainID   string
		setupMock func()
		assertion func(vo.WalletWithdrawal, error)
	}{
		{
			name:   "wallet not found when user empty",
			userID: "  ",
			amount: 100,
			assertion: func(result vo.WalletWithdrawal, err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrWalletNotFound)
				assert.Equal(s.T(), vo.WalletWithdrawal{}, result)
			},
		},
		{
			name:   "invalid amount",
			userID: "user-1",
			amount: 0,
			assertion: func(result vo.WalletWithdrawal, err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInvalidAmount)
				assert.Equal(s.T(), vo.WalletWithdrawal{}, result)
			},
		},
		{
			name:    "propagates repository error",
			userID:  "user-1",
			amount:  100,
			chainID: "chain-1",
			setupMock: func() {
				s.repository.EXPECT().
					WithdrawWalletBalanceByUserID(mock.Anything, "user-1", int64(100), "chain-1").
					Return(domain.WalletBalance{}, repoErr)
			},
			assertion: func(result vo.WalletWithdrawal, err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, repoErr)
				assert.Equal(s.T(), vo.WalletWithdrawal{}, result)
			},
		},
		{
			name:    "success",
			userID:  "user-1",
			amount:  100,
			chainID: "chain-1",
			setupMock: func() {
				s.repository.EXPECT().
					WithdrawWalletBalanceByUserID(mock.Anything, "user-1", int64(100), "chain-1").
					Return(domain.WalletBalance{UserID: "user-1", BalanceMinor: 900, Currency: "IDR", UpdatedAt: now}, nil)
			},
			assertion: func(result vo.WalletWithdrawal, err error) {
				require.NoError(s.T(), err)
				assert.Equal(s.T(), vo.WalletWithdrawal{
					UserID:       "user-1",
					AmountMinor:  100,
					BalanceMinor: 900,
					Currency:     "IDR",
					ChainID:      "chain-1",
					UpdatedAt:    now,
				}, result)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setupMock != nil {
				tc.setupMock()
			}

			result, err := s.service.WithdrawBalance(context.Background(), tc.userID, tc.amount, tc.chainID)
			tc.assertion(result, err)
		})
	}
}

func TestInquiryWithdrawBalanceServiceSuite(t *testing.T) {
	suite.Run(t, new(InquiryWithdrawBalanceServiceSuite))
}
