package application

import (
	"context"

	"github.com/StarwardSword/bank/features/ledger/domain"
	"github.com/google/uuid"
)

type Application struct {
	ledger *domain.Ledger
}

func NewApplication(repo domain.Journal) (*Application, error) {
	l := domain.NewLedger(repo)

	return &Application{
		ledger: l,
	}, nil
}

// MakeTransaction может вернуть ошибки:
//
// Домен:
//   - [domain.ErrTargetIsSelf] — sourceID == targetID
//   - [domain.ErrAccountIsNil] — sourceID или targetID равен uuid.Nil
//   - [domain.ErrInvalidAmount] — amount <= 0
//   - [domain.ErrInsufficientFund] — на счёте sourceID меньше amount
//   - [domain.ErrNotFound] — счёт sourceID или targetID не найден или удалён
func (a *Application) MakeTransaction(ctx context.Context, sourceID uuid.UUID, targetID uuid.UUID, amount int64) (uuid.UUID, error) {
	return a.ledger.MakeTransaction(ctx, sourceID, targetID, amount)
}
