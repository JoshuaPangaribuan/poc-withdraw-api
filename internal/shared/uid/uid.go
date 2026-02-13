package uid

import (
	"context"
	"fmt"
)

// Strategy defines which UID generation algorithm to use.
type Strategy string

const (
	StrategySnowflake Strategy = "snowflake"
	StrategyUUIDv7    Strategy = "uuidv7"
)

// Options configures the UID generator.
type Options struct {
	// Strategy selects the generation algorithm.
	Strategy Strategy

	// NodeID identifies this node in a distributed system (Snowflake only).
	// Valid range: 0–1023.
	NodeID int64
}

// UIDGenerator is the interface consumers depend on for generating unique identifiers.
// Implementations must be safe for concurrent use.
type UIDGenerator interface {
	// Generate returns a new unique identifier as a string.
	Generate(ctx context.Context) (string, error)
}

// New creates a UIDGenerator based on the provided options.
// Returns an error if the strategy is unknown or configuration is invalid.
func New(opts Options) (UIDGenerator, error) {
	switch opts.Strategy {
	case StrategySnowflake:
		return NewSnowflake(opts.NodeID)
	case StrategyUUIDv7:
		return NewUUIDv7()
	default:
		return nil, fmt.Errorf("uid: unknown strategy %q", opts.Strategy)
	}
}
