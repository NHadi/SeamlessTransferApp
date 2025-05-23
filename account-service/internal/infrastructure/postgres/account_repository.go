package postgres

import (
	"context"
	"fmt"
	"internal-transfers/account-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountRepository struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) domain.AccountRepository {
	return &AccountRepository{
		db: db,
	}
}

func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	query := `
		INSERT INTO accounts (id, balance)
		VALUES ($1, $2)
	`

	if _, err := r.db.Exec(ctx, query, account.ID, account.Balance); err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	return nil
}

func (r *AccountRepository) GetByID(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	query := `
		SELECT id, balance
		FROM accounts
		WHERE id = $1
	`

	account := &domain.Account{}
	if err := r.db.QueryRow(ctx, query, id).Scan(&account.ID, &account.Balance); err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return account, nil
}

func (r *AccountRepository) Update(ctx context.Context, account *domain.Account) error {
	query := `
		UPDATE accounts
		SET balance = $2
		WHERE id = $1
	`

	if _, err := r.db.Exec(ctx, query, account.ID, account.Balance); err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	return nil
}
