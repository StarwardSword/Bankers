package domain

import (
	"context"

	"github.com/google/uuid"
)

type Ledger interface {
	MakeTransaction(ctx context.Context, sourceID, targetID uuid.UUID, amount int64) (uuid.UUID, error)
}
