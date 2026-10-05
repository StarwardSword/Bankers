package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/StarwardSword/bank/features/ledger/application"
	"github.com/StarwardSword/bank/features/ledger/domain"
	"github.com/google/uuid"
)

var errJournal = errors.New("journal is down")

func setupFake(t *testing.T) (context.Context, *application.Application, *fakeJournal) {
	t.Helper()

	journal := &fakeJournal{
		balances: map[uuid.UUID]int64{},
	}

	app, err := application.NewApplication(journal)
	if err != nil {
		t.Fatalf("creating application: %v", err)
	}

	return t.Context(), app, journal
}

func requireBalance(t *testing.T, journal *fakeJournal, accountID uuid.UUID, want int64) {
	t.Helper()

	if got := journal.balances[accountID]; got != want {
		t.Fatalf("balance of %s = %d; want %d", accountID, got, want)
	}
}

func requireNoRecords(t *testing.T, journal *fakeJournal) {
	t.Helper()

	if len(journal.records) != 0 {
		t.Fatalf("journal stored %d record(s); want none", len(journal.records))
	}
}

func TestApplicationMakeTransaction(t *testing.T) {
	ctx, app, journal := setupFake(t)
	sourceID := journal.addAccount(100)
	targetID := journal.addAccount(0)

	tid, err := app.MakeTransaction(ctx, sourceID, targetID, 30)
	if err != nil {
		t.Fatalf("could not make transaction: %v", err)
	}
	if len(journal.records) != 1 {
		t.Fatalf("journal stored %d record(s); want 1", len(journal.records))
	}

	got := journal.records[0]
	want := domain.TransactionRecord{
		ID:       tid,
		SourceID: sourceID,
		TargetID: targetID,
		Amount:   30,
		Created:  got.Created,
	}
	if got != want {
		t.Fatalf("journal record = %+v; want %+v", got, want)
	}

	requireBalance(t, journal, sourceID, 70)
	requireBalance(t, journal, targetID, 30)
}

func TestApplicationMakeTransactionInsufficientFunds(t *testing.T) {
	ctx, app, journal := setupFake(t)
	sourceID := journal.addAccount(29)
	targetID := journal.addAccount(0)

	tid, err := app.MakeTransaction(ctx, sourceID, targetID, 30)
	if tid != uuid.Nil || !errors.Is(err, domain.ErrInsufficientFund) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, domain.ErrInsufficientFund)
	}

	requireNoRecords(t, journal)
	requireBalance(t, journal, sourceID, 29)
}

func TestApplicationMakeTransactionInvalidAmount(t *testing.T) {
	ctx, app, journal := setupFake(t)
	sourceID := journal.addAccount(100)
	targetID := journal.addAccount(0)

	tid, err := app.MakeTransaction(ctx, sourceID, targetID, 0)
	if tid != uuid.Nil || !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, domain.ErrInvalidAmount)
	}
	if len(journal.locked) != 0 {
		t.Fatalf("journal locked %v; want no calls", journal.locked)
	}
}

func TestApplicationMakeTransactionUnknownSource(t *testing.T) {
	ctx, app, journal := setupFake(t)
	targetID := journal.addAccount(0)

	tid, err := app.MakeTransaction(ctx, uuid.New(), targetID, 30)
	if tid != uuid.Nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, domain.ErrNotFound)
	}

	requireNoRecords(t, journal)
}

func TestApplicationMakeTransactionUnknownTarget(t *testing.T) {
	ctx, app, journal := setupFake(t)
	sourceID := journal.addAccount(100)

	tid, err := app.MakeTransaction(ctx, sourceID, uuid.New(), 30)
	if tid != uuid.Nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, domain.ErrNotFound)
	}

	requireNoRecords(t, journal)
	requireBalance(t, journal, sourceID, 100)
}

func TestApplicationMakeTransactionJournalError(t *testing.T) {
	ctx, app, journal := setupFake(t)
	sourceID := journal.addAccount(100)
	targetID := journal.addAccount(0)
	journal.err = errJournal

	tid, err := app.MakeTransaction(ctx, sourceID, targetID, 30)
	if tid != uuid.Nil || !errors.Is(err, errJournal) {
		t.Fatalf("MakeTransaction = %s, %v; want %s, %v", tid, err, uuid.Nil, errJournal)
	}

	requireNoRecords(t, journal)
	requireBalance(t, journal, sourceID, 100)
}

type fakeJournal struct {
	balances map[uuid.UUID]int64
	records  []domain.TransactionRecord
	locked   []uuid.UUID
	err      error
}

func (j *fakeJournal) addAccount(balance int64) uuid.UUID {
	id := uuid.New()
	j.balances[id] = balance

	return id
}

func (j *fakeJournal) WithLockedAccount(ctx context.Context, accountID uuid.UUID, fn func(tx domain.JournalTx) error) error {
	j.locked = append(j.locked, accountID)

	if j.err != nil {
		return j.err
	}
	if _, ok := j.balances[accountID]; !ok {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, accountID)
	}

	tx := &fakeJournalTx{journal: j}
	if err := fn(tx); err != nil {
		return err
	}

	for _, record := range tx.appended {
		j.balances[record.SourceID] -= record.Amount
		j.balances[record.TargetID] += record.Amount
		j.records = append(j.records, record)
	}

	return nil
}

type fakeJournalTx struct {
	journal  *fakeJournal
	appended []domain.TransactionRecord
}

func (tx *fakeJournalTx) Balance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	return tx.journal.balances[accountID], nil
}

func (tx *fakeJournalTx) Append(ctx context.Context, r domain.TransactionRecord) error {
	if _, ok := tx.journal.balances[r.TargetID]; !ok {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, r.TargetID)
	}

	tx.appended = append(tx.appended, r)

	return nil
}
