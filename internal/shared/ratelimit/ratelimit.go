// Package ratelimit provides rate limiting functionality with pluggable
// storage backends and algorithms. Implementations are safe for concurrent use.
package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// Algorithm defines the rate limiting algorithm to use.
type Algorithm string

const (
	// AlgorithmTokenBucket uses token bucket algorithm.
	// Good for: APIs that need burst handling with steady rate refill.
	AlgorithmTokenBucket Algorithm = "token_bucket"

	// AlgorithmSlidingWindow uses sliding window log algorithm.
	// Good for: Precise rate limiting without burst allowance.
	AlgorithmSlidingWindow Algorithm = "sliding_window"

	// AlgorithmFixedWindow uses fixed window counter algorithm.
	// Good for: Simple, memory-efficient rate limiting.
	// Note: Allows burst at window boundaries.
	AlgorithmFixedWindow Algorithm = "fixed_window"
)

// KeyExtractor extracts a rate limit key from context.
// Common use: user_id, IP address, API key, or combination.
type KeyExtractor func(ctx context.Context) (string, error)

// Result contains the rate limit decision and metadata.
type Result struct {
	// Allowed indicates if the request is permitted.
	Allowed bool

	// Limit is the maximum requests per window.
	Limit int64

	// Remaining is the number of requests left in current window.
	Remaining int64

	// ResetAt is when the rate limit window resets.
	ResetAt time.Time

	// RetryAfter is the duration to wait before retrying (if not allowed).
	RetryAfter time.Duration
}

// Config configures the rate limiter.
type Config struct {
	// Algorithm selects the rate limiting algorithm.
	Algorithm Algorithm

	// Limit is the maximum number of requests allowed per window.
	Limit int64

	// Window is the time window for rate limiting.
	Window time.Duration

	// Burst allows temporary exceedance of limit (token bucket only).
	// If 0, defaults to Limit.
	Burst int64

	// KeyExtractor extracts the rate limit key from context.
	// If nil, defaults to IP-based extraction.
	KeyExtractor KeyExtractor

	// OnLimited is called when rate limit is exceeded.
	// Can be used for custom logging or metrics.
	OnLimited func(ctx context.Context, key string, result Result)
}

// Store is the interface for rate limit storage backends.
// Implementations must be safe for concurrent use.
type Store interface {
	// Allow checks if a request is allowed and consumes a token/slot.
	// Returns Result with decision and metadata.
	Allow(ctx context.Context, key string, config Config) (Result, error)

	// Reset resets the rate limit for a specific key.
	Reset(ctx context.Context, key string) error

	// Close releases any resources used by the store.
	Close() error
}

// Limiter is the main rate limiter interface.
type Limiter interface {
	// Allow checks if a request is allowed for the given context.
	// The key is extracted using the configured KeyExtractor.
	Allow(ctx context.Context) (Result, error)

	// AllowKey checks if a request is allowed for a specific key.
	// Useful for manual key management.
	AllowKey(ctx context.Context, key string) (Result, error)

	// Reset resets the rate limit for the extracted key.
	Reset(ctx context.Context) error

	// ResetKey resets the rate limit for a specific key.
	ResetKey(ctx context.Context, key string) error

	// Close releases resources.
	Close() error
}

// limiter is the concrete implementation of Limiter.
type limiter struct {
	store  Store
	config Config
}

// New creates a new rate limiter with the provided store and configuration.
func New(store Store, config Config) (Limiter, error) {
	if store == nil {
		return nil, fmt.Errorf("ratelimit: store is required")
	}

	if config.Limit <= 0 {
		return nil, fmt.Errorf("ratelimit: limit must be positive")
	}

	if config.Window <= 0 {
		return nil, fmt.Errorf("ratelimit: window must be positive")
	}

	if config.Algorithm == "" {
		config.Algorithm = AlgorithmTokenBucket
	}

	if config.Burst <= 0 {
		config.Burst = config.Limit
	}

	if config.KeyExtractor == nil {
		config.KeyExtractor = DefaultKeyExtractor
	}

	return &limiter{
		store:  store,
		config: config,
	}, nil
}

