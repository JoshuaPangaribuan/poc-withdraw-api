package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/joshuarp/withdraw-api/internal/shared/config"
	sharedhash "github.com/joshuarp/withdraw-api/internal/shared/hash"
	sharedjwt "github.com/joshuarp/withdraw-api/internal/shared/jwt"
	sharedlog "github.com/joshuarp/withdraw-api/internal/shared/log"
	"go.uber.org/fx"
)

type configBinIn struct {
	fx.In
	Bin string `name:"bin"`
}

func New(bin string, modules ...fx.Option) *fx.App {
	normalizedBin := strings.TrimSpace(strings.ToLower(bin))
	opts := []fx.Option{
		fx.Supply(
			fx.Annotate(
				normalizedBin,
				fx.ResultTags(`name:"bin"`),
			),
		),
		CoreModule(),
	}
	opts = append(opts, modules...)
	opts = append(opts, fx.Invoke(registerLifecycle))
	return fx.New(opts...)
}

func CoreModule() fx.Option {
	return fx.Module("core",
		fx.Provide(
			provideConfig,
			sharedlog.NewJSONLogger,
			provideRedisClient,
			fx.Annotate(
				provideAuthPostgresSQLX,
				fx.ResultTags(`name:"db_auth"`),
			),
			fx.Annotate(
				provideWalletPostgresSQLX,
				fx.ResultTags(`name:"db_wallet"`),
			),
			provideFiberApp,
			providePasswordHasher,
			provideJWTTokenManager,
			provideRouterGroups,
		),
	)
}

func provideConfig(in configBinIn) (config.ConfigProvider, error) {
	bin := strings.TrimSpace(strings.ToLower(in.Bin))
	if bin == "inqury" {
		bin = "inquiry"
	}

	loadOrder := make([]config.Options, 0, 4)
	if bin == "inquiry" || bin == "withdraw" {
		loadOrder = append(loadOrder,
			config.Options{
				YAMLPath: fmt.Sprintf("config.%s.yaml", bin),
				EnvPath:  fmt.Sprintf(".env.%s", bin),
			},
			config.Options{
				YAMLPath: fmt.Sprintf("config.%s.yaml.example", bin),
				EnvPath:  fmt.Sprintf(".env.%s.example", bin),
			},
		)
	}

	loadOrder = append(loadOrder,
		config.Options{
			YAMLPath: "config.yaml",
			EnvPath:  ".env",
		},
		config.Options{
			YAMLPath: "config.yaml.example",
			EnvPath:  ".env.example",
		},
	)

	var lastErr error
	for _, opts := range loadOrder {
		provider, err := config.Init(opts)
		if err == nil {
			return provider, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

func provideFiberApp(cfg config.ConfigProvider) *fiber.App {
	readTimeout := cfg.GetDuration("server.read_timeout")
	if readTimeout <= 0 {
		readTimeout = 30 * time.Second
	}

	writeTimeout := cfg.GetDuration("server.write_timeout")
	if writeTimeout <= 0 {
		writeTimeout = 30 * time.Second
	}

	return fiber.New(fiber.Config{
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})
}

func providePasswordHasher() (sharedhash.Hasher, error) {
	return sharedhash.New(sharedhash.Options{Strategy: sharedhash.StrategyBcrypt})
}

func provideJWTTokenManager(cfg config.ConfigProvider) (sharedjwt.TokenManager, error) {
	secret := cfg.GetString("security.jwt.secret")
	if secret == "" {
		secret = cfg.GetString("jwt.secret")
	}
	if secret == "" {
		secret = "change-me-please-use-strong-secret-in-production"
	}

	if len(secret) < 32 {
		secret = secret + strings.Repeat("x", 32-len(secret))
	}

	ttl := cfg.GetDuration("security.jwt.ttl")
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	tokenManager, err := sharedjwt.New(sharedjwt.Options{
		Strategy:  sharedjwt.StrategyHMAC,
		Secret:    []byte(secret),
		Algorithm: "HS256",
		TTL:       ttl,
		Issuer:    cfg.GetString("security.jwt.issuer"),
	})
	if err != nil {
		return nil, fmt.Errorf("app: failed to init JWT manager: %w", err)
	}

	return tokenManager, nil
}
