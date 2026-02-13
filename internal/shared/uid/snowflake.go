package uid

import (
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/snowflake"
)

var _ UIDGenerator = (*snowflakeGenerator)(nil)

type snowflakeGenerator struct {
	node *snowflake.Node
	mu   sync.Mutex
}

// NewSnowflake creates a Snowflake-based UIDGenerator.
// nodeID must be unique per node in a distributed setup (0–1023).
func NewSnowflake(nodeID int64) (UIDGenerator, error) {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("uid: failed to create snowflake node: %w", err)
	}
	return &snowflakeGenerator{node: node}, nil
}

func (g *snowflakeGenerator) Generate(ctx context.Context) (string, error) {
	g.mu.Lock()
	id := g.node.Generate()
	g.mu.Unlock()
	return id.String(), nil
}
