package application_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/StarwardSword/bank/features/accounts/application"
	"github.com/StarwardSword/bank/features/accounts/domain"
	"github.com/StarwardSword/bank/features/accounts/infrastructure/postgrex"
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

	app, err := application.NewApplication(repo, tableLedger{pool: pool})
	if err != nil {
		t.Fatalf("creating application: %v", err)
	}

	return t.Context(), app, pool
}

func createUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := pool.QueryRow(t.Context(), `
		INSERT INTO users (username, password, role)
		VALUES ($1, 'hash', 1)
		RETURNING id
	`, uuid.NewString()).Scan(&id)
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	return id
}

func createAccount(t *testing.T, app *application.Application, userID uuid.UUID) uuid.UUID {
	t.Helper()

	accountID, err := app.CreateAccount(t.Context(), userID)
	if err != nil {
		t.Fatalf("creating account: %v", err)
	}

	return accountID
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

func requireAppBalance(t *testing.T, app *application.Application, userID, accountID uuid.UUID, want int64) {
	t.Helper()

	got, err := app.GetBalance(t.Context(), userID, accountID)
	if err != nil || got != want {
		t.Fatalf("GetBalance(%s) = %d, %v; want %d, nil", accountID, got, err, want)
	}
}

func TestApplicationPostgresCreateAccount(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	userID := createUser(t, pool)

	accountID, err := app.CreateAccount(ctx, userID)
	if err != nil {
		t.Fatalf("could not create account: %v", err)
	}

	account, err := app.GetById(ctx, userID, accountID)
	if err != nil {
		t.Fatalf("could not get created account: %v", err)
	}
	if account.ID != accountID || account.HolderID != userID || account.Balance() != 0 {
		t.Fatalf("account = {%s %s %d}; want {%s %s 0}", account.ID, account.HolderID, account.Balance(), accountID, userID)
	}
}

func TestApplicationPostgresCreateAccountUnknownUser(t *testing.T) {
	ctx, app, _ := setupPostgres(t)

	if _, err := app.CreateAccount(ctx, uuid.New()); err == nil {
		t.Fatalf("created an account for a user that does not exist")
	}
}

func TestApplicationPostgresGetBalance(t *testing.T) {
	_, app, pool := setupPostgres(t)
	userID := createUser(t, pool)
	accountID := createAccount(t, app, userID)

	fund(t, pool, accountID, 42)

	requireAppBalance(t, app, userID, accountID, 42)
}

func TestApplicationPostgresGetBalanceNotHolder(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	accountID := createAccount(t, app, createUser(t, pool))

	if _, err := app.GetBalance(ctx, createUser(t, pool), accountID); !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotHolder)
	}
}

func TestApplicationPostgresTransfer(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	userID := createUser(t, pool)
	targetHolderID := createUser(t, pool)
	sourceID := createAccount(t, app, userID)
	targetID := createAccount(t, app, targetHolderID)
	fund(t, pool, sourceID, 100)

	ok, err := app.Transfer(ctx, userID, sourceID, targetID, 30)
	if err != nil || !ok {
		t.Fatalf("Transfer = %t, %v; want true, nil", ok, err)
	}

	requireAppBalance(t, app, userID, sourceID, 70)
	requireAppBalance(t, app, targetHolderID, targetID, 30)
}

func TestApplicationPostgresTransferInsufficientFunds(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	userID := createUser(t, pool)
	targetHolderID := createUser(t, pool)
	sourceID := createAccount(t, app, userID)
	targetID := createAccount(t, app, targetHolderID)
	fund(t, pool, sourceID, 29)

	ok, err := app.Transfer(ctx, userID, sourceID, targetID, 30)
	if ok || !errors.Is(err, domain.ErrInsufficientFund) {
		t.Fatalf("Transfer = %t, %v; want false, %v", ok, err, domain.ErrInsufficientFund)
	}

	requireAppBalance(t, app, userID, sourceID, 29)
	requireAppBalance(t, app, targetHolderID, targetID, 0)
}

func TestApplicationPostgresTransferNotHolder(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	userID := createUser(t, pool)
	sourceID := createAccount(t, app, userID)
	targetID := createAccount(t, app, createUser(t, pool))
	fund(t, pool, sourceID, 100)

	ok, err := app.Transfer(ctx, createUser(t, pool), sourceID, targetID, 30)
	if ok || !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("Transfer = %t, %v; want false, %v", ok, err, domain.ErrNotHolder)
	}

	requireAppBalance(t, app, userID, sourceID, 100)
}

func TestApplicationPostgresTransferUnknownTarget(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	userID := createUser(t, pool)
	sourceID := createAccount(t, app, userID)
	fund(t, pool, sourceID, 100)

	ok, err := app.Transfer(ctx, userID, sourceID, uuid.New(), 30)
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Transfer = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}

	requireAppBalance(t, app, userID, sourceID, 100)
}

func TestApplicationPostgresGetUserAccounts(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	userID := createUser(t, pool)
	first := createAccount(t, app, userID)
	second := createAccount(t, app, userID)
	createAccount(t, app, createUser(t, pool))
	fund(t, pool, second, 50)

	accounts, err := app.GetUserAccounts(ctx, userID)
	if err != nil {
		t.Fatalf("could not get user accounts: %v", err)
	}

	got := map[uuid.UUID]int64{}
	for _, account := range accounts {
		if account.HolderID != userID {
			t.Fatalf("account %s belongs to %s; want %s", account.ID, account.HolderID, userID)
		}
		got[account.ID] = account.Balance()
	}

	want := map[uuid.UUID]int64{first: 0, second: 50}
	if len(accounts) != len(want) || !maps.Equal(got, want) {
		t.Fatalf("accounts = %v; want %v", got, want)
	}
}

func TestApplicationPostgresGetUserAccountsUnknownUser(t *testing.T) {
	ctx, app, _ := setupPostgres(t)

	if _, err := app.GetUserAccounts(ctx, uuid.New()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, application.ErrNotFound)
	}
}

func TestApplicationPostgresGetUserAccountsContextCancelled(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	userID := createUser(t, pool)
	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	if _, err := app.GetUserAccounts(ctx, userID); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v; want %v", err, context.Canceled)
	}
}

func TestApplicationPostgresGetByIdNotHolder(t *testing.T) {
	ctx, app, pool := setupPostgres(t)
	accountID := createAccount(t, app, createUser(t, pool))

	if _, err := app.GetById(ctx, createUser(t, pool), accountID); !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotHolder)
	}
}

type tableLedger struct {
	pool *pgxpool.Pool
}

func (l tableLedger) MakeTransaction(ctx context.Context, sourceID, targetID uuid.UUID, amount int64) (uuid.UUID, error) {
	id := uuid.New()
	_, err := l.pool.Exec(ctx, `
		INSERT INTO transaction (id, debit_account_id, credit_account_id, amount)
		VALUES ($1, $2, $3, $4)
	`, id, sourceID, targetID, amount)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}
