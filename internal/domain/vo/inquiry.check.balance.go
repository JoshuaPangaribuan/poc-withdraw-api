package vo

import "time"

type BalanceInquiry struct {
	UserID       string    `json:"user_id"`
	BalanceMinor int64     `json:"balance_minor"`
	Currency     string    `json:"currency"`
	UpdatedAt    time.Time `json:"updated_at"`
}
