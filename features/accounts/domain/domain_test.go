package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/StarwardSword/bank/features/accounts/domain"
	"github.com/google/uuid"
)

func setup(t *testing.T, sourceBalance int64) (context.Context, *fakeLedger, *domain.Account, *domain.Account) {
	t.Helper()

	ctx := t.Context()
	ledger := &fakeLedger{}

	sourceAccount := domain.NewAccount(uuid.New(), sourceBalance)
	targetAccount := domain.NewAccount(uuid.New(), 0)

	return ctx, ledger, sourceAccount, targetAccount
}

func requireNoTransaction(t *testing.T, ledger *fakeLedger, tid uuid.UUID) {
	t.Helper()

	if tid != uuid.Nil {
		t.Fatalf("transaction id = %s; want uuid.Nil", tid)
	}
	if len(ledger.transactions) != 0 {
		t.Fatalf("ledger made %d transaction(s); want none", len(ledger.transactions))
	}
}

func TestNewAccount(t *testing.T) {
	holderID := uuid.New()

	account := domain.NewAccount(holderID, 42)

	if account.ID == uuid.Nil {
		t.Fatalf("account id is not set")
	}
	if account.HolderID != holderID {
		t.Fatalf("holder id = %s; want %s", account.HolderID, holderID)
	}
	if account.Balance() != 42 {
		t.Fatalf("balance = %d; want 42", account.Balance())
	}
}

func TestAccountTransferring(t *testing.T) {
	const operationalAmount = 10
	ctx, ledger, sourceAccount, targetAccount := setup(t, operationalAmount)

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, targetAccount, operationalAmount)
	if err != nil {
		t.Fatalf("could not transfer: %s", err.Error())
	}

	want := fakeTransaction{
		ID:       tid,
		SourceID: sourceAccount.ID,
		TargetID: targetAccount.ID,
		Amount:   operationalAmount,
	}

	if len(ledger.transactions) != 1 || ledger.transactions[0] != want {
		t.Fatalf("ledger transactions = %+v; want [%+v]", ledger.transactions, want)
	}
}

func TestAccountTransferringInsufficientFunds(t *testing.T) {
	ctx, ledger, sourceAccount, targetAccount := setup(t, 9)

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, targetAccount, 10)
	if !errors.Is(err, domain.ErrInsufficientFund) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInsufficientFund)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringZeroAmount(t *testing.T) {
	ctx, ledger, sourceAccount, targetAccount := setup(t, 10)

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, targetAccount, 0)
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInvalidAmount)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringNegativeAmount(t *testing.T) {
	ctx, ledger, sourceAccount, targetAccount := setup(t, 10)

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, targetAccount, -1)
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInvalidAmount)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringNotHolder(t *testing.T) {
	ctx, ledger, sourceAccount, targetAccount := setup(t, 10)

	tid, err := sourceAccount.Transfer(ctx, ledger, uuid.New(), targetAccount, 10)
	if !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotHolder)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringNotHolderHidesBalance(t *testing.T) {
	ctx, ledger, sourceAccount, targetAccount := setup(t, 0)

	tid, err := sourceAccount.Transfer(ctx, ledger, uuid.New(), targetAccount, 10)
	if !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotHolder)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringNotHolderToNilTarget(t *testing.T) {
	ctx, ledger, sourceAccount, _ := setup(t, 10)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("transfer panicked: %v", r)
		}
	}()

	tid, err := sourceAccount.Transfer(ctx, ledger, uuid.New(), nil, 10)
	if !errors.Is(err, domain.ErrNotHolder) && !errors.Is(err, domain.ErrTargetIsNil) {
		t.Fatalf("err = %v; want %v or %v", err, domain.ErrNotHolder, domain.ErrTargetIsNil)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringToNilTarget(t *testing.T) {
	ctx, ledger, sourceAccount, _ := setup(t, 10)

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, nil, 10)
	if !errors.Is(err, domain.ErrTargetIsNil) {
		t.Fatalf("err = %v; want %v", err, domain.ErrTargetIsNil)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringToSelf(t *testing.T) {
	ctx, ledger, sourceAccount, _ := setup(t, 10)

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, sourceAccount, 10)
	if !errors.Is(err, domain.ErrTargetIsSelf) {
		t.Fatalf("err = %v; want %v", err, domain.ErrTargetIsSelf)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringValidatesBeforeCheckingBalance(t *testing.T) {
	ctx, ledger, sourceAccount, _ := setup(t, 0)

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, nil, 10)
	if !errors.Is(err, domain.ErrTargetIsNil) {
		t.Fatalf("err = %v; want %v", err, domain.ErrTargetIsNil)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringRejectedByLedger(t *testing.T) {
	ctx, ledger, sourceAccount, targetAccount := setup(t, 10)
	ledger.transactionErr = domain.ErrInsufficientFund

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, targetAccount, 10)
	if !errors.Is(err, domain.ErrInsufficientFund) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInsufficientFund)
	}

	requireNoTransaction(t, ledger, tid)
}

func TestAccountTransferringContextCancelled(t *testing.T) {
	ctx, ledger, sourceAccount, targetAccount := setup(t, 10)
	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	tid, err := sourceAccount.Transfer(ctx, ledger, sourceAccount.HolderID, targetAccount, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v; want %v", err, context.Canceled)
	}

	requireNoTransaction(t, ledger, tid)
}

type fakeTransaction struct {
	ID       uuid.UUID
	SourceID uuid.UUID
	TargetID uuid.UUID
	Amount   int64
}

type fakeLedger struct {
	transactions   []fakeTransaction
	transactionErr error
}

func (l *fakeLedger) MakeTransaction(ctx context.Context, sourceID, targetID uuid.UUID, amount int64) (uuid.UUID, error) {
	if ctx.Err() != nil {
		return uuid.Nil, ctx.Err()
	}

	if l.transactionErr != nil {
		return uuid.Nil, l.transactionErr
	}

	transaction := fakeTransaction{
		ID:       uuid.New(),
		SourceID: sourceID,
		TargetID: targetID,
		Amount:   amount,
	}

	l.transactions = append(l.transactions, transaction)

	return transaction.ID, nil
}
