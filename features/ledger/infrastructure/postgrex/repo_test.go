package postgrex_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StarwardSword/bank/features/ledger/domain"
	"github.com/StarwardSword/bank/features/ledger/infrastructure/postgrex"
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

func fund(t *testing.T, pool *pgxpool.Pool, accountID uuid.UUID, amount int64) {
	t.Helper()

	transfer(t, pool, treasuryAccount(t, pool), accountID, amount)
}

func newRecord(sourceID, targetID uuid.UUID, amount int64) domain.TransactionRecord {
	return domain.TransactionRecord{
		ID:       uuid.New(),
		SourceID: sourceID,
		TargetID: targetID,
		Amount:   amount,
		Created:  time.Now().UTC(),
	}
}

func requireBalance(t *testing.T, pool *pgxpool.Pool, accountID uuid.UUID, want int64) {
	t.Helper()

	var got int64
	err := pool.QueryRow(t.Context(), `
		SELECT get_account_balance($1)
	`, accountID).Scan(&got)
	if err != nil {
		t.Fatalf("getting balance of %s: %v", accountID, err)
	}
	if got != want {
		t.Fatalf("balance of %s = %d; want %d", accountID, got, want)
	}
}

func requireRecord(t *testing.T, pool *pgxpool.Pool, want domain.TransactionRecord) {
	t.Helper()

	got := domain.TransactionRecord{ID: want.ID}
	err := pool.QueryRow(t.Context(), `
		SELECT
			debit_account_id
			,credit_account_id
			,amount
			,created_at
		FROM transaction
		WHERE id = $1
	`, want.ID).Scan(&got.SourceID, &got.TargetID, &got.Amount, &got.Created)
	if err != nil {
		t.Fatalf("reading record %s: %v", want.ID, err)
	}

	want.Created = want.Created.Truncate(time.Microsecond)
	if got.SourceID != want.SourceID || got.TargetID != want.TargetID || got.Amount != want.Amount || !got.Created.Equal(want.Created) {
		t.Fatalf("record = %+v; want %+v", got, want)
	}
}

func requireNoRecord(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	var count int
	err := pool.QueryRow(t.Context(), `
		SELECT count(*)
		FROM transaction
		WHERE id = $1
	`, id).Scan(&count)
	if err != nil {
		t.Fatalf("counting records with id %s: %v", id, err)
	}
	if count != 0 {
		t.Fatalf("record %s was written; want none", id)
	}
}

func TestRepositoryAppend(t *testing.T) {
	ctx, repo, pool := setup(t)
	source := createAccount(t, pool)
	target := createAccount(t, pool)
	fund(t, pool, source, 100)
	record := newRecord(source, target, 30)

	err := repo.WithLockedAccount(ctx, source, func(tx domain.JournalTx) error {
		return tx.Append(ctx, record)
	})
	if err != nil {
		t.Fatalf("could not append record: %v", err)
	}

	requireRecord(t, pool, record)
	requireBalance(t, pool, source, 70)
	requireBalance(t, pool, target, 30)
}

func TestRepositoryAppendUnknownTarget(t *testing.T) {
	ctx, repo, pool := setup(t)
	source := createAccount(t, pool)
	fund(t, pool, source, 100)
	record := newRecord(source, uuid.New(), 30)

	err := repo.WithLockedAccount(ctx, source, func(tx domain.JournalTx) error {
		return tx.Append(ctx, record)
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotFound)
	}

	requireNoRecord(t, pool, record.ID)
	requireBalance(t, pool, source, 100)
}

func TestRepositoryBalance(t *testing.T) {
	ctx, repo, pool := setup(t)
	account := createAccount(t, pool)
	other := createAccount(t, pool)
	fund(t, pool, account, 100)
	transfer(t, pool, account, other, 30)
	transfer(t, pool, other, account, 5)

	var balance int64
	err := repo.WithLockedAccount(ctx, account, func(tx domain.JournalTx) error {
		var err error
		balance, err = tx.Balance(ctx, account)
		return err
	})
	if err != nil || balance != 75 {
		t.Fatalf("Balance = %d, %v; want 75, nil", balance, err)
	}
}

func TestRepositoryWithLockedAccountNotFound(t *testing.T) {
	ctx, repo, _ := setup(t)

	called := false
	err := repo.WithLockedAccount(ctx, uuid.New(), func(domain.JournalTx) error {
		called = true
		return nil
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotFound)
	}
	if called {
		t.Fatalf("fn was called for an account that does not exist")
	}
}

