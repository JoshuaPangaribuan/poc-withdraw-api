package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	handlermocks "github.com/joshuarp/withdraw-api/internal/mock/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/joshuarp/withdraw-api/internal/domain/vo"
	"github.com/joshuarp/withdraw-api/internal/middlewares"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func performJSONRequest(app *fiber.App, method, path string, body []byte, headers map[string]string) (*http.Response, map[string]interface{}, []byte) {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if len(body) > 0 {
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := app.Test(req)
	if err != nil {
		return nil, nil, nil
	}

	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	parsed := map[string]interface{}{}
	_ = json.Unmarshal(rawBody, &parsed)

	return resp, parsed, rawBody
}

type AuthLoginHandlerSuite struct {
	suite.Suite

	service *handlermocks.AuthLoginService
	handler *AuthLoginHandler
	app     *fiber.App
}

func (s *AuthLoginHandlerSuite) SetupTest() {
	s.service = handlermocks.NewAuthLoginService(s.T())
	s.handler = NewAuthLoginHandler(s.service, newTestLogger())
	s.app = fiber.New()
	s.app.Post("/auth/login", s.handler.Handle)
}

func (s *AuthLoginHandlerSuite) TestHandle_TableDriven() {
	serviceErr := errors.New("service error")

	tests := []struct {
		name      string
		body      []byte
		setupMock func()
		assertion func(*http.Response, map[string]interface{}, []byte)
	}{
		{
			name: "invalid body",
			body: []byte(`{"email":`),
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusBadRequest, resp.StatusCode)
				assert.Equal(s.T(), "invalid request body", payload["error"])
			},
		},
		{
			name: "missing email or password",
			body: []byte(`{"email":"","password":""}`),
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusBadRequest, resp.StatusCode)
				assert.Equal(s.T(), "email and password are required", payload["error"])
			},
		},
		{
			name: "invalid credentials",
			body: []byte(`{"email":"user@example.com","password":"secret"}`),
			setupMock: func() {
				s.service.EXPECT().
					Login(mock.Anything, "user@example.com", "secret").
					Return(vo.AuthLogin{}, vo.ErrInvalidCredentials)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusUnauthorized, resp.StatusCode)
				assert.Equal(s.T(), "invalid email or password", payload["error"])
			},
		},
		{
			name: "internal error",
			body: []byte(`{"email":"user@example.com","password":"secret"}`),
			setupMock: func() {
				s.service.EXPECT().
					Login(mock.Anything, "user@example.com", "secret").
					Return(vo.AuthLogin{}, serviceErr)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusInternalServerError, resp.StatusCode)
				assert.Equal(s.T(), "internal server error", payload["error"])
			},
		},
		{
			name: "success",
			body: []byte(`{"email":"user@example.com","password":"secret"}`),
			setupMock: func() {
				s.service.EXPECT().
					Login(mock.Anything, "user@example.com", "secret").
					Return(vo.AuthLogin{AccessToken: "token-123", TokenType: "Bearer"}, nil)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusOK, resp.StatusCode)
				assert.Equal(s.T(), "token-123", payload["access_token"])
				assert.Equal(s.T(), "Bearer", payload["token_type"])
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setupMock != nil {
				tc.setupMock()
			}

			resp, payload, raw := performJSONRequest(s.app, http.MethodPost, "/auth/login", tc.body, nil)
			if resp == nil {
				s.T().Fatal("failed to execute request")
			}
			tc.assertion(resp, payload, raw)
		})
	}
}

func TestAuthLoginHandlerSuite(t *testing.T) {
	suite.Run(t, new(AuthLoginHandlerSuite))
}

type InquiryCheckBalanceHandlerSuite struct {
	suite.Suite

	service *handlermocks.BalanceInquiryService
	handler *InquiryCheckBalanceHandler
	app     *fiber.App
}

