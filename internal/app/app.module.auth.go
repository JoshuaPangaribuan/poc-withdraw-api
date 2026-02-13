package app

import (
	"github.com/joshuarp/withdraw-api/internal/handlers"
	"github.com/joshuarp/withdraw-api/internal/repository"
	"github.com/joshuarp/withdraw-api/internal/services"
	"go.uber.org/fx"
)

func AuthModule() fx.Option {
	return fx.Module("auth",
		fx.Provide(
			fx.Annotate(
				repository.NewAuthLoginRepository,
				fx.ParamTags(`name:"db_auth"`),
				fx.As(new(services.AuthLoginRepository)),
			),
			fx.Annotate(
				services.NewAuthLoginService,
				fx.As(new(handlers.AuthLoginService)),
			),
			handlers.NewAuthLoginHandler,
		),
		fx.Invoke(registerAuthRoutes),
	)
}
