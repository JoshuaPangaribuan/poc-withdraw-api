package app

import (
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joshuarp/withdraw-api/internal/shared/config"
	"go.uber.org/fx"
)

type dbProviderIn struct {
	fx.In

	Config config.ConfigProvider
	Bin    string `name:"bin"`
}

func provideAuthPostgresSQLX(in dbProviderIn) (*sqlx.DB, error) {
	return providePostgresSQLXForModule(in.Config, in.Bin, "auth")
}

func provideWalletPostgresSQLX(in dbProviderIn) (*sqlx.DB, error) {
	return providePostgresSQLXForModule(in.Config, in.Bin, "wallet")
}

func providePostgresSQLXForModule(cfg config.ConfigProvider, bin, module string) (*sqlx.DB, error) {
	useModuleConfig := !isSingleBinaryBin(bin)

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		moduleDBString(cfg, module, "host", useModuleConfig),
		moduleDBInt(cfg, module, "port", useModuleConfig),
		moduleDBString(cfg, module, "user", useModuleConfig),
		moduleDBString(cfg, module, "password", useModuleConfig),
		moduleDBString(cfg, module, "name", useModuleConfig),
		moduleDBString(cfg, module, "ssl_mode", useModuleConfig),
	)

	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("db(%s): failed to open postgres connection: %w", module, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db(%s): failed to ping postgres: %w", module, err)
	}

	return db, nil
}

func moduleDBString(cfg config.ConfigProvider, module, key string, useModuleConfig bool) string {
	if useModuleConfig {
		moduleKey := fmt.Sprintf("database.%s.%s", module, key)
		if cfg.IsSet(moduleKey) {
			return cfg.GetString(moduleKey)
		}

		moduleEnvKey := moduleDBEnvKey(module, key)
		if cfg.IsSet(moduleEnvKey) {
			return cfg.GetString(moduleEnvKey)
		}
	}

	globalKey := fmt.Sprintf("database.%s", key)
	if cfg.IsSet(globalKey) {
		return cfg.GetString(globalKey)
	}

	return cfg.GetString(globalDBEnvKey(key))
}

func moduleDBInt(cfg config.ConfigProvider, module, key string, useModuleConfig bool) int {
	if useModuleConfig {
		moduleKey := fmt.Sprintf("database.%s.%s", module, key)
		if cfg.IsSet(moduleKey) {
			return cfg.GetInt(moduleKey)
		}

		moduleEnvKey := moduleDBEnvKey(module, key)
		if cfg.IsSet(moduleEnvKey) {
			return cfg.GetInt(moduleEnvKey)
		}
	}

	globalKey := fmt.Sprintf("database.%s", key)
	if cfg.IsSet(globalKey) {
		return cfg.GetInt(globalKey)
	}

	return cfg.GetInt(globalDBEnvKey(key))
}

func isSingleBinaryBin(bin string) bool {
	normalized := strings.TrimSpace(strings.ToLower(bin))
	return normalized == "" || normalized == "all"
}

func moduleDBEnvKey(module, key string) string {
	normalizedKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	return fmt.Sprintf("DATABASE_%s_%s", strings.ToUpper(module), normalizedKey)
}

func globalDBEnvKey(key string) string {
	normalizedKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	return fmt.Sprintf("DATABASE_%s", normalizedKey)
}
