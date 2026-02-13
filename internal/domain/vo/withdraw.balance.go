package vo

import "time"

type WalletWithdrawal struct {
	UserID       string    `json:"user_id"`
	AmountMinor  int64     `json:"amount_minor"`
	BalanceMinor int64     `json:"balance_minor"`
	Currency     string    `json:"currency"`
	ChainID      string    `json:"chain_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}
