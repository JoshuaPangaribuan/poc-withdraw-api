package app

import (
	"github.com/joshuarp/withdraw-api/internal/handlers"
	"github.com/joshuarp/withdraw-api/internal/repository"
	"github.com/joshuarp/withdraw-api/internal/services"
	"go.uber.org/fx"
)

func InquiryModule() fx.Option {
	return fx.Module("inquiry",
		fx.Provide(
			fx.Annotate(
				repository.NewInquiryCheckBalanceRepository,
				fx.ParamTags(`name:"db_wallet"`),
				fx.As(new(services.BalanceInquiryRepository)),
			),
			fx.Annotate(
				services.NewInquiryCheckBalanceService,
				fx.As(new(handlers.BalanceInquiryService)),
			),
			handlers.NewInquiryCheckBalanceHandler,
		),
		fx.Invoke(registerInquiryRoutes),
	)
}
