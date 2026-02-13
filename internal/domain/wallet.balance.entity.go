package domain

import "time"

type WalletBalance struct {
	UserID       string
	BalanceMinor int64
	Currency     string
	UpdatedAt    time.Time
}
