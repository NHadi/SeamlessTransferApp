package domain

// TransactionEvent represents a transaction-related event
type TransactionEvent struct {
	TransactionID        TransactionID `json:"transaction_id"`
	SourceAccountID      AccountID     `json:"source_account_id"`
	DestinationAccountID AccountID     `json:"destination_account_id"`
	Amount               string        `json:"amount"`
	Status               string        `json:"status"`
}

// Event types
const (
	EventTransactionSubmitted = "transaction.submitted"
	EventTransactionCompleted = "transaction.completed"
	EventTransactionFailed    = "transaction.failed"
	EventTransactionRollback  = "transaction.rollback"
)
