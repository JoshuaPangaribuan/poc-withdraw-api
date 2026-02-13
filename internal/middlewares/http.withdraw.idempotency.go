package middlewares

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	sharedidempotency "github.com/joshuarp/withdraw-api/internal/shared/idempotency"
)

const IdempotencyKeyHeader = "X-Idempotency-Key"

func NewHTTPWithdrawIdempotencyMiddleware(store sharedidempotency.Store) fiber.Handler {
	return func(c fiber.Ctx) error {
		if store == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "idempotency store is not available"})
		}

		userIDValue := c.Locals("user_id")
		userID, ok := userIDValue.(string)
		if !ok || strings.TrimSpace(userID) == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated user"})
		}

		idempotencyKey := strings.TrimSpace(c.Get(IdempotencyKeyHeader))
		if idempotencyKey == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing idempotency key"})
		}

		requestBody := append([]byte(nil), c.BodyRaw()...)
		hash := withdrawRequestHash(c.Method(), c.Path(), userID, requestBody)
		request := sharedidempotency.Request{
			Scope:       fmt.Sprintf("withdraw:%s", userID),
			Key:         idempotencyKey,
			RequestHash: hash,
		}

		decision, err := store.Acquire(c.Context(), request)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to acquire idempotency key"})
		}

		switch decision.Type {
		case sharedidempotency.DecisionReplay:
			if decision.ContentType != "" {
				c.Set(fiber.HeaderContentType, decision.ContentType)
			}
			if decision.StatusCode <= 0 {
				decision.StatusCode = fiber.StatusOK
			}

			return c.Status(decision.StatusCode).Send(decision.Body)
		case sharedidempotency.DecisionInProgress:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "request is already in progress"})
		case sharedidempotency.DecisionConflict:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "idempotency key reused with different payload"})
		case sharedidempotency.DecisionAcquired:
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "invalid idempotency state"})
		}

		handlerErr := c.Next()
		response := sharedidempotency.StoredResponse{
			StatusCode:  c.Response().StatusCode(),
			Body:        append([]byte(nil), c.Response().Body()...),
			ContentType: string(c.Response().Header.ContentType()),
		}

		if err := store.Complete(c.Context(), request, response); err != nil {
			if handlerErr != nil {
				return handlerErr
			}

			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to persist idempotency response"})
		}

		return handlerErr
	}
}

func withdrawRequestHash(method, path, userID string, body []byte) string {
	hasher := sha256.New()
	hasher.Write([]byte(strings.ToUpper(strings.TrimSpace(method))))
	hasher.Write([]byte("\n"))
	hasher.Write([]byte(strings.TrimSpace(path)))
	hasher.Write([]byte("\n"))
	hasher.Write([]byte(strings.TrimSpace(userID)))
	hasher.Write([]byte("\n"))
	hasher.Write(body)

	return hex.EncodeToString(hasher.Sum(nil))
}
