package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	idempotencymocks "github.com/joshuarp/withdraw-api/internal/mock/shared/idempotency"
	jwtmocks "github.com/joshuarp/withdraw-api/internal/mock/shared/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sharedidempotency "github.com/joshuarp/withdraw-api/internal/shared/idempotency"
	sharedjwt "github.com/joshuarp/withdraw-api/internal/shared/jwt"
	sharedratelimit "github.com/joshuarp/withdraw-api/internal/shared/ratelimit"
)

func doRequest(app *fiber.App, method, path string, body []byte, headers map[string]string) (*http.Response, map[string]interface{}, []byte, error) {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if len(body) > 0 {
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := app.Test(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, nil, err
	}

	parsed := map[string]interface{}{}
	_ = json.Unmarshal(rawBody, &parsed)

	return resp, parsed, rawBody, nil
}

type HTTPJWTMiddlewareSuite struct {
	suite.Suite

	tokenManager *jwtmocks.TokenManager
	app          *fiber.App
}

func (s *HTTPJWTMiddlewareSuite) SetupTest() {
	s.tokenManager = jwtmocks.NewTokenManager(s.T())
	s.app = fiber.New()
	s.app.Use(NewHTTPJWTMiddleware(s.tokenManager))
	s.app.Get("/secure", func(c fiber.Ctx) error {
		claims, _ := c.Locals("jwt_claims").(*sharedjwt.Claims)
		return c.JSON(fiber.Map{
			"user_id": c.Locals("user_id"),
			"subject": claims.Subject,
		})
	})
	s.app.Post("/auth/login", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})
}

func (s *HTTPJWTMiddlewareSuite) TestNewHTTPJWTMiddleware_TableDriven() {
	verifyErr := errors.New("invalid")

	tests := []struct {
		name      string
		method    string
		path      string
		headers   map[string]string
		setupMock func()
		assertion func(*http.Response, map[string]interface{})
	}{
		{
			name:   "bypass auth login route",
			method: http.MethodPost,
			path:   "/auth/login",
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusOK, resp.StatusCode)
				assert.Equal(s.T(), true, payload["ok"])
			},
		},
		{
			name:    "missing authorization header",
			method:  http.MethodGet,
			path:    "/secure",
			headers: map[string]string{},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusUnauthorized, resp.StatusCode)
				assert.Equal(s.T(), "missing or invalid authorization header", payload["error"])
			},
		},
		{
			name:   "missing bearer token",
			method: http.MethodGet,
			path:   "/secure",
			headers: map[string]string{
				fiber.HeaderAuthorization: "Bearer   ",
			},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusUnauthorized, resp.StatusCode)
				assert.Equal(s.T(), "missing or invalid authorization header", payload["error"])
			},
		},
		{
			name:   "invalid token",
			method: http.MethodGet,
			path:   "/secure",
			headers: map[string]string{
				fiber.HeaderAuthorization: "Bearer token-123",
			},
			setupMock: func() {
				s.tokenManager.EXPECT().Verify(mock.Anything, "token-123").Return(nil, verifyErr)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusUnauthorized, resp.StatusCode)
				assert.Equal(s.T(), "invalid token", payload["error"])
			},
		},
		{
			name:   "valid token",
			method: http.MethodGet,
			path:   "/secure",
			headers: map[string]string{
				fiber.HeaderAuthorization: "Bearer token-123",
			},
			setupMock: func() {
				s.tokenManager.EXPECT().Verify(mock.Anything, "token-123").Return(&sharedjwt.Claims{Subject: "user-1"}, nil)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusOK, resp.StatusCode)
				assert.Equal(s.T(), "user-1", payload["user_id"])
				assert.Equal(s.T(), "user-1", payload["subject"])
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setupMock != nil {
				tc.setupMock()
			}

			resp, payload, _, err := doRequest(s.app, tc.method, tc.path, nil, tc.headers)
			require.NoError(s.T(), err)
			tc.assertion(resp, payload)
		})
	}
}

func TestHTTPJWTMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(HTTPJWTMiddlewareSuite))
}

type HTTPWithdrawIdempotencyMiddlewareSuite struct {
	suite.Suite

	store *idempotencymocks.Store
	app   *fiber.App
}

func (s *HTTPWithdrawIdempotencyMiddlewareSuite) SetupTest() {
	s.store = idempotencymocks.NewStore(s.T())
	s.app = fiber.New()
}

func (s *HTTPWithdrawIdempotencyMiddlewareSuite) TestNewHTTPWithdrawIdempotencyMiddleware_TableDriven() {
	acquireErr := errors.New("acquire failed")
	completeErr := errors.New("complete failed")
	responseBody := []byte(`{"ok":true}`)

	tests := []struct {
		name      string
		storeNil  bool
		userID    string
		headers   map[string]string
		body      []byte
		setupMock func(store *idempotencymocks.Store)
		assertion func(*http.Response, map[string]interface{}, []byte)
	}{
		{
			name:     "store not available",
			storeNil: true,
			userID:   "user-1",
			headers:  map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:     []byte(`{"amount_minor":100}`),
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusInternalServerError, resp.StatusCode)
				assert.Equal(s.T(), "idempotency store is not available", payload["error"])
			},
		},
		{
			name:    "missing authenticated user",
			userID:  "",
			headers: map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:    []byte(`{"amount_minor":100}`),
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusUnauthorized, resp.StatusCode)
				assert.Equal(s.T(), "missing authenticated user", payload["error"])
			},
		},
		{
			name:   "missing idempotency key",
			userID: "user-1",
			body:   []byte(`{"amount_minor":100}`),
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusBadRequest, resp.StatusCode)
				assert.Equal(s.T(), "missing idempotency key", payload["error"])
			},
		},
		{
			name:    "acquire failed",
			userID:  "user-1",
			headers: map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:    []byte(`{"amount_minor":100}`),
			setupMock: func(store *idempotencymocks.Store) {
				store.EXPECT().Acquire(mock.Anything, mock.Anything).Return(sharedidempotency.Decision{}, acquireErr)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusInternalServerError, resp.StatusCode)
				assert.Equal(s.T(), "failed to acquire idempotency key", payload["error"])
			},
		},
		{
			name:    "replay existing response",
			userID:  "user-1",
			headers: map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:    []byte(`{"amount_minor":100}`),
			setupMock: func(store *idempotencymocks.Store) {
				store.EXPECT().Acquire(mock.Anything, mock.Anything).Return(sharedidempotency.Decision{
					Type:        sharedidempotency.DecisionReplay,
					StatusCode:  fiber.StatusAccepted,
					Body:        []byte(`{"status":"replay"}`),
					ContentType: fiber.MIMEApplicationJSON,
				}, nil)
			},
			assertion: func(resp *http.Response, _ map[string]interface{}, raw []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusAccepted, resp.StatusCode)
				assert.JSONEq(s.T(), `{"status":"replay"}`, string(raw))
			},
		},
		{
			name:    "request still in progress",
			userID:  "user-1",
			headers: map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:    []byte(`{"amount_minor":100}`),
			setupMock: func(store *idempotencymocks.Store) {
				store.EXPECT().Acquire(mock.Anything, mock.Anything).Return(sharedidempotency.Decision{Type: sharedidempotency.DecisionInProgress}, nil)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusConflict, resp.StatusCode)
				assert.Equal(s.T(), "request is already in progress", payload["error"])
			},
		},
		{
			name:    "idempotency conflict",
			userID:  "user-1",
			headers: map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:    []byte(`{"amount_minor":100}`),
			setupMock: func(store *idempotencymocks.Store) {
				store.EXPECT().Acquire(mock.Anything, mock.Anything).Return(sharedidempotency.Decision{Type: sharedidempotency.DecisionConflict}, nil)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusConflict, resp.StatusCode)
				assert.Equal(s.T(), "idempotency key reused with different payload", payload["error"])
			},
		},
		{
			name:    "invalid decision type",
			userID:  "user-1",
			headers: map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:    []byte(`{"amount_minor":100}`),
			setupMock: func(store *idempotencymocks.Store) {
				store.EXPECT().Acquire(mock.Anything, mock.Anything).Return(sharedidempotency.Decision{Type: "unknown"}, nil)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusInternalServerError, resp.StatusCode)
				assert.Equal(s.T(), "invalid idempotency state", payload["error"])
			},
		},
		{
			name:    "complete failed",
			userID:  "user-1",
			headers: map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:    []byte(`{"amount_minor":100}`),
			setupMock: func(store *idempotencymocks.Store) {
				store.EXPECT().Acquire(mock.Anything, mock.Anything).Return(sharedidempotency.Decision{Type: sharedidempotency.DecisionAcquired}, nil)
				store.EXPECT().Complete(mock.Anything, mock.Anything, mock.Anything).Return(completeErr)
			},
			assertion: func(resp *http.Response, payload map[string]interface{}, _ []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusInternalServerError, resp.StatusCode)
				assert.Equal(s.T(), "failed to persist idempotency response", payload["error"])
			},
		},
		{
			name:    "acquired and persisted",
			userID:  "user-1",
			headers: map[string]string{IdempotencyKeyHeader: "idem-1"},
			body:    []byte(`{"amount_minor":100}`),
			setupMock: func(store *idempotencymocks.Store) {
				store.EXPECT().Acquire(mock.Anything, mock.Anything).Return(sharedidempotency.Decision{Type: sharedidempotency.DecisionAcquired}, nil)
				store.EXPECT().Complete(mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			assertion: func(resp *http.Response, _ map[string]interface{}, raw []byte) {
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), fiber.StatusCreated, resp.StatusCode)
				assert.JSONEq(s.T(), string(responseBody), string(raw))
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()

			var middleware fiber.Handler
			if tc.storeNil {
				middleware = NewHTTPWithdrawIdempotencyMiddleware(nil)
			} else {
				if tc.setupMock != nil {
					tc.setupMock(s.store)
				}
				middleware = NewHTTPWithdrawIdempotencyMiddleware(s.store)
			}

			s.app.Use(func(c fiber.Ctx) error {
				if tc.userID != "" {
					c.Locals("user_id", tc.userID)
				}
				return c.Next()
			})
			s.app.Post("/withdrawals", middleware, func(c fiber.Ctx) error {
				return c.Status(fiber.StatusCreated).Send(responseBody)
			})

			resp, payload, raw, err := doRequest(s.app, http.MethodPost, "/withdrawals", tc.body, tc.headers)
			require.NoError(s.T(), err)
			tc.assertion(resp, payload, raw)
		})
	}
}

