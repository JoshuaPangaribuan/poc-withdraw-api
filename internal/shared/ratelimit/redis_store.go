package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a distributed rate limit store using Redis.
// Safe for multi-instance deployments.
type RedisStore struct {
	client *redis.Client
	prefix string
}

// RedisStoreOption configures the Redis store.
type RedisStoreOption func(*RedisStore)

// WithRedisPrefix sets a prefix for all Redis keys.
func WithRedisPrefix(prefix string) RedisStoreOption {
	return func(s *RedisStore) {
		s.prefix = prefix
	}
}

// NewRedisStore creates a new Redis-based rate limit store.
func NewRedisStore(client *redis.Client, opts ...RedisStoreOption) *RedisStore {
	s := &RedisStore{
		client: client,
		prefix: "ratelimit",
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *RedisStore) Allow(ctx context.Context, key string, config Config) (Result, error) {
	if s == nil || s.client == nil {
		return Result{}, errors.New("ratelimit: redis store is not initialized")
	}

	fullKey := s.prefix + ":" + key

	switch config.Algorithm {
	case AlgorithmTokenBucket:
		return s.tokenBucket(ctx, fullKey, config)
	case AlgorithmSlidingWindow:
		return s.slidingWindow(ctx, fullKey, config)
	case AlgorithmFixedWindow:
		return s.fixedWindow(ctx, fullKey, config)
	default:
		return s.tokenBucket(ctx, fullKey, config)
	}
}

func (s *RedisStore) tokenBucket(ctx context.Context, key string, config Config) (Result, error) {
	const script = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local now = tonumber(ARGV[4])
local requested = tonumber(ARGV[5])

local data = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(data[1]) or burst
local lastRefill = tonumber(data[2]) or now

local elapsed = now - lastRefill
local refillRate = limit / window
tokens = math.min(burst, tokens + (elapsed * refillRate))

local allowed = 0
local remaining = tokens - requested
local retryAfter = 0

if tokens >= requested then
	tokens = tokens - requested
	allowed = 1
	remaining = tokens
else
	retryAfter = (requested - tokens) / refillRate
	remaining = 0
end

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
redis.call('PEXPIRE', key, window * 2)

return {allowed, math.floor(remaining), math.floor(retryAfter)}
`

	now := float64(time.Now().UnixMilli())
	windowMs := float64(config.Window.Milliseconds())

	result, err := s.client.Eval(ctx, script, []string{key},
		config.Limit,
		config.Burst,
		windowMs,
		now,
		1,
	).Slice()
	if err != nil {
		return Result{}, fmt.Errorf("ratelimit: redis eval failed: %w", err)
	}

	allowed := toInt64(result[0]) == 1
	remaining := toInt64(result[1])
	retryAfterMs := toInt64(result[2])

	return Result{
		Allowed:    allowed,
		Limit:      config.Limit,
		Remaining:  remaining,
		ResetAt:    time.Now().Add(config.Window),
		RetryAfter: time.Duration(retryAfterMs) * time.Millisecond,
	}, nil
}

func (s *RedisStore) slidingWindow(ctx context.Context, key string, config Config) (Result, error) {
	const script = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local windowStart = now - window
redis.call('ZREMRANGEBYSCORE', key, '-inf', windowStart)

local count = redis.call('ZCARD', key)
local allowed = 0
local remaining = limit - count - 1
local retryAfter = 0

if count < limit then
	redis.call('ZADD', key, now, now .. '-' .. math.random())
	allowed = 1
else
	local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
	if oldest[2] then
		retryAfter = tonumber(oldest[2]) + window - now
		if retryAfter < 0 then retryAfter = 0 end
	end
	remaining = 0
end

redis.call('PEXPIRE', key, window * 2)

return {allowed, remaining, math.floor(retryAfter)}
`

	now := float64(time.Now().UnixMilli())
	windowMs := config.Window.Milliseconds()

	result, err := s.client.Eval(ctx, script, []string{key},
		config.Limit,
		windowMs,
		now,
	).Slice()
	if err != nil {
		return Result{}, fmt.Errorf("ratelimit: redis eval failed: %w", err)
	}

	allowed := toInt64(result[0]) == 1
	remaining := toInt64(result[1])
	retryAfterMs := toInt64(result[2])

	return Result{
		Allowed:    allowed,
		Limit:      config.Limit,
		Remaining:  remaining,
		ResetAt:    time.Now().Add(config.Window),
		RetryAfter: time.Duration(retryAfterMs) * time.Millisecond,
	}, nil
}

func (s *RedisStore) fixedWindow(ctx context.Context, key string, config Config) (Result, error) {
	const script = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = tonumber(redis.call('INCR', key))
if current == 1 then
	redis.call('PEXPIRE', key, window)
end

local ttl = redis.call('PTTL', key)
local allowed = 0
local remaining = 0

if current <= limit then
	allowed = 1
	remaining = limit - current
else
	remaining = 0
end

return {allowed, remaining, ttl}
`

	windowMs := config.Window.Milliseconds()

	result, err := s.client.Eval(ctx, script, []string{key},
		config.Limit,
		windowMs,
	).Slice()
	if err != nil {
		return Result{}, fmt.Errorf("ratelimit: redis eval failed: %w", err)
	}

	allowed := toInt64(result[0]) == 1
	remaining := toInt64(result[1])
	ttlMs := toInt64(result[2])

	var retryAfter time.Duration
	if !allowed {
		retryAfter = time.Duration(ttlMs) * time.Millisecond
	}

	return Result{
		Allowed:    allowed,
		Limit:      config.Limit,
		Remaining:  remaining,
		ResetAt:    time.Now().Add(time.Duration(ttlMs) * time.Millisecond),
		RetryAfter: retryAfter,
	}, nil
}

func (s *RedisStore) Reset(ctx context.Context, key string) error {
	if s == nil || s.client == nil {
		return errors.New("ratelimit: redis store is not initialized")
	}

	fullKey := s.prefix + ":" + key
	return s.client.Del(ctx, fullKey).Err()
}

func (s *RedisStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}
