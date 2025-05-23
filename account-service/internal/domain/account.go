package domain

import "context"

// AccountID represents a unique identifier for an account
type AccountID int64

// TransactionID represents a unique identifier for a transaction
type TransactionID int64

// Account represents a bank account
type Account struct {
	ID      AccountID `json:"id"`
	Balance string    `json:"balance"`
}

type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id AccountID) (*Account, error)
	Update(ctx context.Context, account *Account) error
}
