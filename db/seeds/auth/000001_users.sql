-- +goose Up
INSERT INTO users (id, email, password_hash, status)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'sample.user@example.com',
    '$2a$10$HdK16SQHMMLmXD487u2KkeICvfAP3i105kNIHJykNlEUAzHQonate',
    'active'
)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM users
WHERE id = '11111111-1111-1111-1111-111111111111';
