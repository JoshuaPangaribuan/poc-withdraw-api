-- +goose Up
ALTER TABLE wallets
DROP CONSTRAINT IF EXISTS wallets_user_id_fkey;

-- +goose Down
ALTER TABLE wallets
DROP CONSTRAINT IF EXISTS wallets_user_id_fkey;

ALTER TABLE wallets
ADD CONSTRAINT wallets_user_id_fkey
FOREIGN KEY (user_id) REFERENCES users(id);
