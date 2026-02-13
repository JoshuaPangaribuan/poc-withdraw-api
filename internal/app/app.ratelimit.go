package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/joshuarp/withdraw-api/internal/shared/config"
	sharedratelimit "github.com/joshuarp/withdraw-api/internal/shared/ratelimit"
)

func provideRedisClient(cfg config.ConfigProvider) *redis.Client {
	host := strings.TrimSpace(cfg.GetString("redis.host"))
	if host == "" {
		host = "localhost"
	}

	port := cfg.GetInt("redis.port")
	if port == 0 {
		port = 6379
	}

	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: cfg.GetString("redis.password"),
		DB:       cfg.GetInt("redis.db"),
	})
}

func provideWithdrawRateLimiter(cfg config.ConfigProvider, redisClient *redis.Client, logger *slog.Logger) (sharedratelimit.Limiter, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("app: redis client is required for withdraw rate limiter")
	}

	limit := cfg.GetInt("rate_limit.withdraw.limit")
	if limit <= 0 {
		limit = 20
	}

	window := cfg.GetDuration("rate_limit.withdraw.window")
	if window <= 0 {
		window = time.Minute
	}

	burst := cfg.GetInt("rate_limit.withdraw.burst")
	if burst <= 0 {
		burst = limit
	}

	algorithm := parseRateLimitAlgorithm(cfg.GetString("rate_limit.withdraw.algorithm"))
	store := sharedratelimit.NewRedisStore(redisClient, sharedratelimit.WithRedisPrefix("withdraw-api:withdraw"))

	return sharedratelimit.New(store, sharedratelimit.Config{
		Algorithm: algorithm,
		Limit:     int64(limit),
		Window:    window,
		Burst:     int64(burst),
		OnLimited: func(_ context.Context, key string, result sharedratelimit.Result) {
			if logger != nil {
				logger.Warn("rate limit exceeded", "scope", "withdraw", "key", key, "limit", result.Limit)
			}
		},
	})
}

func parseRateLimitAlgorithm(value string) sharedratelimit.Algorithm {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "sliding_window":
		return sharedratelimit.AlgorithmSlidingWindow
	case "fixed_window":
		return sharedratelimit.AlgorithmFixedWindow
	default:
		return sharedratelimit.AlgorithmTokenBucket
	}
}
