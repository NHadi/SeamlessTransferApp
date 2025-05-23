package application

import (
	"context"
	"errors"
	"fmt"
	"internal-transfers/account-service/internal/domain"
	"internal-transfers/account-service/internal/infrastructure/messaging"
	"log/slog"
	"math/big"
	"os"
	"strings"
)

// Common errors that can occur during account operations
var (
	ErrInvalidAmount     = errors.New("invalid amount format")
	ErrNegativeAmount    = errors.New("amount cannot be negative")
	ErrAccountExists     = errors.New("account already exists")
	ErrAccountNotFound   = errors.New("account not found")
	ErrInvalidAccountID  = errors.New("invalid account ID")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// CreateAccountDTO represents the data needed to create a new account
type CreateAccountDTO struct {
	AccountID      domain.AccountID
	InitialBalance string
}

// AccountService defines the interface for account-related operations
type AccountService interface {
	// CreateAccount creates a new account with the specified initial balance
	CreateAccount(ctx context.Context, dto CreateAccountDTO) error
	// GetAccount retrieves an account by its ID
	GetAccount(ctx context.Context, id domain.AccountID) (*domain.Account, error)
	// HandleTransactionSubmitted processes a transaction submitted event
	HandleTransactionSubmitted(ctx context.Context, event domain.TransactionEvent) error
}

type accountService struct {
	repo   domain.AccountRepository
	broker messaging.MessageBroker
	logger *slog.Logger
}

// NewAccountService creates a new instance of AccountService
func NewAccountService(repo domain.AccountRepository, broker messaging.MessageBroker) AccountService {
	return &accountService{
		repo:   repo,
		broker: broker,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// validateAmount checks if the amount string is valid and non-negative
func validateAmount(amount string) error {
	// Remove any whitespace
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return ErrInvalidAmount
	}

	// Parse the amount as a decimal
	value, ok := new(big.Float).SetString(amount)
	if !ok {
		return ErrInvalidAmount
	}

	// Check if the amount is negative
	if value.Sign() < 0 {
		return ErrNegativeAmount
	}

	return nil
}

// validateAccountID checks if the account ID is valid
func validateAccountID(id domain.AccountID) error {
	if id <= 0 {
		return ErrInvalidAccountID
	}
	return nil
}

// CreateAccount implements the account creation logic with validation
func (s *accountService) CreateAccount(ctx context.Context, dto CreateAccountDTO) error {
	s.logger.Info("creating account",
		"account_id", dto.AccountID,
		"initial_balance", dto.InitialBalance)

	// Validate account ID
	if err := validateAccountID(dto.AccountID); err != nil {
		s.logger.Error("invalid account ID",
			"error", err,
			"account_id", dto.AccountID)
		return fmt.Errorf("invalid account ID: %w", err)
	}

	// Validate initial balance
	if err := validateAmount(dto.InitialBalance); err != nil {
		s.logger.Error("invalid initial balance",
			"error", err,
			"amount", dto.InitialBalance)
		return fmt.Errorf("invalid initial balance: %w", err)
	}

	// Check if account already exists
	existingAccount, err := s.repo.GetByID(ctx, dto.AccountID)
	if err == nil && existingAccount != nil {
		s.logger.Warn("account already exists",
			"account_id", dto.AccountID)
		return ErrAccountExists
	}

	// Create new account
	account := &domain.Account{
		ID:      dto.AccountID,
		Balance: dto.InitialBalance,
	}

	// Create account in database
	if err := s.repo.Create(ctx, account); err != nil {
		s.logger.Error("failed to create account",
			"error", err,
			"account_id", dto.AccountID)
		return fmt.Errorf("failed to create account: %w", err)
	}

	s.logger.Info("account created successfully",
		"account_id", account.ID,
		"balance", account.Balance)

	// Publish account created event
	if err := s.broker.PublishAccountCreated(ctx, account); err != nil {
		s.logger.Error("failed to publish account created event",
			"error", err,
			"account_id", account.ID)
	}

	return nil
}

// GetAccount implements the account retrieval logic with validation
func (s *accountService) GetAccount(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	s.logger.Info("getting account",
		"account_id", id)

	// Validate account ID
	if err := validateAccountID(id); err != nil {
		s.logger.Error("invalid account ID",
			"error", err,
			"account_id", id)
		return nil, fmt.Errorf("invalid account ID: %w", err)
	}

	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get account",
			"error", err,
			"account_id", id)
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	if account == nil {
		s.logger.Warn("account not found",
			"account_id", id)
		return nil, ErrAccountNotFound
	}

	s.logger.Info("account retrieved successfully",
		"account_id", account.ID,
		"balance", account.Balance)

	return account, nil
}

// HandleTransactionSubmitted processes a transaction submitted event
func (s *accountService) HandleTransactionSubmitted(ctx context.Context, event domain.TransactionEvent) error {
	s.logger.Info("handling transaction submitted",
		"transaction_id", event.TransactionID,
		"source_account", event.SourceAccountID,
		"destination_account", event.DestinationAccountID,
		"amount", event.Amount)

	// Get source account
	sourceAccount, err := s.repo.GetByID(ctx, event.SourceAccountID)
	if err != nil {
		s.logger.Error("failed to get source account",
			"error", err,
			"account_id", event.SourceAccountID)

		// Publish transaction failed event
		failedEvent := domain.TransactionEvent{
			TransactionID:        event.TransactionID,
			SourceAccountID:      event.SourceAccountID,
			DestinationAccountID: event.DestinationAccountID,
			Amount:               event.Amount,
			Status:               "failed: source account not found",
		}
		if err := s.broker.PublishTransactionFailed(ctx, failedEvent); err != nil {
			s.logger.Error("failed to publish transaction failed event",
				"error", err,
				"transaction_id", event.TransactionID)
		}
		return fmt.Errorf("failed to get source account: %w", err)
	}
	if sourceAccount == nil {
		s.logger.Error("source account not found",
			"account_id", event.SourceAccountID)

		// Publish transaction failed event
		failedEvent := domain.TransactionEvent{
			TransactionID:        event.TransactionID,
			SourceAccountID:      event.SourceAccountID,
			DestinationAccountID: event.DestinationAccountID,
			Amount:               event.Amount,
			Status:               "failed: source account not found",
		}
		if err := s.broker.PublishTransactionFailed(ctx, failedEvent); err != nil {
			s.logger.Error("failed to publish transaction failed event",
				"error", err,
				"transaction_id", event.TransactionID)
		}
		return ErrAccountNotFound
	}

	// Get destination account
	destAccount, err := s.repo.GetByID(ctx, event.DestinationAccountID)
	if err != nil {
		s.logger.Error("failed to get destination account",
			"error", err,
			"account_id", event.DestinationAccountID)

		// Publish transaction failed event
		failedEvent := domain.TransactionEvent{
			TransactionID:        event.TransactionID,
			SourceAccountID:      event.SourceAccountID,
			DestinationAccountID: event.DestinationAccountID,
			Amount:               event.Amount,
			Status:               "failed: destination account not found",
		}
		if err := s.broker.PublishTransactionFailed(ctx, failedEvent); err != nil {
			s.logger.Error("failed to publish transaction failed event",
				"error", err,
				"transaction_id", event.TransactionID)
		}
		return fmt.Errorf("failed to get destination account: %w", err)
	}
	if destAccount == nil {
		s.logger.Error("destination account not found",
			"account_id", event.DestinationAccountID)

		// Publish transaction failed event
		failedEvent := domain.TransactionEvent{
			TransactionID:        event.TransactionID,
			SourceAccountID:      event.SourceAccountID,
			DestinationAccountID: event.DestinationAccountID,
			Amount:               event.Amount,
			Status:               "failed: destination account not found",
		}
		if err := s.broker.PublishTransactionFailed(ctx, failedEvent); err != nil {
			s.logger.Error("failed to publish transaction failed event",
				"error", err,
				"transaction_id", event.TransactionID)
		}
		return ErrAccountNotFound
	}

	// Validate amount
	if err := validateAmount(event.Amount); err != nil {
		s.logger.Error("invalid amount",
			"error", err,
			"amount", event.Amount)

		// Publish transaction failed event
		failedEvent := domain.TransactionEvent{
			TransactionID:        event.TransactionID,
			SourceAccountID:      event.SourceAccountID,
			DestinationAccountID: event.DestinationAccountID,
			Amount:               event.Amount,
			Status:               "failed: invalid amount",
		}
		if err := s.broker.PublishTransactionFailed(ctx, failedEvent); err != nil {
			s.logger.Error("failed to publish transaction failed event",
				"error", err,
				"transaction_id", event.TransactionID)
		}
		return fmt.Errorf("invalid amount: %w", err)
	}

	// Convert balances to big.Float for comparison
	sourceBalance, _ := new(big.Float).SetString(sourceAccount.Balance)
	amount, _ := new(big.Float).SetString(event.Amount)
	destBalance, _ := new(big.Float).SetString(destAccount.Balance)

	// Check if source account has sufficient funds
	if sourceBalance.Cmp(amount) < 0 {
		s.logger.Error("insufficient funds",
			"source_account", event.SourceAccountID,
			"balance", sourceAccount.Balance,
			"amount", event.Amount)

		// Publish transaction failed event
		failedEvent := domain.TransactionEvent{
			TransactionID:        event.TransactionID,
			SourceAccountID:      event.SourceAccountID,
			DestinationAccountID: event.DestinationAccountID,
			Amount:               event.Amount,
			Status:               "failed: insufficient funds",
		}
		if err := s.broker.PublishTransactionFailed(ctx, failedEvent); err != nil {
			s.logger.Error("failed to publish transaction failed event",
				"error", err,
				"transaction_id", event.TransactionID)
		}
		return ErrInsufficientFunds
	}

	// Update balances
	sourceBalance.Sub(sourceBalance, amount)
	destBalance.Add(destBalance, amount)

	// Update accounts
	sourceAccount.Balance = sourceBalance.Text('f', 2)
	destAccount.Balance = destBalance.Text('f', 2)

	// Save changes
	if err := s.repo.Update(ctx, sourceAccount); err != nil {
		s.logger.Error("failed to update source account",
			"error", err,
			"account_id", sourceAccount.ID)

		// Publish transaction failed event
		failedEvent := domain.TransactionEvent{
			TransactionID:        event.TransactionID,
			SourceAccountID:      event.SourceAccountID,
			DestinationAccountID: event.DestinationAccountID,
			Amount:               event.Amount,
			Status:               "failed: could not update source account",
		}
		if err := s.broker.PublishTransactionFailed(ctx, failedEvent); err != nil {
			s.logger.Error("failed to publish transaction failed event",
				"error", err,
				"transaction_id", event.TransactionID)
		}
		return fmt.Errorf("failed to update source account: %w", err)
	}
	if err := s.repo.Update(ctx, destAccount); err != nil {
		s.logger.Error("failed to update destination account",
			"error", err,
			"account_id", destAccount.ID)

		// Publish transaction failed event
		failedEvent := domain.TransactionEvent{
			TransactionID:        event.TransactionID,
			SourceAccountID:      event.SourceAccountID,
			DestinationAccountID: event.DestinationAccountID,
			Amount:               event.Amount,
			Status:               "failed: could not update destination account",
		}
		if err := s.broker.PublishTransactionFailed(ctx, failedEvent); err != nil {
			s.logger.Error("failed to publish transaction failed event",
				"error", err,
				"transaction_id", event.TransactionID)
		}
		return fmt.Errorf("failed to update destination account: %w", err)
	}

	s.logger.Info("accounts updated successfully",
		"source_account", sourceAccount.ID,
		"source_balance", sourceAccount.Balance,
		"destination_account", destAccount.ID,
		"destination_balance", destAccount.Balance)

	// Publish transaction completed event
	completedEvent := domain.TransactionEvent{
		TransactionID:        event.TransactionID,
		SourceAccountID:      event.SourceAccountID,
		DestinationAccountID: event.DestinationAccountID,
		Amount:               event.Amount,
		Status:               "complete",
	}
	if err := s.broker.PublishTransactionCompleted(ctx, completedEvent); err != nil {
		s.logger.Error("failed to publish transaction completed event",
			"error", err,
			"transaction_id", event.TransactionID)
	}

	return nil
}