func (s *HTTPWithdrawIdempotencyMiddlewareSuite) TestWithdrawRequestHash_TableDriven() {
	tests := []struct {
		name     string
		method   string
		path     string
		userID   string
		body     []byte
		other    []byte
		assertFn func(string, string)
	}{
		{
			name:   "same payload produces same hash",
			method: "post",
			path:   " /withdrawals ",
			userID: " user-1 ",
			body:   []byte(`{"amount_minor":100}`),
			other:  []byte(`{"amount_minor":100}`),
			assertFn: func(left, right string) {
				assert.Equal(s.T(), left, right)
			},
		},
		{
			name:   "different payload produces different hash",
			method: "POST",
			path:   "/withdrawals",
			userID: "user-1",
			body:   []byte(`{"amount_minor":100}`),
			other:  []byte(`{"amount_minor":200}`),
			assertFn: func(left, right string) {
				assert.NotEqual(s.T(), left, right)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			first := withdrawRequestHash(tc.method, tc.path, tc.userID, tc.body)
			second := withdrawRequestHash(tc.method, tc.path, tc.userID, tc.other)
			tc.assertFn(first, second)
		})
	}
}

func TestHTTPWithdrawIdempotencyMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(HTTPWithdrawIdempotencyMiddlewareSuite))
}

