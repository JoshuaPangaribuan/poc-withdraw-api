-- name: GetWalletBalanceByUserID :one
SELECT
    w.user_id::text AS user_id,
    w.balance_minor,
    w.currency,
    w.updated_at
FROM wallets AS w
WHERE w.user_id = sqlc.arg(user_id)::uuid
LIMIT 1;
