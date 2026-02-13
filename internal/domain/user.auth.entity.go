package domain

type UserAuth struct {
	ID           string
	Email        string
	PasswordHash string
	Status       string
}
