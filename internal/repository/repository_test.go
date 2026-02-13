package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/joshuarp/withdraw-api/internal/domain/vo"
)

func newSQLXMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mockDB, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return sqlx.NewDb(sqlDB, "sqlmock"), mockDB
}

type AuthLoginRepositorySuite struct{ suite.Suite }

func (s *AuthLoginRepositorySuite) TestGetUserAuthByEmail_TableDriven() {
	repoErr := errors.New("query failed")

	tests := []struct {
		name      string
		email     string
		setupMock func(sqlmock.Sqlmock)
		assertion func(error)
	}{
		{
			name:  "invalid when email empty",
			email: "   ",
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInvalidCredentials)
			},
		},
		{
			name:  "invalid when user not found",
			email: "user@example.com",
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectQuery(regexp.QuoteMeta("SELECT id::text AS id, email, password_hash, status")).
					WithArgs("user@example.com").
					WillReturnError(sql.ErrNoRows)
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInvalidCredentials)
			},
		},
		{
			name:  "wraps query errors",
			email: "user@example.com",
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectQuery(regexp.QuoteMeta("SELECT id::text AS id, email, password_hash, status")).
					WithArgs("user@example.com").
					WillReturnError(repoErr)
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "get user auth by email failed")
				assert.ErrorIs(s.T(), err, repoErr)
			},
		},
		{
			name:  "invalid when status not active",
			email: "user@example.com",
			setupMock: func(mockDB sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "password_hash", "status"}).
					AddRow("user-1", "user@example.com", "hashed", "inactive")
				mockDB.ExpectQuery(regexp.QuoteMeta("SELECT id::text AS id, email, password_hash, status")).
					WithArgs("user@example.com").
					WillReturnRows(rows)
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInvalidCredentials)
			},
		},
		{
			name:  "success",
			email: "user@example.com",
			setupMock: func(mockDB sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "password_hash", "status"}).
					AddRow("user-1", "user@example.com", "hashed", "active")
				mockDB.ExpectQuery(regexp.QuoteMeta("SELECT id::text AS id, email, password_hash, status")).
					WithArgs("user@example.com").
					WillReturnRows(rows)
			},
			assertion: func(err error) {
				require.NoError(s.T(), err)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			db, mockDB := newSQLXMock(s.T())
			repo := NewAuthLoginRepository(db)
			if tc.setupMock != nil {
				tc.setupMock(mockDB)
			}

			result, err := repo.GetUserAuthByEmail(context.Background(), tc.email)
			tc.assertion(err)
			if err == nil {
				assert.Equal(s.T(), "user-1", result.ID)
				assert.Equal(s.T(), "user@example.com", result.Email)
			}
			require.NoError(s.T(), mockDB.ExpectationsWereMet())
		})
	}
}

func TestAuthLoginRepositorySuite(t *testing.T) {
	suite.Run(t, new(AuthLoginRepositorySuite))
}

type InquiryCheckBalanceRepositorySuite struct{ suite.Suite }

