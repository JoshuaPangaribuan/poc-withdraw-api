package hash

import (
	"context"
	"fmt"
)

// Strategy defines which hashing algorithm to use.
type Strategy string

const (
	StrategyBcrypt Strategy = "bcrypt"
)

// Options configures the hasher.
type Options struct {
	// Strategy selects the hashing algorithm.
	Strategy Strategy

	// Cost is the work factor (Bcrypt only).
	// Zero uses bcrypt.DefaultCost (10).
	Cost int
}

// Hasher is the interface consumers depend on for hashing and comparing secrets.
// Implementations must be safe for concurrent use.
type Hasher interface {
	// Hash returns a hashed representation of the plaintext.
	Hash(ctx context.Context, plaintext string) (string, error)

	// Compare checks whether the plaintext matches the hashed value.
	// Returns nil on success, or an error if they do not match.
	Compare(ctx context.Context, hashed, plaintext string) error
}

// New creates a Hasher based on the provided options.
// Returns an error if the strategy is unknown or configuration is invalid.
func New(opts Options) (Hasher, error) {
	switch opts.Strategy {
	case StrategyBcrypt:
		return NewBcrypt(opts.Cost)
	default:
		return nil, fmt.Errorf("hash: unknown strategy %q", opts.Strategy)
	}
}
