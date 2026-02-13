package hash

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

var _ Hasher = (*bcryptHasher)(nil)

type bcryptHasher struct {
	cost int
}

// NewBcrypt creates a bcrypt-based Hasher.
// If cost is zero, bcrypt.DefaultCost (10) is used.
// Cost must be between bcrypt.MinCost (4) and bcrypt.MaxCost (31).
func NewBcrypt(cost int) (Hasher, error) {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("hash: bcrypt cost %d out of range [%d, %d]", cost, bcrypt.MinCost, bcrypt.MaxCost)
	}
	return &bcryptHasher{cost: cost}, nil
}

func (h *bcryptHasher) Hash(_ context.Context, plaintext string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintext), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash: bcrypt hashing failed: %w", err)
	}
	return string(hashed), nil
}

func (h *bcryptHasher) Compare(_ context.Context, hashed, plaintext string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plaintext)); err != nil {
		return fmt.Errorf("hash: bcrypt comparison failed: %w", err)
	}
	return nil
}