func (s *InquiryCheckBalanceHandlerSuite) SetupTest() {
	s.service = handlermocks.NewBalanceInquiryService(s.T())
	s.handler = NewInquiryCheckBalanceHandler(s.service, newTestLogger())
	s.app = fiber.New()
}

func (s *InquiryCheckBalanceHandlerSuite) TestHandle_TableDriven() {
	now := time.Now().UTC()
	serviceErr := errors.New("service failed")

	tests := []struct {
		name      string
		userID    string
		setupMock func()
		assertion func(*http.Response, map[string]interface{})
	}{
		{
			name:   "missing authenticated user",
			userID: "",
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusUnauthorized, resp.StatusCode)
				assert.Equal(s.T(), "missing authenticated user", payload["error"])
			},
		},
		{
			name:   "wallet not found",
			userID: "user-1",
			setupMock: func() {
				s.service.EXPECT().CheckBalance(mock.Anything, "user-1").Return(vo.BalanceInquiry{}, vo.ErrWalletNotFound)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusNotFound, resp.StatusCode)
				assert.Equal(s.T(), "wallet not found", payload["error"])
			},
		},
		{
			name:   "internal error",
			userID: "user-1",
			setupMock: func() {
				s.service.EXPECT().CheckBalance(mock.Anything, "user-1").Return(vo.BalanceInquiry{}, serviceErr)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusInternalServerError, resp.StatusCode)
				assert.Equal(s.T(), "internal server error", payload["error"])
			},
		},
		{
			name:   "success",
			userID: "user-1",
			setupMock: func() {
				s.service.EXPECT().CheckBalance(mock.Anything, "user-1").Return(vo.BalanceInquiry{
					UserID:       "user-1",
					BalanceMinor: 1200,
					Currency:     "IDR",
					UpdatedAt:    now,
				}, nil)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusOK, resp.StatusCode)
				assert.Equal(s.T(), "user-1", payload["user_id"])
				assert.Equal(s.T(), float64(1200), payload["balance_minor"])
				assert.Equal(s.T(), "IDR", payload["currency"])
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.app.Get("/inquiries/balance", func(c fiber.Ctx) error {
				if tc.userID != "" {
					c.Locals("user_id", tc.userID)
				}
				return s.handler.Handle(c)
			})
			if tc.setupMock != nil {
				tc.setupMock()
			}

			resp, payload, _ := performJSONRequest(s.app, http.MethodGet, "/inquiries/balance", nil, nil)
			if resp == nil {
				s.T().Fatal("failed to execute request")
			}
			tc.assertion(resp, payload)
		})
	}
}

func TestInquiryCheckBalanceHandlerSuite(t *testing.T) {
	suite.Run(t, new(InquiryCheckBalanceHandlerSuite))
}

type InquiryWithdrawBalanceHandlerSuite struct {
	suite.Suite

	service *handlermocks.BalanceWithdrawService
	handler *InquiryWithdrawBalanceHandler
	app     *fiber.App
}

func (s *InquiryWithdrawBalanceHandlerSuite) SetupTest() {
	s.service = handlermocks.NewBalanceWithdrawService(s.T())
	s.handler = NewInquiryWithdrawBalanceHandler(s.service, newTestLogger())
	s.app = fiber.New()
}

