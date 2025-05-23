package postgres

import (
	"context"
	"fmt"
	"internal-transfers/transaction-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionRepository struct {
	pool *pgxpool.Pool
}

// NewTransactionRepository creates a new instance of TransactionRepository
func NewTransactionRepository(pool *pgxpool.Pool) domain.TransactionRepository {
	return &transactionRepository{pool: pool}
}

// Create creates a new transaction record
func (r *transactionRepository) Create(ctx context.Context, transaction *domain.Transaction) error {
	query := `
		INSERT INTO transactions (
			source_account_id,
			destination_account_id,
			amount,
			status
		) VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		transaction.SourceAccountID,
		transaction.DestinationAccountID,
		transaction.Amount,
		transaction.Status,
	).Scan(&transaction.ID)

	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a transaction by its ID
func (r *transactionRepository) GetByID(ctx context.Context, id domain.TransactionID) (*domain.Transaction, error) {
	query := `
		SELECT id, source_account_id, destination_account_id, amount, status
		FROM transactions
		WHERE id = $1
	`

	var transaction domain.Transaction
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&transaction.ID,
		&transaction.SourceAccountID,
		&transaction.DestinationAccountID,
		&transaction.Amount,
		&transaction.Status,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return &transaction, nil
}

// Update updates a transaction's information
func (r *transactionRepository) Update(ctx context.Context, transaction *domain.Transaction) error {
	query := `
		UPDATE transactions
		SET status = $1
		WHERE id = $2
	`

	_, err := r.pool.Exec(ctx, query, transaction.Status, transaction.ID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}