type stubRateLimiter struct {
	result  sharedratelimit.Result
	err     error
	lastKey string
}

func (s *stubRateLimiter) Allow(_ context.Context) (sharedratelimit.Result, error) {
	return s.result, s.err
}

func (s *stubRateLimiter) AllowKey(_ context.Context, key string) (sharedratelimit.Result, error) {
	s.lastKey = key
	return s.result, s.err
}

func (s *stubRateLimiter) Reset(_ context.Context) error {
	return nil
}

func (s *stubRateLimiter) ResetKey(_ context.Context, _ string) error {
	return nil
}

func (s *stubRateLimiter) Close() error {
	return nil
}

func TestHTTPRateLimitMiddleware_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		limiter       *stubRateLimiter
		keyExtractor  func(c fiber.Ctx) string
		expectedCode  int
		expectedError string
		assertHeaders bool
		expectedKey   string
	}{
		{
			name:          "allows request and sets headers",
			limiter:       &stubRateLimiter{result: sharedratelimit.Result{Allowed: true, Limit: 20, Remaining: 19, ResetAt: time.Unix(200, 0)}},
			keyExtractor:  func(c fiber.Ctx) string { return "withdraw:user:test-user" },
			expectedCode:  fiber.StatusOK,
			assertHeaders: true,
			expectedKey:   "withdraw:user:test-user",
		},
		{
			name:          "rejects when limit exceeded",
			limiter:       &stubRateLimiter{result: sharedratelimit.Result{Allowed: false, Limit: 20, Remaining: 0, RetryAfter: 5 * time.Second, ResetAt: time.Unix(250, 0)}},
			keyExtractor:  func(c fiber.Ctx) string { return "withdraw:user:test-user" },
			expectedCode:  fiber.StatusTooManyRequests,
			expectedError: "rate limit exceeded",
			expectedKey:   "withdraw:user:test-user",
		},
		{
			name:          "returns internal error when limiter fails",
			limiter:       &stubRateLimiter{err: errors.New("boom")},
			keyExtractor:  func(c fiber.Ctx) string { return "withdraw:user:test-user" },
			expectedCode:  fiber.StatusInternalServerError,
			expectedError: "internal server error",
			expectedKey:   "withdraw:user:test-user",
		},
		{
			name:          "passes through when limiter is nil",
			limiter:       nil,
			keyExtractor:  func(c fiber.Ctx) string { return "withdraw:user:test-user" },
			expectedCode:  fiber.StatusOK,
			expectedError: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Locals("user_id", "test-user")
				return c.Next()
			})

			var limiter sharedratelimit.Limiter
			if tc.limiter != nil {
				limiter = tc.limiter
			}

			app.Use(NewHTTPRateLimitMiddleware(RateLimitConfig{
				Limiter:      limiter,
				KeyExtractor: tc.keyExtractor,
			}))

			app.Get("/limited", func(c fiber.Ctx) error {
				return c.JSON(fiber.Map{"ok": true})
			})

			resp, payload, _, err := doRequest(app, http.MethodGet, "/limited", nil, nil)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tc.expectedCode, resp.StatusCode)

			if tc.expectedError != "" {
				assert.Equal(t, tc.expectedError, payload["error"])
			}

			if tc.assertHeaders {
				assert.Equal(t, "20", resp.Header.Get("X-RateLimit-Limit"))
				assert.Equal(t, "19", resp.Header.Get("X-RateLimit-Remaining"))
			}

			if tc.limiter != nil {
				assert.Equal(t, tc.expectedKey, tc.limiter.lastKey)
			}
		})
	}
}
