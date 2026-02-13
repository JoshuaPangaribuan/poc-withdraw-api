package middlewares

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

const ChainIDHeader = "X-Request-ID"

func NewHTTPRequestIDMiddleware() fiber.Handler {
	return requestid.New(requestid.Config{
		Header: ChainIDHeader,
	})
}

func ChainIDFromContext(c fiber.Ctx) string {
	chainID := requestid.FromContext(c)
	if chainID != "" {
		return chainID
	}

	return c.Get(ChainIDHeader)
}
