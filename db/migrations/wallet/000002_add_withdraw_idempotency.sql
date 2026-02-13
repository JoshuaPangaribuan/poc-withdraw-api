-- +goose Up
CREATE TABLE withdraw_idempotency (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope varchar(128) NOT NULL,
    idempotency_key varchar(128) NOT NULL,
    request_hash char(64) NOT NULL,
    status varchar(32) NOT NULL,
    response_status integer,
    response_body bytea,
    response_content_type varchar(255),
    locked_until timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE UNIQUE INDEX uq_withdraw_idempotency_scope_key
ON withdraw_idempotency (scope, idempotency_key);

CREATE INDEX idx_withdraw_idempotency_status_lock
ON withdraw_idempotency (status, locked_until);

-- +goose Down
DROP INDEX IF EXISTS idx_withdraw_idempotency_status_lock;
DROP INDEX IF EXISTS uq_withdraw_idempotency_scope_key;
DROP TABLE IF EXISTS withdraw_idempotency;
