CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS wallets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL UNIQUE,
    balance_minor bigint NOT NULL DEFAULT 0,
    currency varchar(3) NOT NULL DEFAULT 'IDR',
    version bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wallet_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id uuid NOT NULL REFERENCES wallets(id),
    entry_type varchar(30) NOT NULL,
    amount_minor bigint NOT NULL,
    balance_after_minor bigint NOT NULL,
    reference_id varchar(100),
    chain_id varchar(128),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wallet_ledger_wallet_created_at_desc
ON wallet_ledger (wallet_id, created_at DESC);
