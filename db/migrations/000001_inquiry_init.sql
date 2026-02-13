-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE wallets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL UNIQUE REFERENCES users(id),
    balance_minor bigint NOT NULL DEFAULT 0,
    currency varchar(3) NOT NULL DEFAULT 'IDR',
    version bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE wallet_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id uuid NOT NULL REFERENCES wallets(id),
    entry_type varchar(30) NOT NULL,
    amount_minor bigint NOT NULL,
    balance_after_minor bigint NOT NULL,
    reference_id varchar(100),
    chain_id varchar(128),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_wallet_ledger_wallet_created_at_desc
ON wallet_ledger (wallet_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_wallet_ledger_wallet_created_at_desc;
DROP TABLE IF EXISTS wallet_ledger;
DROP TABLE IF EXISTS wallets;
DROP TABLE IF EXISTS users;
