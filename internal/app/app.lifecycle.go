package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/gofiber/fiber/v3"
	"github.com/jmoiron/sqlx"
	"github.com/joshuarp/withdraw-api/internal/shared/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

func registerLifecycle(
	lifecycle fx.Lifecycle,
	app *fiber.App,
	cfg config.ConfigProvider,
	logger *slog.Logger,
	dbs lifecycleDatabasesIn,
) {
	port := cfg.GetInt("server.port")
	if port == 0 {
		port = 8080
	}
	address := fmt.Sprintf(":%d", port)
	var serveErrCh chan error

	lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			listener, err := net.Listen("tcp", address)
			if err != nil {
				return fmt.Errorf("app: failed to bind server address %s: %w", address, err)
			}

			serveErrCh = make(chan error, 1)
			go func() {
				err := app.Listener(listener)
				if err != nil && !errors.Is(err, net.ErrClosed) {
					logger.Error("fiber server stopped unexpectedly", "error", err)
				}
				serveErrCh <- err
			}()

			logger.Info("fiber server started", "address", address)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			var shutdownErrors []error

			if err := app.ShutdownWithContext(ctx); err != nil {
				shutdownErrors = append(shutdownErrors, err)
			}

			if serveErrCh != nil {
				select {
				case err := <-serveErrCh:
					if err != nil && !errors.Is(err, net.ErrClosed) {
						shutdownErrors = append(shutdownErrors, err)
					}
				case <-ctx.Done():
					shutdownErrors = append(shutdownErrors, ctx.Err())
				}
			}

			closed := make(map[*sqlx.DB]struct{}, 2)
			closeDB := func(db *sqlx.DB) {
				if db == nil {
					return
				}
				if _, exists := closed[db]; exists {
					return
				}
				closed[db] = struct{}{}
				if err := db.Close(); err != nil {
					shutdownErrors = append(shutdownErrors, err)
				}
			}

			closeDB(dbs.AuthDB)
			closeDB(dbs.WalletDB)

			if dbs.Redis != nil {
				if err := dbs.Redis.Close(); err != nil {
					shutdownErrors = append(shutdownErrors, err)
				}
			}

			if len(shutdownErrors) > 0 {
				return errors.Join(shutdownErrors...)
			}

			logger.Info("fiber server shutdown completed")
			return nil
		},
	})
}

type lifecycleDatabasesIn struct {
	fx.In

	AuthDB   *sqlx.DB      `name:"db_auth" optional:"true"`
	WalletDB *sqlx.DB      `name:"db_wallet" optional:"true"`
	Redis    *redis.Client `optional:"true"`
}
