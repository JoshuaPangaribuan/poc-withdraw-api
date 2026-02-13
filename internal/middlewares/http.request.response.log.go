package middlewares

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

func NewHTTPRequestResponseLogMiddleware(logger *slog.Logger) fiber.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c fiber.Ctx) error {
		start := time.Now().UTC()
		err := c.Next()
		latency := time.Since(start)

		requestID := ChainIDFromContext(c)
		statusCode := c.Response().StatusCode()

		attrs := []any{
			"request_id", requestID,
			"method", c.Method(),
			"path", c.Path(),
			"status", statusCode,
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.IP(),
			"user_agent", c.Get(fiber.HeaderUserAgent),
		}

		if err != nil {
			logger.Error("http_request", append(attrs, "error", err.Error())...)
			return err
		}

		logger.Info("http_request", attrs...)
		return nil
	}
}
