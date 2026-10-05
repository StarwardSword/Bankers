package postgrex_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/StarwardSword/bank/features/accounts/application"
	"github.com/StarwardSword/bank/features/accounts/domain"
	"github.com/StarwardSword/bank/features/accounts/infrastructure/postgrex"
	"github.com/StarwardSword/bank/pkg/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setup(t *testing.T) (context.Context, *postgrex.Repository, *pgxpool.Pool) {
	t.Helper()

	pool := testdb.Pool(t)
	repo, err := postgrex.NewRepository(pool)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}

	return t.Context(), repo, pool
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

func createAccount(t *testing.T, repo *postgrex.Repository, holderID uuid.UUID) *domain.Account {
	t.Helper()

	account := domain.NewAccount(holderID, 0)
	if err := repo.SaveAccount(t.Context(), account); err != nil {
		t.Fatalf("saving account: %v", err)
	}

	return account
}

func treasuryAccount(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := pool.QueryRow(t.Context(), `
		SELECT accounts.id
		FROM accounts
		JOIN users ON users.id = accounts.holder_id
		WHERE users.username = 'treasury'
	`).Scan(&id)
	if err != nil {
		t.Fatalf("finding treasury account: %v", err)
	}

	return id
}

func transfer(t *testing.T, pool *pgxpool.Pool, sourceID, targetID uuid.UUID, amount int64) {
	t.Helper()

	_, err := pool.Exec(t.Context(), `
		INSERT INTO transaction (debit_account_id, credit_account_id, amount)
		VALUES ($1, $2, $3)
	`, sourceID, targetID, amount)
	if err != nil {
		t.Fatalf("transferring %d from %s to %s: %v", amount, sourceID, targetID, err)
	}
}

func requireAccount(t *testing.T, got domain.Account, id, holderID uuid.UUID, balance int64) {
	t.Helper()

	if got.ID != id || got.HolderID != holderID || got.Balance() != balance {
		t.Fatalf("account = {id: %s, holder: %s, balance: %d}; want {id: %s, holder: %s, balance: %d}",
			got.ID, got.HolderID, got.Balance(), id, holderID, balance)
	}
}

func TestRepositorySaveAccount(t *testing.T) {
	ctx, repo, pool := setup(t)
	holderID := createUser(t, pool)
	account := domain.NewAccount(holderID, 0)
	id := account.ID

	if err := repo.SaveAccount(ctx, account); err != nil {
		t.Fatalf("could not save account: %v", err)
	}
	if account.ID != id {
		t.Fatalf("account id changed on save: %s -> %s", id, account.ID)
	}

	got, err := repo.GetById(ctx, id)
	if err != nil {
		t.Fatalf("could not get saved account: %v", err)
	}

	requireAccount(t, got, id, holderID, 0)
}

func TestRepositorySaveAccountIgnoresBalanceSnapshot(t *testing.T) {
	ctx, repo, pool := setup(t)
	holderID := createUser(t, pool)
	account := domain.NewAccount(holderID, 100)

	if err := repo.SaveAccount(ctx, account); err != nil {
		t.Fatalf("could not save account: %v", err)
	}

	got, err := repo.GetById(ctx, account.ID)
	if err != nil {
		t.Fatalf("could not get saved account: %v", err)
	}

	requireAccount(t, got, account.ID, holderID, 0)
}

func TestRepositorySaveAccountUnknownHolder(t *testing.T) {
	ctx, repo, _ := setup(t)
	account := domain.NewAccount(uuid.New(), 0)

	if err := repo.SaveAccount(ctx, account); err == nil {
		t.Fatalf("saved an account of a user that does not exist")
	}

	if _, err := repo.GetById(ctx, account.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("GetById err = %v; want %v", err, application.ErrNotFound)
	}
}

func TestRepositorySaveAccountDoesNotOverwrite(t *testing.T) {
	ctx, repo, pool := setup(t)
	holderID := createUser(t, pool)
	account := createAccount(t, repo, holderID)

	stolen := domain.NewAccount(createUser(t, pool), 0)
	stolen.ID = account.ID

	err := repo.SaveAccount(ctx, stolen)
	if err == nil {
		t.Fatalf("saved an account over an existing one")
	}
	if errors.Is(err, application.ErrNotFound) {
		t.Fatalf("duplicate account reported as %v", application.ErrNotFound)
	}

	got, err := repo.GetById(ctx, account.ID)
	if err != nil {
		t.Fatalf("could not get account: %v", err)
	}

	requireAccount(t, got, account.ID, holderID, 0)
}