func (l *limiter) Allow(ctx context.Context) (Result, error) {
	key, err := l.config.KeyExtractor(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("ratelimit: failed to extract key: %w", err)
	}
	return l.AllowKey(ctx, key)
}

func (l *limiter) AllowKey(ctx context.Context, key string) (Result, error) {
	result, err := l.store.Allow(ctx, key, l.config)
	if err != nil {
		return Result{}, fmt.Errorf("ratelimit: store error: %w", err)
	}

	if !result.Allowed && l.config.OnLimited != nil {
		l.config.OnLimited(ctx, key, result)
	}

	return result, nil
}

func (l *limiter) Reset(ctx context.Context) error {
	key, err := l.config.KeyExtractor(ctx)
	if err != nil {
		return fmt.Errorf("ratelimit: failed to extract key: %w", err)
	}
	return l.ResetKey(ctx, key)
}

func (l *limiter) ResetKey(ctx context.Context, key string) error {
	return l.store.Reset(ctx, key)
}

func (l *limiter) Close() error {
	return l.store.Close()
}

// DefaultKeyExtractor extracts IP address as the rate limit key.
// Override with custom KeyExtractor for user-based or API key-based limiting.
func DefaultKeyExtractor(ctx context.Context) (string, error) {
	// Try to get IP from Fiber context (set by middleware)
	if ip, ok := ctx.Value(contextKeyIP).(string); ok && ip != "" {
		return "ip:" + ip, nil
	}

	// Fallback to a default key (should not happen in production)
	return "default", nil
}

// UserKeyExtractor creates a KeyExtractor that uses user_id from context.
// Useful for authenticated endpoints where rate limiting is per-user.
func UserKeyExtractor(prefix string) KeyExtractor {
	return func(ctx context.Context) (string, error) {
		if userID, ok := ctx.Value(contextKeyUserID).(string); ok && userID != "" {
			if prefix != "" {
				return prefix + ":user:" + userID, nil
			}
			return "user:" + userID, nil
		}
		return "", fmt.Errorf("ratelimit: user_id not found in context")
	}
}

// IPKeyExtractor creates a KeyExtractor that uses IP address with a prefix.
func IPKeyExtractor(prefix string) KeyExtractor {
	return func(ctx context.Context) (string, error) {
		if ip, ok := ctx.Value(contextKeyIP).(string); ok && ip != "" {
			if prefix != "" {
				return prefix + ":ip:" + ip, nil
			}
			return "ip:" + ip, nil
		}
		return "", fmt.Errorf("ratelimit: ip not found in context")
	}
}

// CompositeKeyExtractor creates a KeyExtractor from multiple extractors.
// Keys are joined with ":" separator.
func CompositeKeyExtractor(extractors ...KeyExtractor) KeyExtractor {
	return func(ctx context.Context) (string, error) {
		parts := make([]string, 0, len(extractors))
		for _, ext := range extractors {
			part, err := ext(ctx)
			if err != nil {
				return "", err
			}
			parts = append(parts, part)
		}
		return joinKeys(parts...), nil
	}
}

func joinKeys(parts ...string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ":"
		}
		result += p
	}
	return result
}

// Context key types for type-safe context values.
type contextKey string

const (
	contextKeyIP     contextKey = "ratelimit:ip"
	contextKeyUserID contextKey = "ratelimit:user_id"
)

// WithIP adds IP address to context for rate limiting.
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, contextKeyIP, ip)
}

// WithUserID adds user ID to context for rate limiting.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// GetIP retrieves IP from context.
func GetIP(ctx context.Context) string {
	if ip, ok := ctx.Value(contextKeyIP).(string); ok {
		return ip
	}
	return ""
}

// GetUserID retrieves user ID from context.
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(contextKeyUserID).(string); ok {
		return userID
	}
	return ""
}
