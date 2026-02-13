-- name: HasWalletByUserID :one
SELECT EXISTS (
    SELECT 1
    FROM wallets
    WHERE user_id = sqlc.arg(user_id)::uuid
);

-- name: WithdrawWalletBalanceByUserID :one
UPDATE wallets
SET
    balance_minor = balance_minor - sqlc.arg(amount_minor)::bigint,
    version = version + 1,
    updated_at = now()
WHERE user_id = sqlc.arg(user_id)::uuid
  AND balance_minor >= sqlc.arg(amount_minor)::bigint
RETURNING
    id AS wallet_id,
    user_id::text AS user_id,
    balance_minor,
    currency,
    updated_at;

-- name: InsertWalletLedger :exec
INSERT INTO wallet_ledger (
    wallet_id,
    entry_type,
    amount_minor,
    balance_after_minor,
    reference_id,
    chain_id,
    created_at
)
VALUES (
    sqlc.arg(wallet_id)::uuid,
    sqlc.arg(entry_type),
    sqlc.arg(amount_minor)::bigint,
    sqlc.arg(balance_after_minor)::bigint,
    sqlc.narg(reference_id),
    sqlc.narg(chain_id),
    now()
);
