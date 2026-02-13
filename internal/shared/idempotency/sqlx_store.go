package idempotency

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const defaultLockTTL = 30 * time.Second

type SQLXStore struct {
	db *sqlx.DB
}

func NewSQLXStore(db *sqlx.DB) *SQLXStore {
	return &SQLXStore{db: db}
}

func (s *SQLXStore) Acquire(ctx context.Context, request Request) (Decision, error) {
	if s == nil || s.db == nil {
		return Decision{}, errors.New("idempotency: store is not initialized")
	}

	lockTTL := request.LockTTL
	if lockTTL <= 0 {
		lockTTL = defaultLockTTL
	}

	scope := strings.TrimSpace(request.Scope)
	if scope == "" {
		return Decision{}, errors.New("idempotency: scope is required")
	}

	key := strings.TrimSpace(request.Key)
	if key == "" {
		return Decision{}, errors.New("idempotency: key is required")
	}

	hash := strings.TrimSpace(request.RequestHash)
	if hash == "" {
		return Decision{}, errors.New("idempotency: request hash is required")
	}

	now := time.Now().UTC()
	lockUntil := now.Add(lockTTL)

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return Decision{}, fmt.Errorf("idempotency: failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	type row struct {
		RequestHash    string         `db:"request_hash"`
		Status         string         `db:"status"`
		ResponseStatus sql.NullInt64  `db:"response_status"`
		ResponseBody   []byte         `db:"response_body"`
		ResponseType   sql.NullString `db:"response_content_type"`
		LockedUntil    time.Time      `db:"locked_until"`
	}

	const selectQuery = `
SELECT request_hash, status, response_status, response_body, response_content_type, locked_until
FROM withdraw_idempotency
WHERE scope = $1 AND idempotency_key = $2
FOR UPDATE`

	var existing row
	err = tx.GetContext(ctx, &existing, selectQuery, scope, key)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return Decision{}, fmt.Errorf("idempotency: failed to query key: %w", err)
		}

		const insertQuery = `
INSERT INTO withdraw_idempotency (
	scope, idempotency_key, request_hash, status, locked_until, created_at, updated_at
) VALUES ($1, $2, $3, 'in_progress', $4, now(), now())`

		if _, insertErr := tx.ExecContext(ctx, insertQuery, scope, key, hash, lockUntil); insertErr != nil {
			return Decision{}, fmt.Errorf("idempotency: failed to insert key: %w", insertErr)
		}

		if commitErr := tx.Commit(); commitErr != nil {
			return Decision{}, fmt.Errorf("idempotency: failed to commit acquire insert: %w", commitErr)
		}

		return Decision{Type: DecisionAcquired}, nil
	}

	if existing.RequestHash != hash {
		if commitErr := tx.Commit(); commitErr != nil {
			return Decision{}, fmt.Errorf("idempotency: failed to commit conflict read: %w", commitErr)
		}

		return Decision{Type: DecisionConflict}, nil
	}

	if existing.Status == "completed" {
		if commitErr := tx.Commit(); commitErr != nil {
			return Decision{}, fmt.Errorf("idempotency: failed to commit replay read: %w", commitErr)
		}

		decision := Decision{
			Type: DecisionReplay,
		}
		if existing.ResponseStatus.Valid {
			decision.StatusCode = int(existing.ResponseStatus.Int64)
		}
		decision.Body = append([]byte(nil), existing.ResponseBody...)
		if existing.ResponseType.Valid {
			decision.ContentType = existing.ResponseType.String
		}

		return decision, nil
	}

	if existing.Status == "in_progress" && existing.LockedUntil.After(now) {
		if commitErr := tx.Commit(); commitErr != nil {
			return Decision{}, fmt.Errorf("idempotency: failed to commit in-progress read: %w", commitErr)
		}

		return Decision{Type: DecisionInProgress}, nil
	}

	const reacquireQuery = `
UPDATE withdraw_idempotency
SET status = 'in_progress', locked_until = $3, updated_at = now()
WHERE scope = $1 AND idempotency_key = $2`

	if _, updateErr := tx.ExecContext(ctx, reacquireQuery, scope, key, lockUntil); updateErr != nil {
		return Decision{}, fmt.Errorf("idempotency: failed to reacquire key: %w", updateErr)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return Decision{}, fmt.Errorf("idempotency: failed to commit reacquire: %w", commitErr)
	}

	return Decision{Type: DecisionAcquired}, nil
}

func (s *SQLXStore) Complete(ctx context.Context, request Request, response StoredResponse) error {
	if s == nil || s.db == nil {
		return errors.New("idempotency: store is not initialized")
	}

	scope := strings.TrimSpace(request.Scope)
	if scope == "" {
		return errors.New("idempotency: scope is required")
	}

	key := strings.TrimSpace(request.Key)
	if key == "" {
		return errors.New("idempotency: key is required")
	}

	hash := strings.TrimSpace(request.RequestHash)
	if hash == "" {
		return errors.New("idempotency: request hash is required")
	}

	contentType := strings.TrimSpace(response.ContentType)

	const updateQuery = `
UPDATE withdraw_idempotency
SET
	status = 'completed',
	response_status = $4,
	response_body = $5,
	response_content_type = $6,
	locked_until = now(),
	completed_at = now(),
	updated_at = now()
WHERE scope = $1 AND idempotency_key = $2 AND request_hash = $3`

	result, err := s.db.ExecContext(ctx, updateQuery, scope, key, hash, response.StatusCode, response.Body, contentType)
	if err != nil {
		return fmt.Errorf("idempotency: failed to persist response: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("idempotency: failed to read affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("idempotency: key not found for completion")
	}

	return nil
}
