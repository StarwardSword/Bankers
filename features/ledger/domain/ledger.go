package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInsufficientFund = errors.New("insufficient funds")
	ErrTargetIsSelf     = errors.New("can not transfer to self")
	ErrAccountIsNil     = errors.New("account is nil")
	ErrInvalidAmount    = errors.New("amount must be positive")
	ErrNotFound         = errors.New("account not found")
)

type Ledger struct {
	journal Journal
}

func NewLedger(j Journal) *Ledger {
	return &Ledger{
		journal: j,
	}
}

func (l *Ledger) MakeTransaction(ctx context.Context, sourceID, targetID uuid.UUID, amount int64) (uuid.UUID, error) {
	switch {
	case sourceID == targetID:
		return uuid.Nil, ErrTargetIsSelf
	case targetID == uuid.Nil:
		return uuid.Nil, fmt.Errorf("target account: %w", ErrAccountIsNil)
	case sourceID == uuid.Nil:
		return uuid.Nil, fmt.Errorf("source account: %w", ErrAccountIsNil)
	case amount <= 0:
		return uuid.Nil, ErrInvalidAmount
	}

	record := TransactionRecord{
		ID:       uuid.New(),
		SourceID: sourceID,
		TargetID: targetID,
		Amount:   amount,
		Created:  time.Now().UTC(),
	}

	err := l.journal.WithLockedAccount(ctx, sourceID, func(tx JournalTx) error {
		balance, err := tx.Balance(ctx, sourceID)
		if err != nil {
			return fmt.Errorf("requesting balance: %w", err)
		}

		if balance < amount {
			return ErrInsufficientFund
		}

		return tx.Append(ctx, record)
	})

	if err != nil {
		return uuid.Nil, fmt.Errorf("executing transaction: %w", err)
	}

	return record.ID, nil
}