func TestRepositoryGetByIdNotFound(t *testing.T) {
	ctx, repo, _ := setup(t)

	_, err := repo.GetById(ctx, uuid.New())
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, application.ErrNotFound)
	}
}

func TestRepositoryGetByIdBalance(t *testing.T) {
	ctx, repo, pool := setup(t)
	source := createAccount(t, repo, createUser(t, pool))
	target := createAccount(t, repo, createUser(t, pool))

	transfer(t, pool, treasuryAccount(t, pool), source.ID, 100)
	transfer(t, pool, source.ID, target.ID, 30)

	gotSource, err := repo.GetById(ctx, source.ID)
	if err != nil {
		t.Fatalf("could not get source: %v", err)
	}
	requireAccount(t, gotSource, source.ID, source.HolderID, 70)

	gotTarget, err := repo.GetById(ctx, target.ID)
	if err != nil {
		t.Fatalf("could not get target: %v", err)
	}
	requireAccount(t, gotTarget, target.ID, target.HolderID, 30)
}

func TestRepositoryGetUserAccounts(t *testing.T) {
	ctx, repo, pool := setup(t)
	holderID := createUser(t, pool)
	first := createAccount(t, repo, holderID)
	second := createAccount(t, repo, holderID)
	createAccount(t, repo, createUser(t, pool))

	transfer(t, pool, treasuryAccount(t, pool), second.ID, 50)

	accounts, err := repo.GetUserAccounts(ctx, holderID)
	if err != nil {
		t.Fatalf("could not get user accounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("got %d account(s); want 2", len(accounts))
	}

	byID := make(map[uuid.UUID]domain.Account, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}

	requireAccount(t, byID[first.ID], first.ID, holderID, 0)
	requireAccount(t, byID[second.ID], second.ID, holderID, 50)
}

func TestRepositoryGetUserAccountsOrder(t *testing.T) {
	ctx, repo, pool := setup(t)
	holderID := createUser(t, pool)

	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(b[:], a[:]) })

	for _, id := range ids {
		account := domain.NewAccount(holderID, 0)
		account.ID = id
		if err := repo.SaveAccount(ctx, account); err != nil {
			t.Fatalf("saving account: %v", err)
		}
	}

	accounts, err := repo.GetUserAccounts(ctx, holderID)
	if err != nil {
		t.Fatalf("could not get user accounts: %v", err)
	}

	got := make([]uuid.UUID, 0, len(accounts))
	for _, account := range accounts {
		got = append(got, account.ID)
	}

	/*
		Вначале тест сортирует Id в убывающем порядке
		База возвращает accounts в возрастающем порядке их id
		3десь ids переворачивается и получается сортировка по возрастанию
	*/
	slices.Reverse(ids)
	if !slices.Equal(got, ids) {
		t.Fatalf("account order = %v; want %v", got, ids)
	}
}

func TestRepositoryGetUserAccountsEmpty(t *testing.T) {
	ctx, repo, pool := setup(t)

	accounts, err := repo.GetUserAccounts(ctx, createUser(t, pool))
	if err != nil {
		t.Fatalf("could not get user accounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("got %d account(s); want none", len(accounts))
	}
}

func TestRepositoryUserExist(t *testing.T) {
	ctx, repo, pool := setup(t)

	exist, err := repo.UserExist(ctx, createUser(t, pool))
	if err != nil || !exist {
		t.Fatalf("UserExist = %t, %v; want true, nil", exist, err)
	}
}

func TestRepositoryUserExistUnknown(t *testing.T) {
	ctx, repo, _ := setup(t)

	exist, err := repo.UserExist(ctx, uuid.New())
	if err != nil || exist {
		t.Fatalf("UserExist = %t, %v; want false, nil", exist, err)
	}
}

func TestRepositoryContextCancelled(t *testing.T) {
	ctx, repo, pool := setup(t)
	holderID := createUser(t, pool)
	account := createAccount(t, repo, holderID)

	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	t.Run("SaveAccount", func(t *testing.T) {
		err := repo.SaveAccount(ctx, domain.NewAccount(holderID, 0))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})

	t.Run("GetById", func(t *testing.T) {
		_, err := repo.GetById(ctx, account.ID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})

	t.Run("GetUserAccounts", func(t *testing.T) {
		_, err := repo.GetUserAccounts(ctx, holderID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})

	t.Run("UserExist", func(t *testing.T) {
		_, err := repo.UserExist(ctx, holderID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})
}
