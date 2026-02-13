package idempotency

import (
	"context"
	"time"
)

type DecisionType string

const (
	DecisionAcquired   DecisionType = "acquired"
	DecisionReplay     DecisionType = "replay"
	DecisionInProgress DecisionType = "in_progress"
	DecisionConflict   DecisionType = "conflict"
)

type Request struct {
	Scope       string
	Key         string
	RequestHash string
	LockTTL     time.Duration
}

type Decision struct {
	Type        DecisionType
	StatusCode  int
	Body        []byte
	ContentType string
}

type StoredResponse struct {
	StatusCode  int
	Body        []byte
	ContentType string
}

type Store interface {
	Acquire(ctx context.Context, request Request) (Decision, error)
	Complete(ctx context.Context, request Request, response StoredResponse) error
}
