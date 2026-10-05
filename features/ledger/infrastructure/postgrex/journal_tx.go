package postgrex

import (
	"context"
	"errors"
	"fmt"

	"github.com/StarwardSword/bank/features/ledger/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type TransactionRepository struct {
	transaction pgx.Tx
}

func NewTransactionRepo(tx pgx.Tx) *TransactionRepository {
	return &TransactionRepository{
		transaction: tx,
	}
}

func (r *TransactionRepository) Balance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	var balance int64

	err := r.transaction.QueryRow(ctx, `
		SELECT get_account_balance($1)
	`, accountID).Scan(&balance)

	// get_account_balance не вернёт pgx.ErrNoRows, так что проверять существование нужно выше

	if err != nil {
		return 0, fmt.Errorf("querying row: %w", err)
	}

	return balance, nil
}

func (r *TransactionRepository) Append(ctx context.Context, txr domain.TransactionRecord) error {
	cmdTag, err := r.transaction.Exec(ctx, `
		INSERT INTO transaction(id, created_at, debit_account_id, credit_account_id, amount)
		SELECT $1, $2, $3, $4, $5
		WHERE EXISTS (
			SELECT 1
			FROM accounts
			WHERE accounts.id = $4 AND NOT accounts.removed
		)
	`, txr.ID, txr.Created, txr.SourceID, txr.TargetID, txr.Amount)

	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23503" {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, txr.TargetID.String())
	}
	if err != nil {
		return fmt.Errorf("writing transaction to db: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, txr.TargetID.String())
	}

	return nil
}