func (s *InquiryCheckBalanceRepositorySuite) TestGetWalletBalanceByUserID_TableDriven() {
	repoErr := errors.New("query failed")
	userID := uuid.New()
	now := time.Now().UTC()

	tests := []struct {
		name      string
		userID    string
		setupMock func(sqlmock.Sqlmock)
		assertion func(error)
	}{
		{
			name:   "invalid user id",
			userID: "not-uuid",
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "invalid user_id")
			},
		},
		{
			name:   "wallet not found",
			userID: userID.String(),
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectQuery(regexp.QuoteMeta("SELECT\n    w.user_id::text AS user_id")).
					WithArgs(userID).
					WillReturnError(sql.ErrNoRows)
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrWalletNotFound)
			},
		},
		{
			name:   "wrap query errors",
			userID: userID.String(),
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectQuery(regexp.QuoteMeta("SELECT\n    w.user_id::text AS user_id")).
					WithArgs(userID).
					WillReturnError(repoErr)
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "get wallet balance by user_id failed")
				assert.ErrorIs(s.T(), err, repoErr)
			},
		},
		{
			name:   "success",
			userID: userID.String(),
			setupMock: func(mockDB sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"user_id", "balance_minor", "currency", "updated_at"}).
					AddRow(userID.String(), int64(2000), "IDR", now)
				mockDB.ExpectQuery(regexp.QuoteMeta("SELECT\n    w.user_id::text AS user_id")).
					WithArgs(userID).
					WillReturnRows(rows)
			},
			assertion: func(err error) {
				require.NoError(s.T(), err)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			db, mockDB := newSQLXMock(s.T())
			repo := NewInquiryCheckBalanceRepository(db)
			if tc.setupMock != nil {
				tc.setupMock(mockDB)
			}

			result, err := repo.GetWalletBalanceByUserID(context.Background(), tc.userID)
			tc.assertion(err)
			if err == nil {
				assert.Equal(s.T(), userID.String(), result.UserID)
				assert.Equal(s.T(), int64(2000), result.BalanceMinor)
				assert.Equal(s.T(), "IDR", result.Currency)
				assert.Equal(s.T(), now, result.UpdatedAt)
			}
			require.NoError(s.T(), mockDB.ExpectationsWereMet())
		})
	}
}

func TestInquiryCheckBalanceRepositorySuite(t *testing.T) {
	suite.Run(t, new(InquiryCheckBalanceRepositorySuite))
}

type WithdrawBalanceRepositorySuite struct{ suite.Suite }

