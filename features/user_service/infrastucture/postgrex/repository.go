package postgrex

import (
	"context"
	"errors"
	"fmt"

	"github.com/StarwardSword/bank/features/user_service/application"
	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ application.Repository = new(Repository)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func (r *Repository) SaveUser(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, username, password, role)
		VALUES ($1, $2, $3, $4)
	`, u.ID, u.Username, u.Password, u.Role)

	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return fmt.Errorf("username already in use: %w", application.ErrAlreadyExists)
	}
	if err != nil {
		return fmt.Errorf("querying database: %w", err)
	}

	return nil
}

func (r *Repository) RemoveUser(ctx context.Context, u *domain.User) error {
	// TODO: Возвращение средств в казну при удалении счета
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("creating pgx.tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cmdTag, err := tx.Exec(ctx, `
		UPDATE users SET
			removed = true,
			username = $2
		WHERE id = $1 AND NOT removed
	`, u.ID, "removed_user_"+uuid.NewString())
	if err != nil {
		return fmt.Errorf("removing user %s: %w", u.ID.String(), err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", application.ErrNotFound, u.ID.String())
	}

	_, err = tx.Exec(ctx, `
		UPDATE accounts SET
			removed = true
		WHERE holder_id = $1 AND NOT removed
	`, u.ID)
	if err != nil {
		return fmt.Errorf("removing accounts for user %s: %w", u.ID.String(), err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commiiting transaction: %w", err)
	}

	return nil
}

func (r *Repository) UpdateUser(ctx context.Context, u *domain.User) error {
	cmdTag, err := r.pool.Exec(ctx, `
		UPDATE users SET
			username = $1,
			password = $2,
			role = $3
		WHERE id = $4 AND NOT removed
	`, u.Username, u.Password, u.Role, u.ID)
	if err != nil {
		return fmt.Errorf("updating table: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", application.ErrNotFound, u.ID.String())
	}

	return nil
}

func (r *Repository) GetUserById(ctx context.Context, id uuid.UUID) (domain.User, error) {
	res := domain.User{}
	err := r.pool.QueryRow(ctx, `
		SELECT 
			users.id
			,users.username
			,users.password 
			,users.role
		FROM users
		WHERE users.id = $1 AND NOT users.removed
	`, id).Scan(&res.ID, &res.Username, &res.Password, &res.Role)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("not found: %w", application.ErrNotFound)
	}

	if err != nil {
		return domain.User{}, fmt.Errorf("querying row: %w", err)
	}

	return res, nil
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	res := domain.User{}
	err := r.pool.QueryRow(ctx, `
		SELECT 
			users.id
			,users.username
			,users.password 
			,users.role
		FROM users
		WHERE users.username = $1 AND NOT users.removed
	`, username).Scan(&res.ID, &res.Username, &res.Password, &res.Role)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("not found: %w", application.ErrNotFound)
	}

	if err != nil {
		return domain.User{}, fmt.Errorf("querying row: %w", err)
	}

	return res, nil
}
