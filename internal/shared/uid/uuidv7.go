package uid

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

var _ UIDGenerator = (*uuidv7Generator)(nil)

type uuidv7Generator struct{}

// NewUUIDv7 creates a UUID v7-based UIDGenerator.
func NewUUIDv7() (UIDGenerator, error) {
	return &uuidv7Generator{}, nil
}

func (g *uuidv7Generator) Generate(ctx context.Context) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("uid: failed to generate uuid v7: %w", err)
	}
	return id.String(), nil
}
