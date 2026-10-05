package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/StarwardSword/bank/features/ledger/application"
	"github.com/StarwardSword/bank/features/ledger/domain"
	"github.com/StarwardSword/bank/features/ledger/infrastructure/postgrex"
	"github.com/StarwardSword/bank/pkg/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupPostgres(t *testing.T) (context.Context, *application.Application, *pgxpool.Pool) {
	t.Helper()

	pool := testdb.Pool(t)
	repo, err := postgrex.NewRepository(pool)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}

	app, err := application.NewApplication(repo)
	if err != nil {
		t.Fatalf("creating application: %v", err)
	}

	return t.Context(), app, pool
}

func createAccount(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := pool.QueryRow(t.Context(), `
		WITH holder AS (
			INSERT INTO users (username, password, role)
			VALUES ($1, 'hash', 1)
			RETURNING id
		)
		INSERT INTO accounts (holder_id)
		SELECT id FROM holder
		RETURNING id
	`, uuid.NewString()).Scan(&id)
	if err != nil {
		t.Fatalf("creating account: %v", err)
	}

	return id
}

func fund(t *testing.T, pool *pgxpool.Pool, accountID uuid.UUID, amount int64) {
	t.Helper()

	tag, err := pool.Exec(t.Context(), `
		INSERT INTO transaction (debit_account_id, credit_account_id, amount)
		SELECT accounts.id, $1, $2
		FROM accounts
		JOIN users ON users.id = accounts.holder_id
		WHERE users.username = 'treasury'
	`, accountID, amount)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("funding %s with %d from the treasury: %v (rows: %d)", accountID, amount, err, tag.RowsAffected())
	}
}

func requirePostgresBalance(t *testing.T, pool *pgxpool.Pool, accountID uuid.UUID, want int64) {
	t.Helper()

	var got int64
	err := pool.QueryRow(t.Context(), `
		SELECT get_account_balance($1)
	`, accountID).Scan(&got)
	if err != nil || got != want {
		t.Fatalf("balance of %s = %d, %v; want %d, nil", accountID, got, err, want)
	}
}

func requirePostgresRecord(t *testing.T, pool *pgxpool.Pool, want domain.TransactionRecord) {
	t.Helper()

	got := domain.TransactionRecord{ID: want.ID}
	err := pool.QueryRow(t.Context(), `
		SELECT
			debit_account_id
			,credit_account_id
			,amount
		FROM transaction
		WHERE id = $1
	`, want.ID).Scan(&got.SourceID, &got.TargetID, &got.Amount)
	if err != nil {
		t.Fatalf("reading transaction %s: %v", want.ID, err)
	}
	if got != want {
		t.Fatalf("transaction = %+v; want %+v", got, want)
	}
}

func TestApplicationPostgresMakeTransaction(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	sourceID := createAccount(t, pool)
	targetID := createAccount(t, pool)
	fund(t, pool, sourceID, 100)

	tid, err := app.MakeTransaction(ctx, sourceID, targetID, 30)
	if err != nil {
		t.Fatalf("could not make transaction: %v", err)
	}

	requirePostgresRecord(t, pool, domain.TransactionRecord{
		ID:       tid,
		SourceID: sourceID,
		TargetID: targetID,
		Amount:   30,
	})
	requirePostgresBalance(t, pool, sourceID, 70)
	requirePostgresBalance(t, pool, targetID, 30)
}

func TestApplicationPostgresMakeTransactionInsufficientFunds(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	sourceID := createAccount(t, pool)
	targetID := createAccount(t, pool)
	fund(t, pool, sourceID, 29)

	tid, err := app.MakeTransaction(ctx, sourceID, targetID, 30)
	if tid != uuid.Nil || !errors.Is(err, domain.ErrInsufficientFund) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, domain.ErrInsufficientFund)
	}

	requirePostgresBalance(t, pool, sourceID, 29)
	requirePostgresBalance(t, pool, targetID, 0)
}

func TestApplicationPostgresMakeTransactionUnknownSource(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	targetID := createAccount(t, pool)

	tid, err := app.MakeTransaction(ctx, uuid.New(), targetID, 30)
	if tid != uuid.Nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, domain.ErrNotFound)
	}

	requirePostgresBalance(t, pool, targetID, 0)
}

func TestApplicationPostgresMakeTransactionUnknownTarget(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	sourceID := createAccount(t, pool)
	fund(t, pool, sourceID, 100)

	tid, err := app.MakeTransaction(ctx, sourceID, uuid.New(), 30)
	if tid != uuid.Nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, domain.ErrNotFound)
	}

	requirePostgresBalance(t, pool, sourceID, 100)
}

func TestApplicationPostgresMakeTransactionContextCancelled(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	sourceID := createAccount(t, pool)
	targetID := createAccount(t, pool)
	fund(t, pool, sourceID, 100)

	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	tid, err := app.MakeTransaction(ctx, sourceID, targetID, 30)
	if tid != uuid.Nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, context.Canceled)
	}

	requirePostgresBalance(t, pool, sourceID, 100)
	requirePostgresBalance(t, pool, targetID, 0)
}
