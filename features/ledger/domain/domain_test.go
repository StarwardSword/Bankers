package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StarwardSword/bank/features/ledger/domain"
	"github.com/google/uuid"
)

func setup(t *testing.T, sourceBalance int64) (context.Context, *fakeJournal, *domain.Ledger, uuid.UUID, uuid.UUID) {
	t.Helper()

	ctx := t.Context()
	sourceID := uuid.New()
	targetID := uuid.New()

	journal := &fakeJournal{
		balances: map[uuid.UUID]int64{
			sourceID: sourceBalance,
		},
	}

	return ctx, journal, domain.NewLedger(journal), sourceID, targetID
}

func requireNoTransaction(t *testing.T, journal *fakeJournal, tid uuid.UUID) {
	t.Helper()

	if tid != uuid.Nil {
		t.Fatalf("transaction id = %s; want uuid.Nil", tid)
	}
	if len(journal.records) != 0 {
		t.Fatalf("journal stored %d record(s); want none", len(journal.records))
	}
}

func requireJournalNotCalled(t *testing.T, journal *fakeJournal) {
	t.Helper()

	if len(journal.locked) != 0 {
		t.Fatalf("journal locked %v; want no calls", journal.locked)
	}
}

func TestLedgerMakeTransaction(t *testing.T) {
	const operationalAmount = 10
	ctx, journal, ledger, sourceID, targetID := setup(t, operationalAmount)

	before := time.Now()
	tid, err := ledger.MakeTransaction(ctx, sourceID, targetID, operationalAmount)
	after := time.Now()
	if err != nil {
		t.Fatalf("could not make transaction: %v", err)
	}

	if len(journal.locked) != 1 || journal.locked[0] != sourceID {
		t.Fatalf("locked accounts = %v; want [%s]", journal.locked, sourceID)
	}
	if len(journal.records) != 1 {
		t.Fatalf("journal stored %d record(s); want 1", len(journal.records))
	}

	got := journal.records[0]
	if got.Created.Before(before) || got.Created.After(after) {
		t.Fatalf("created = %s; want between %s and %s", got.Created, before, after)
	}

	want := domain.TransactionRecord{
		ID:       tid,
		SourceID: sourceID,
		TargetID: targetID,
		Amount:   operationalAmount,
		Created:  got.Created,
	}
	if got != want {
		t.Fatalf("journal record = %+v; want %+v", got, want)
	}
}

func TestLedgerMakeTransactionInsufficientFunds(t *testing.T) {
	ctx, journal, ledger, sourceID, targetID := setup(t, 9)
	journal.balances[targetID] = 100

	tid, err := ledger.MakeTransaction(ctx, sourceID, targetID, 10)
	if !errors.Is(err, domain.ErrInsufficientFund) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInsufficientFund)
	}

	requireNoTransaction(t, journal, tid)
}

func TestLedgerMakeTransactionZeroAmount(t *testing.T) {
	ctx, journal, ledger, sourceID, targetID := setup(t, 10)

	tid, err := ledger.MakeTransaction(ctx, sourceID, targetID, 0)
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInvalidAmount)
	}

	requireNoTransaction(t, journal, tid)
	requireJournalNotCalled(t, journal)
}

func TestLedgerMakeTransactionNegativeAmount(t *testing.T) {
	ctx, journal, ledger, sourceID, targetID := setup(t, 10)

	tid, err := ledger.MakeTransaction(ctx, sourceID, targetID, -1)
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInvalidAmount)
	}

	requireNoTransaction(t, journal, tid)
	requireJournalNotCalled(t, journal)
}

func TestLedgerMakeTransactionToSelf(t *testing.T) {
	ctx, journal, ledger, sourceID, _ := setup(t, 10)

	tid, err := ledger.MakeTransaction(ctx, sourceID, sourceID, 10)
	if !errors.Is(err, domain.ErrTargetIsSelf) {
		t.Fatalf("err = %v; want %v", err, domain.ErrTargetIsSelf)
	}

	requireNoTransaction(t, journal, tid)
	requireJournalNotCalled(t, journal)
}

