-- +goose Up
INSERT INTO wallets (id, user_id, balance_minor, currency, version)
VALUES (
    '22222222-2222-2222-2222-222222222222',
    '11111111-1111-1111-1111-111111111111',
    1000000,
    'IDR',
    0
)
ON CONFLICT (user_id) DO NOTHING;

-- +goose Down
DELETE FROM wallets
WHERE id = '22222222-2222-2222-2222-222222222222';
