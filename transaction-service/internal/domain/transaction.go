package domain

import "context"

// TransactionID represents a unique identifier for a transaction
type TransactionID int64

// AccountID represents a unique identifier for an account
type AccountID int64

// TransactionStatus represents the possible states of a transaction
type TransactionStatus string

const (
	TransactionStatusPending  TransactionStatus = "pending"
	TransactionStatusComplete TransactionStatus = "complete"
	TransactionStatusFailed   TransactionStatus = "failed"
	TransactionStatusRollback TransactionStatus = "rollback"
)

// Transaction represents a money transfer between accounts
type Transaction struct {
	ID                   TransactionID     `json:"id"`
	SourceAccountID      AccountID         `json:"source_account_id"`
	DestinationAccountID AccountID         `json:"destination_account_id"`
	Amount               string            `json:"amount"`
	Status               TransactionStatus `json:"status"`
	CreatedAt            string            `json:"created_at"`
	UpdatedAt            string            `json:"updated_at"`
}

type TransactionRepository interface {
	Create(ctx context.Context, transaction *Transaction) error
	GetByID(ctx context.Context, id TransactionID) (*Transaction, error)
	Update(ctx context.Context, transaction *Transaction) error
}
