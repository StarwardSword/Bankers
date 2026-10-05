package postgrex

import (
	"context"
	"errors"
	"fmt"

	"github.com/StarwardSword/bank/features/ledger/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ domain.Journal = new(Repository)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) (*Repository, error) {
	return &Repository{
		pool: pool,
	}, nil
}

func (r *Repository) WithLockedAccount(ctx context.Context, accountID uuid.UUID, fn func(tx domain.JournalTx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("initializing transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		SELECT id
		FROM accounts
		WHERE id = $1 AND NOT removed
		FOR NO KEY UPDATE
	`, accountID).Scan(&uuid.UUID{})

	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, accountID)
	}
	if err != nil {
		return fmt.Errorf("locking account: %w", err)
	}

	if err := fn(NewTransactionRepo(tx)); err != nil {
		return fmt.Errorf("executing transactional func: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commiting transaction: %w", err)
	}

	return nil
}