func (s *WithdrawBalanceRepositorySuite) TestWithdrawWalletBalanceByUserID_TableDriven() {
	userUUID := uuid.New()
	walletUUID := uuid.New()
	now := time.Now().UTC()
	beginErr := errors.New("begin failed")
	withdrawErr := errors.New("withdraw failed")
	hasWalletErr := errors.New("exist check failed")
	insertLedgerErr := errors.New("insert ledger failed")
	commitErr := errors.New("commit failed")

	tests := []struct {
		name      string
		userID    string
		amount    int64
		chainID   string
		setupMock func(sqlmock.Sqlmock)
		assertion func(error)
	}{
		{
			name:   "invalid user id",
			userID: "not-uuid",
			amount: 100,
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "invalid user_id")
			},
		},
		{
			name:   "invalid amount",
			userID: userUUID.String(),
			amount: 0,
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInvalidAmount)
			},
		},
		{
			name:   "begin transaction failed",
			userID: userUUID.String(),
			amount: 100,
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectBegin().WillReturnError(beginErr)
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "failed to start transaction")
				assert.ErrorIs(s.T(), err, beginErr)
			},
		},
		{
			name:   "wallet not found",
			userID: userUUID.String(),
			amount: 100,
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectBegin()
				mockDB.ExpectQuery("UPDATE wallets").WithArgs(int64(100), userUUID).WillReturnError(sql.ErrNoRows)
				rows := sqlmock.NewRows([]string{"exists"}).AddRow(false)
				mockDB.ExpectQuery("SELECT EXISTS").WithArgs(userUUID).WillReturnRows(rows)
				mockDB.ExpectRollback()
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrWalletNotFound)
			},
		},
		{
			name:   "has wallet check failed",
			userID: userUUID.String(),
			amount: 100,
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectBegin()
				mockDB.ExpectQuery("UPDATE wallets").WithArgs(int64(100), userUUID).WillReturnError(sql.ErrNoRows)
				mockDB.ExpectQuery("SELECT EXISTS").WithArgs(userUUID).WillReturnError(hasWalletErr)
				mockDB.ExpectRollback()
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "failed to check wallet existence")
				assert.ErrorIs(s.T(), err, hasWalletErr)
			},
		},
		{
			name:   "insufficient balance",
			userID: userUUID.String(),
			amount: 100,
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectBegin()
				mockDB.ExpectQuery("UPDATE wallets").WithArgs(int64(100), userUUID).WillReturnError(sql.ErrNoRows)
				rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
				mockDB.ExpectQuery("SELECT EXISTS").WithArgs(userUUID).WillReturnRows(rows)
				mockDB.ExpectRollback()
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorIs(s.T(), err, vo.ErrInsufficientBalance)
			},
		},
		{
			name:   "withdraw query failed",
			userID: userUUID.String(),
			amount: 100,
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectBegin()
				mockDB.ExpectQuery("UPDATE wallets").WithArgs(int64(100), userUUID).WillReturnError(withdrawErr)
				mockDB.ExpectRollback()
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "failed to withdraw wallet balance")
				assert.ErrorIs(s.T(), err, withdrawErr)
			},
		},
		{
			name:    "insert ledger failed",
			userID:  userUUID.String(),
			amount:  100,
			chainID: "chain-1",
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectBegin()
				walletRows := sqlmock.NewRows([]string{"wallet_id", "user_id", "balance_minor", "currency", "updated_at"}).
					AddRow(walletUUID, userUUID.String(), int64(900), "IDR", now)
				mockDB.ExpectQuery("UPDATE wallets").WithArgs(int64(100), userUUID).WillReturnRows(walletRows)
				mockDB.ExpectExec("INSERT INTO wallet_ledger").WithArgs(walletUUID, "withdrawal", int64(-100), int64(900), sql.NullString{}, sql.NullString{String: "chain-1", Valid: true}).WillReturnError(insertLedgerErr)
				mockDB.ExpectRollback()
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "failed to insert wallet ledger")
				assert.ErrorIs(s.T(), err, insertLedgerErr)
			},
		},
		{
			name:   "commit failed",
			userID: userUUID.String(),
			amount: 100,
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectBegin()
				walletRows := sqlmock.NewRows([]string{"wallet_id", "user_id", "balance_minor", "currency", "updated_at"}).
					AddRow(walletUUID, userUUID.String(), int64(900), "IDR", now)
				mockDB.ExpectQuery("UPDATE wallets").WithArgs(int64(100), userUUID).WillReturnRows(walletRows)
				mockDB.ExpectExec("INSERT INTO wallet_ledger").WillReturnResult(sqlmock.NewResult(1, 1))
				mockDB.ExpectCommit().WillReturnError(commitErr)
			},
			assertion: func(err error) {
				require.Error(s.T(), err)
				assert.ErrorContains(s.T(), err, "failed to commit transaction")
				assert.ErrorIs(s.T(), err, commitErr)
			},
		},
		{
			name:   "success",
			userID: userUUID.String(),
			amount: 100,
			setupMock: func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectBegin()
				walletRows := sqlmock.NewRows([]string{"wallet_id", "user_id", "balance_minor", "currency", "updated_at"}).
					AddRow(walletUUID, userUUID.String(), int64(900), "IDR", now)
				mockDB.ExpectQuery("UPDATE wallets").WithArgs(int64(100), userUUID).WillReturnRows(walletRows)
				mockDB.ExpectExec("INSERT INTO wallet_ledger").WillReturnResult(sqlmock.NewResult(1, 1))
				mockDB.ExpectCommit()
			},
			assertion: func(err error) {
				require.NoError(s.T(), err)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			db, mockDB := newSQLXMock(s.T())
			repo := NewWithdrawBalanceRepository(db)
			if tc.setupMock != nil {
				tc.setupMock(mockDB)
			}

			result, err := repo.WithdrawWalletBalanceByUserID(context.Background(), tc.userID, tc.amount, tc.chainID)
			tc.assertion(err)
			if err == nil {
				assert.Equal(s.T(), userUUID.String(), result.UserID)
				assert.Equal(s.T(), int64(900), result.BalanceMinor)
				assert.Equal(s.T(), "IDR", result.Currency)
				assert.Equal(s.T(), now, result.UpdatedAt)
			}
			require.NoError(s.T(), mockDB.ExpectationsWereMet())
		})
	}
}

func TestWithdrawBalanceRepositorySuite(t *testing.T) {
	suite.Run(t, new(WithdrawBalanceRepositorySuite))
}
