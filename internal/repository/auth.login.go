package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/joshuarp/withdraw-api/internal/domain"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
)

type AuthLoginRepository struct {
	db *sqlx.DB
}

type userAuthRow struct {
	ID           string `db:"id"`
	Email        string `db:"email"`
	PasswordHash string `db:"password_hash"`
	Status       string `db:"status"`
}

func NewAuthLoginRepository(db *sqlx.DB) *AuthLoginRepository {
	return &AuthLoginRepository{db: db}
}

func (r *AuthLoginRepository) GetUserAuthByEmail(ctx context.Context, email string) (domain.UserAuth, error) {
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	if normalizedEmail == "" {
		return domain.UserAuth{}, vo.ErrInvalidCredentials
	}

	const query = `
		SELECT id::text AS id, email, password_hash, status
		FROM users
		WHERE lower(email) = $1
		LIMIT 1
	`

	var row userAuthRow
	if err := r.db.GetContext(ctx, &row, query, normalizedEmail); err != nil {
		if err == sql.ErrNoRows {
			return domain.UserAuth{}, vo.ErrInvalidCredentials
		}
		return domain.UserAuth{}, fmt.Errorf("repository: get user auth by email failed: %w", err)
	}

	if row.Status != "active" {
		return domain.UserAuth{}, vo.ErrInvalidCredentials
	}

	return domain.UserAuth{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Status:       row.Status,
	}, nil
}
