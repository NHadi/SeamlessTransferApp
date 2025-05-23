package application

import (
	"context"
	"errors"
	"fmt"
	"internal-transfers/transaction-service/internal/domain"
	"internal-transfers/transaction-service/internal/infrastructure/messaging"
	"log/slog"
	"os"
)

// Common errors
var (
	ErrSameAccount       = errors.New("source and destination accounts cannot be the same")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountNotFound   = errors.New("account not found")
)

// TransactionService defines the interface for transaction operations
type TransactionService interface {
	SubmitTransaction(ctx context.Context, dto TransactionDTO) error
	GetTransaction(ctx context.Context, id domain.TransactionID) (*domain.Transaction, error)
	HandleTransactionCompleted(ctx context.Context, event domain.TransactionEvent) error
	HandleTransactionFailed(ctx context.Context, event domain.TransactionEvent) error
}

type transactionService struct {
	repo   domain.TransactionRepository
	broker messaging.MessageBroker
	logger *slog.Logger
}

// NewTransactionService creates a new instance of TransactionService
func NewTransactionService(repo domain.TransactionRepository, broker messaging.MessageBroker) TransactionService {
	return &transactionService{
		repo:   repo,
		broker: broker,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// TransactionDTO represents the data needed to create a new transaction
type TransactionDTO struct {
	SourceAccountID      domain.AccountID
	DestinationAccountID domain.AccountID
	Amount               string
}

// SubmitTransaction implements the transaction submission logic
func (s *transactionService) SubmitTransaction(ctx context.Context, dto TransactionDTO) error {
	s.logger.Info("submitting transaction",
		"source_account", dto.SourceAccountID,
		"destination_account", dto.DestinationAccountID,
		"amount", dto.Amount)

	// Validate source and destination accounts are different
	if dto.SourceAccountID == dto.DestinationAccountID {
		s.logger.Error("same account transfer attempted",
			"account_id", dto.SourceAccountID)
		return ErrSameAccount
	}

	// Create transaction record
	transaction := &domain.Transaction{
		SourceAccountID:      dto.SourceAccountID,
		DestinationAccountID: dto.DestinationAccountID,
		Amount:               dto.Amount,
		Status:               domain.TransactionStatusPending,
	}

	// Save transaction to database
	if err := s.repo.Create(ctx, transaction); err != nil {
		s.logger.Error("failed to create transaction",
			"error", err,
			"source_account", dto.SourceAccountID,
			"destination_account", dto.DestinationAccountID)
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	s.logger.Info("transaction created",
		"transaction_id", transaction.ID,
		"status", transaction.Status)

	// Publish transaction submitted event
	event := domain.TransactionEvent{
		TransactionID:        transaction.ID,
		SourceAccountID:      transaction.SourceAccountID,
		DestinationAccountID: transaction.DestinationAccountID,
		Amount:               transaction.Amount,
		Status:               string(transaction.Status),
	}

	if err := s.broker.PublishTransactionSubmitted(ctx, event); err != nil {
		s.logger.Error("failed to publish transaction event",
			"error", err,
			"transaction_id", transaction.ID)
		// Log the error and mark transaction as failed
		transaction.Status = domain.TransactionStatusFailed
		if updateErr := s.repo.Update(ctx, transaction); updateErr != nil {
			s.logger.Error("failed to update transaction status",
				"error", updateErr,
				"transaction_id", transaction.ID)
		}
		return fmt.Errorf("failed to publish transaction event: %w", err)
	}

	s.logger.Info("transaction event published",
		"transaction_id", transaction.ID,
		"event_type", "transaction.submitted")

	return nil
}

// GetTransaction implements the transaction retrieval logic
func (s *transactionService) GetTransaction(ctx context.Context, id domain.TransactionID) (*domain.Transaction, error) {
	s.logger.Info("getting transaction",
		"transaction_id", id)

	transaction, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get transaction",
			"error", err,
			"transaction_id", id)
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	if transaction == nil {
		s.logger.Warn("transaction not found",
			"transaction_id", id)
		return nil, fmt.Errorf("transaction not found")
	}

	s.logger.Info("transaction retrieved",
		"transaction_id", id,
		"status", transaction.Status)

	return transaction, nil
}

// HandleTransactionCompleted updates transaction status when completed
func (s *transactionService) HandleTransactionCompleted(ctx context.Context, event domain.TransactionEvent) error {
	s.logger.Info("handling transaction completed",
		"transaction_id", event.TransactionID)

	transaction, err := s.repo.GetByID(ctx, event.TransactionID)
	if err != nil {
		s.logger.Error("failed to get transaction for completion",
			"error", err,
			"transaction_id", event.TransactionID)
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	transaction.Status = domain.TransactionStatusComplete
	if err := s.repo.Update(ctx, transaction); err != nil {
		s.logger.Error("failed to update transaction status to complete",
			"error", err,
			"transaction_id", event.TransactionID)
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	s.logger.Info("transaction marked as complete",
		"transaction_id", event.TransactionID)

	return nil
}

// HandleTransactionFailed updates transaction status when failed
func (s *transactionService) HandleTransactionFailed(ctx context.Context, event domain.TransactionEvent) error {
	s.logger.Info("handling transaction failed",
		"transaction_id", event.TransactionID,
		"error", event.Status)

	transaction, err := s.repo.GetByID(ctx, event.TransactionID)
	if err != nil {
		s.logger.Error("failed to get transaction for failure",
			"error", err,
			"transaction_id", event.TransactionID)
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if transaction == nil {
		s.logger.Warn("transaction not found for failure",
			"transaction_id", event.TransactionID)
		return nil
	}

	// Update transaction status
	transaction.Status = domain.TransactionStatusFailed
	if err := s.repo.Update(ctx, transaction); err != nil {
		s.logger.Error("failed to update transaction status to failed",
			"error", err,
			"transaction_id", event.TransactionID)
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	s.logger.Info("transaction marked as failed",
		"transaction_id", event.TransactionID,
		"error", event.Status)

	return nil
}