func (s *InquiryWithdrawBalanceHandlerSuite) TestHandle_TableDriven() {
	now := time.Now().UTC()
	serviceErr := errors.New("service failed")

	tests := []struct {
		name      string
		userID    string
		body      []byte
		headers   map[string]string
		setupMock func()
		assertion func(*http.Response, map[string]interface{})
	}{
		{
			name:   "missing authenticated user",
			userID: "",
			body:   []byte(`{"amount_minor":100}`),
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusUnauthorized, resp.StatusCode)
				assert.Equal(s.T(), "missing authenticated user", payload["error"])
			},
		},
		{
			name:   "invalid request body",
			userID: "user-1",
			body:   []byte(`{"amount_minor":`),
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusBadRequest, resp.StatusCode)
				assert.Equal(s.T(), "invalid request body", payload["error"])
			},
		},
		{
			name:   "invalid amount",
			userID: "user-1",
			body:   []byte(`{"amount_minor":0}`),
			setupMock: func() {
				s.service.EXPECT().WithdrawBalance(mock.Anything, "user-1", int64(0), "chain-1").Return(vo.WalletWithdrawal{}, vo.ErrInvalidAmount)
			},
			headers: map[string]string{middlewares.ChainIDHeader: "chain-1"},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusBadRequest, resp.StatusCode)
				assert.Equal(s.T(), "amount_minor must be greater than 0", payload["error"])
			},
		},
		{
			name:   "wallet not found",
			userID: "user-1",
			body:   []byte(`{"amount_minor":100}`),
			setupMock: func() {
				s.service.EXPECT().WithdrawBalance(mock.Anything, "user-1", int64(100), "chain-1").Return(vo.WalletWithdrawal{}, vo.ErrWalletNotFound)
			},
			headers: map[string]string{middlewares.ChainIDHeader: "chain-1"},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusNotFound, resp.StatusCode)
				assert.Equal(s.T(), "wallet not found", payload["error"])
			},
		},
		{
			name:   "insufficient balance",
			userID: "user-1",
			body:   []byte(`{"amount_minor":100}`),
			setupMock: func() {
				s.service.EXPECT().WithdrawBalance(mock.Anything, "user-1", int64(100), "chain-1").Return(vo.WalletWithdrawal{}, vo.ErrInsufficientBalance)
			},
			headers: map[string]string{middlewares.ChainIDHeader: "chain-1"},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusConflict, resp.StatusCode)
				assert.Equal(s.T(), "insufficient balance", payload["error"])
			},
		},
		{
			name:   "internal error",
			userID: "user-1",
			body:   []byte(`{"amount_minor":100}`),
			setupMock: func() {
				s.service.EXPECT().WithdrawBalance(mock.Anything, "user-1", int64(100), "chain-1").Return(vo.WalletWithdrawal{}, serviceErr)
			},
			headers: map[string]string{middlewares.ChainIDHeader: "chain-1"},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusInternalServerError, resp.StatusCode)
				assert.Equal(s.T(), "internal server error", payload["error"])
			},
		},
		{
			name:   "success",
			userID: "user-1",
			body:   []byte(`{"amount_minor":100}`),
			setupMock: func() {
				s.service.EXPECT().WithdrawBalance(mock.Anything, "user-1", int64(100), "chain-1").Return(vo.WalletWithdrawal{
					UserID:       "user-1",
					AmountMinor:  100,
					BalanceMinor: 900,
					Currency:     "IDR",
					ChainID:      "chain-1",
					UpdatedAt:    now,
				}, nil)
			},
			headers: map[string]string{middlewares.ChainIDHeader: "chain-1"},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusOK, resp.StatusCode)
				assert.Equal(s.T(), "user-1", payload["user_id"])
				assert.Equal(s.T(), float64(100), payload["amount_minor"])
				assert.Equal(s.T(), "chain-1", payload["chain_id"])
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.app.Post("/withdrawals", func(c fiber.Ctx) error {
				if tc.userID != "" {
					c.Locals("user_id", tc.userID)
				}
				return s.handler.Handle(c)
			})
			if tc.setupMock != nil {
				tc.setupMock()
			}

			resp, payload, _ := performJSONRequest(s.app, http.MethodPost, "/withdrawals", tc.body, tc.headers)
			if resp == nil {
				s.T().Fatal("failed to execute request")
			}
			tc.assertion(resp, payload)
		})
	}
}

func TestInquiryWithdrawBalanceHandlerSuite(t *testing.T) {
	suite.Run(t, new(InquiryWithdrawBalanceHandlerSuite))
}
