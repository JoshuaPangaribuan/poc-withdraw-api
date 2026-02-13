package middlewares

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	sharedjwt "github.com/joshuarp/withdraw-api/internal/shared/jwt"
)

func NewHTTPJWTMiddleware(tokenManager sharedjwt.TokenManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		if c.Method() == fiber.MethodPost && strings.Contains(path, "/auth/login") {
			return c.Next()
		}

		authorizationHeader := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
		parts := strings.SplitN(authorizationHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing or invalid authorization header",
			})
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing bearer token",
			})
		}

		claims, err := tokenManager.Verify(context.Background(), tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid token",
			})
		}

		c.Locals("user_id", claims.Subject)
		c.Locals("jwt_claims", claims)
		return c.Next()
	}
}
