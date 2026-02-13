package middlewares

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/joshuarp/withdraw-api/internal/shared/ratelimit"
)

type RateLimitConfig struct {
	Limiter      ratelimit.Limiter
	Skipper      func(c fiber.Ctx) bool
	KeyExtractor func(c fiber.Ctx) string
	Logger       *slog.Logger
}

func NewHTTPRateLimitMiddleware(cfg RateLimitConfig) fiber.Handler {
	if cfg.Limiter == nil {
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	if cfg.Skipper == nil {
		cfg.Skipper = func(c fiber.Ctx) bool { return false }
	}

	if cfg.KeyExtractor == nil {
		cfg.KeyExtractor = defaultKeyExtractor
	}

	return func(c fiber.Ctx) error {
		if cfg.Skipper(c) {
			return c.Next()
		}

		ctx := c.Context()

		key := cfg.KeyExtractor(c)
		ctx = ratelimit.WithIP(ctx, c.IP())

		if userID := c.Locals("user_id"); userID != nil {
			if uid, ok := userID.(string); ok {
				ctx = ratelimit.WithUserID(ctx, uid)
			}
		}

		result, err := cfg.Limiter.AllowKey(ctx, key)
		if err != nil {
			if cfg.Logger != nil {
				cfg.Logger.Error("rate limit check failed", "error", err, "key", key)
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		c.Set("X-RateLimit-Limit", strconv.FormatInt(result.Limit, 10))
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed {
			retryAfter := int(result.RetryAfter.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Set("Retry-After", strconv.Itoa(retryAfter))

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
		}

		return c.Next()
	}
}

func defaultKeyExtractor(c fiber.Ctx) string {
	if userID := c.Locals("user_id"); userID != nil {
		if uid, ok := userID.(string); ok && uid != "" {
			return "user:" + uid
		}
	}
	return "ip:" + c.IP()
}

func SkipHealthCheck(c fiber.Ctx) bool {
	return c.Path() == "/healthz"
}

func SkipAuthRoutes(c fiber.Ctx) bool {
	path := c.Path()
	return path == "/healthz" || (c.Method() == fiber.MethodPost && path == "/api/v1/auth/login")
}

func PerUserKeyExtractor(prefix string) func(c fiber.Ctx) string {
	return func(c fiber.Ctx) string {
		if userID := c.Locals("user_id"); userID != nil {
			if uid, ok := userID.(string); ok && uid != "" {
				return prefix + ":user:" + uid
			}
		}
		return prefix + ":ip:" + c.IP()
	}
}

func PerIPKeyExtractor(prefix string) func(c fiber.Ctx) string {
	return func(c fiber.Ctx) string {
		return prefix + ":ip:" + c.IP()
	}
}

func PerEndpointKeyExtractor(prefix string) func(c fiber.Ctx) string {
	return func(c fiber.Ctx) string {
		if userID := c.Locals("user_id"); userID != nil {
			if uid, ok := userID.(string); ok && uid != "" {
				return prefix + ":" + c.Method() + ":" + c.Path() + ":user:" + uid
			}
		}
		return prefix + ":" + c.Method() + ":" + c.Path() + ":ip:" + c.IP()
	}
}