func TestRepositoryWithLockedAccountRollsBack(t *testing.T) {
	ctx, repo, pool := setup(t)
	source := createAccount(t, pool)
	target := createAccount(t, pool)
	fund(t, pool, source, 100)
	record := newRecord(source, target, 30)
	errAborted := errors.New("aborted after append")

	err := repo.WithLockedAccount(ctx, source, func(tx domain.JournalTx) error {
		if err := tx.Append(ctx, record); err != nil {
			return err
		}

		return errAborted
	})
	if !errors.Is(err, errAborted) {
		t.Fatalf("err = %v; want %v", err, errAborted)
	}

	requireNoRecord(t, pool, record.ID)
	requireBalance(t, pool, source, 100)

	lockCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Second)
	defer cancelFunc()

	if err := repo.WithLockedAccount(lockCtx, source, func(domain.JournalTx) error { return nil }); err != nil {
		t.Fatalf("locking the account after a rollback: %v", err)
	}
}

func TestRepositoryWithLockedAccountWaitsForLock(t *testing.T) {
	ctx, repo, pool := setup(t)
	source := createAccount(t, pool)
	target := createAccount(t, pool)
	fund(t, pool, source, 100)

	locked := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- repo.WithLockedAccount(ctx, source, func(tx domain.JournalTx) error {
			close(locked)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}

			return tx.Append(ctx, newRecord(source, target, 30))
		})
	}()

	select {
	case <-locked:
	case err := <-firstDone:
		t.Fatalf("locking account: %v", err)
	}

	var balance int64
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- repo.WithLockedAccount(ctx, source, func(tx domain.JournalTx) error {
			var err error
			balance, err = tx.Balance(ctx, source)
			return err
		})
	}()

	select {
	case err := <-secondDone:
		t.Fatalf("locked an account held by another transaction (err: %v)", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first transaction: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second transaction: %v", err)
	}
	if balance != 70 {
		t.Fatalf("balance after waiting for the lock = %d; want 70", balance)
	}
}

func TestRepositoryCrossingTransfers(t *testing.T) {
	ctx, repo, pool := setup(t)
	first := createAccount(t, pool)
	second := createAccount(t, pool)
	fund(t, pool, first, 100)
	fund(t, pool, second, 100)

	send := func(sourceID, targetID uuid.UUID, locked chan<- struct{}, otherLocked <-chan struct{}) error {
		return repo.WithLockedAccount(ctx, sourceID, func(tx domain.JournalTx) error {
			close(locked)
			select {
			case <-otherLocked:
			case <-ctx.Done():
				return ctx.Err()
			}

			return tx.Append(ctx, newRecord(sourceID, targetID, 30))
		})
	}

	firstLocked := make(chan struct{})
	secondLocked := make(chan struct{})
	errs := make(chan error, 2)
	go func() { errs <- send(first, second, firstLocked, secondLocked) }()
	go func() { errs <- send(second, first, secondLocked, firstLocked) }()

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("crossing transfer: %v", err)
		}
	}

	requireBalance(t, pool, first, 100)
	requireBalance(t, pool, second, 100)
}

func TestRepositoryConcurrentTransfers(t *testing.T) {
	const (
		funds     = 100
		amount    = 10
		transfers = 20
	)

	ctx, repo, pool := setup(t)
	source := createAccount(t, pool)
	target := createAccount(t, pool)
	fund(t, pool, source, funds)

	ledger := domain.NewLedger(repo)
	start := make(chan struct{})
	errs := make(chan error, transfers)
	for range transfers {
		go func() {
			<-start
			_, err := ledger.MakeTransaction(ctx, source, target, amount)
			errs <- err
		}()
	}
	close(start)

	succeeded := 0
	for range transfers {
		err := <-errs
		if err == nil {
			succeeded++
			continue
		}
		if !errors.Is(err, domain.ErrInsufficientFund) {
			t.Fatalf("transfer: %v", err)
		}
	}

	if succeeded != funds/amount {
		t.Fatalf("%d transfer(s) succeeded; want %d", succeeded, funds/amount)
	}
	requireBalance(t, pool, source, 0)
	requireBalance(t, pool, target, funds)
}

func TestRepositoryContextCancelled(t *testing.T) {
	ctx, repo, pool := setup(t)
	source := createAccount(t, pool)
	target := createAccount(t, pool)
	fund(t, pool, source, 100)

	t.Run("WithLockedAccount", func(t *testing.T) {
		ctx, cancelFunc := context.WithCancel(ctx)
		cancelFunc()

		called := false
		err := repo.WithLockedAccount(ctx, source, func(domain.JournalTx) error {
			called = true
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
		if called {
			t.Fatalf("fn was called with a cancelled context")
		}
	})

	t.Run("Balance", func(t *testing.T) {
		cancelled, cancelFunc := context.WithCancel(ctx)
		cancelFunc()

		err := repo.WithLockedAccount(ctx, source, func(tx domain.JournalTx) error {
			_, err := tx.Balance(cancelled, source)
			return err
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})

	t.Run("Append", func(t *testing.T) {
		cancelled, cancelFunc := context.WithCancel(ctx)
		cancelFunc()
		record := newRecord(source, target, 30)

		err := repo.WithLockedAccount(ctx, source, func(tx domain.JournalTx) error {
			return tx.Append(cancelled, record)
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}

		requireNoRecord(t, pool, record.ID)
	})
}
