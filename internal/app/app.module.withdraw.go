package app

import (
	"github.com/joshuarp/withdraw-api/internal/handlers"
	"github.com/joshuarp/withdraw-api/internal/repository"
	"github.com/joshuarp/withdraw-api/internal/services"
	sharedidempotency "github.com/joshuarp/withdraw-api/internal/shared/idempotency"
	"go.uber.org/fx"
)

func WithdrawModule() fx.Option {
	return fx.Module("withdraw",
		fx.Provide(
			fx.Annotate(
				provideWithdrawRateLimiter,
				fx.ResultTags(`name:"withdraw_rate_limiter"`),
			),
			fx.Annotate(
				sharedidempotency.NewSQLXStore,
				fx.ParamTags(`name:"db_wallet"`),
				fx.ResultTags(`name:"withdraw_idempotency_store"`),
				fx.As(new(sharedidempotency.Store)),
			),
			fx.Annotate(
				repository.NewWithdrawBalanceRepository,
				fx.ParamTags(`name:"db_wallet"`),
				fx.As(new(services.BalanceWithdrawRepository)),
			),
			fx.Annotate(
				services.NewInquiryWithdrawBalanceService,
				fx.As(new(handlers.BalanceWithdrawService)),
			),
			handlers.NewInquiryWithdrawBalanceHandler,
		),
		fx.Invoke(registerWithdrawRoutes),
	)
}
