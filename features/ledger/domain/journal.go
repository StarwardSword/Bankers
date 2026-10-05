package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type JournalTx interface {
	Balance(ctx context.Context, accountID uuid.UUID) (int64, error)
	Append(ctx context.Context, r TransactionRecord) error
}

type Journal interface {
	WithLockedAccount(ctx context.Context, accountID uuid.UUID, fn func(tx JournalTx) error) error
}

type TransactionRecord struct {
	ID       uuid.UUID
	SourceID uuid.UUID
	TargetID uuid.UUID
	Amount   int64
	Created  time.Time
}
