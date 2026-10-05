package postgrex

import (
	"context"
	"errors"
	"fmt"

	"github.com/StarwardSword/bank/features/accounts/application"
	"github.com/StarwardSword/bank/features/accounts/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ application.Repository = new(Repository)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) (*Repository, error) {
	return &Repository{
		pool: pool,
	}, nil
}

func (r *Repository) SaveAccount(ctx context.Context, acc *domain.Account) error {
	cmdTag, err := r.pool.Exec(ctx, `
		INSERT INTO accounts (id, holder_id)
		SELECT $1, $2
		WHERE EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = $2 AND NOT users.removed
			FOR KEY SHARE
		)
	`, acc.ID, acc.HolderID)

	if err != nil {
		return fmt.Errorf("querying database: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: holder %s", application.ErrNotFound, acc.HolderID.String())
	}

	return nil
}

func (r *Repository) GetById(ctx context.Context, accID uuid.UUID) (domain.Account, error) {
	var (
		holderID = uuid.Nil
		balance  = int64(0)
	)

	err := r.pool.QueryRow(ctx, `
		SELECT
			accounts.holder_id
			,get_account_balance(accounts.id)
		FROM accounts
		WHERE accounts.id = $1 AND NOT accounts.removed
	`, accID).Scan(&holderID, &balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, fmt.Errorf("%w: %s", application.ErrNotFound, accID.String())
	}

	if err != nil {
		return domain.Account{}, fmt.Errorf("querying row: %w", err)
	}

	res := domain.NewAccount(holderID, balance)
	res.ID = accID

	return *res, nil
}

func (r *Repository) GetUserAccounts(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	res := make([]domain.Account, 0, 1)
	rows, err := r.pool.Query(ctx, `
		SELECT
			id
			,get_account_balance(accounts.id)
		FROM accounts
		WHERE holder_id = $1 AND NOT removed
		ORDER BY id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying rows: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id      = uuid.Nil
			balance = int64(0)
		)

		if err := rows.Scan(&id, &balance); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		item := *domain.NewAccount(userID, balance)
		item.ID = id

		res = append(res, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("after scanning: %w", err)
	}

	return res, nil
}

func (r *Repository) UserExist(ctx context.Context, userID uuid.UUID) (bool, error) {
	err := r.pool.QueryRow(ctx, `
		SELECT
			users.id
		FROM users
		WHERE users.id = $1 AND NOT users.removed
	`, userID).Scan(&uuid.UUID{})

	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not query database: %w", err)
	}

	return true, nil
}
