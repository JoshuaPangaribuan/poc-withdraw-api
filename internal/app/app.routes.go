package app

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/joshuarp/withdraw-api/internal/handlers"
	"github.com/joshuarp/withdraw-api/internal/middlewares"
	sharedidempotency "github.com/joshuarp/withdraw-api/internal/shared/idempotency"
	sharedjwt "github.com/joshuarp/withdraw-api/internal/shared/jwt"
	sharedratelimit "github.com/joshuarp/withdraw-api/internal/shared/ratelimit"
	"go.uber.org/fx"
)

type routerGroupsOut struct {
	fx.Out
	Public    fiber.Router `name:"api_public"`
	Protected fiber.Router `name:"api_protected"`
}

func provideRouterGroups(
	app *fiber.App,
	logger *slog.Logger,
	tokenManager sharedjwt.TokenManager,
) routerGroupsOut {
	app.Use(middlewares.NewHTTPRecoveryMiddleware())
	app.Use(middlewares.NewHTTPRequestIDMiddleware())
	app.Use(middlewares.NewHTTPCORSMiddleware())
	app.Use(middlewares.NewHTTPRequestResponseLogMiddleware(logger))

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api/v1")
	protected := api.Group("", middlewares.NewHTTPJWTMiddleware(tokenManager))

	return routerGroupsOut{
		Public:    api,
		Protected: protected,
	}
}

type authRoutesIn struct {
	fx.In
	Public  fiber.Router `name:"api_public"`
	Handler *handlers.AuthLoginHandler
}

func registerAuthRoutes(in authRoutesIn) {
	in.Handler.Register(in.Public)
}

type inquiryRoutesIn struct {
	fx.In
	Protected fiber.Router `name:"api_protected"`
	Handler   *handlers.InquiryCheckBalanceHandler
}

func registerInquiryRoutes(in inquiryRoutesIn) {
	in.Handler.Register(in.Protected)
}

type withdrawRoutesIn struct {
	fx.In
	Protected        fiber.Router            `name:"api_protected"`
	IdempotencyStore sharedidempotency.Store `name:"withdraw_idempotency_store"`
	RateLimiter      sharedratelimit.Limiter `name:"withdraw_rate_limiter"`
	Logger           *slog.Logger
	Handler          *handlers.InquiryWithdrawBalanceHandler
}

func registerWithdrawRoutes(in withdrawRoutesIn) {
	rateLimitMiddleware := middlewares.NewHTTPRateLimitMiddleware(middlewares.RateLimitConfig{
		Limiter:      in.RateLimiter,
		Logger:       in.Logger,
		KeyExtractor: middlewares.PerUserKeyExtractor("withdraw"),
	})

	withdrawRouter := in.Protected.Group("", rateLimitMiddleware, middlewares.NewHTTPWithdrawIdempotencyMiddleware(in.IdempotencyStore))
	in.Handler.Register(withdrawRouter)
}