func TestLedgerMakeTransactionNilSource(t *testing.T) {
	ctx, journal, ledger, _, targetID := setup(t, 10)

	tid, err := ledger.MakeTransaction(ctx, uuid.Nil, targetID, 10)
	if !errors.Is(err, domain.ErrAccountIsNil) {
		t.Fatalf("err = %v; want %v", err, domain.ErrAccountIsNil)
	}

	requireNoTransaction(t, journal, tid)
	requireJournalNotCalled(t, journal)
}

func TestLedgerMakeTransactionNilTarget(t *testing.T) {
	ctx, journal, ledger, sourceID, _ := setup(t, 10)

	tid, err := ledger.MakeTransaction(ctx, sourceID, uuid.Nil, 10)
	if !errors.Is(err, domain.ErrAccountIsNil) {
		t.Fatalf("err = %v; want %v", err, domain.ErrAccountIsNil)
	}

	requireNoTransaction(t, journal, tid)
	requireJournalNotCalled(t, journal)
}

func TestLedgerMakeTransactionSourceNotFound(t *testing.T) {
	ctx, journal, ledger, sourceID, targetID := setup(t, 10)
	journal.lockErr = domain.ErrNotFound

	tid, err := ledger.MakeTransaction(ctx, sourceID, targetID, 10)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotFound)
	}

	requireNoTransaction(t, journal, tid)
}

func TestLedgerMakeTransactionBalanceError(t *testing.T) {
	ctx, journal, ledger, sourceID, targetID := setup(t, 10)
	errBalance := errors.New("balance is unavailable")
	journal.balanceErr = errBalance

	tid, err := ledger.MakeTransaction(ctx, sourceID, targetID, 10)
	if !errors.Is(err, errBalance) {
		t.Fatalf("err = %v; want %v", err, errBalance)
	}

	requireNoTransaction(t, journal, tid)
}

func TestLedgerMakeTransactionAppendError(t *testing.T) {
	ctx, journal, ledger, sourceID, targetID := setup(t, 10)
	journal.appendErr = domain.ErrNotFound

	tid, err := ledger.MakeTransaction(ctx, sourceID, targetID, 10)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotFound)
	}

	requireNoTransaction(t, journal, tid)
}

func TestLedgerMakeTransactionContextCancelled(t *testing.T) {
	ctx, journal, ledger, sourceID, targetID := setup(t, 10)
	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	tid, err := ledger.MakeTransaction(ctx, sourceID, targetID, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v; want %v", err, context.Canceled)
	}

	requireNoTransaction(t, journal, tid)
}

type fakeJournal struct {
	balances map[uuid.UUID]int64
	records  []domain.TransactionRecord
	locked   []uuid.UUID

	lockErr    error
	balanceErr error
	appendErr  error
}

func (j *fakeJournal) WithLockedAccount(ctx context.Context, accountID uuid.UUID, fn func(tx domain.JournalTx) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	j.locked = append(j.locked, accountID)
	if j.lockErr != nil {
		return j.lockErr
	}

	tx := &fakeJournalTx{journal: j}
	if err := fn(tx); err != nil {
		return err
	}

	j.records = append(j.records, tx.appended...)

	return nil
}

type fakeJournalTx struct {
	journal  *fakeJournal
	appended []domain.TransactionRecord
}

func (tx *fakeJournalTx) Balance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	if tx.journal.balanceErr != nil {
		return 0, tx.journal.balanceErr
	}

	return tx.journal.balances[accountID], nil
}

func (tx *fakeJournalTx) Append(ctx context.Context, r domain.TransactionRecord) error {
	if tx.journal.appendErr != nil {
		return tx.journal.appendErr
	}

	tx.appended = append(tx.appended, r)

	return nil
}
