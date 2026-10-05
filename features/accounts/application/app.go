package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/StarwardSword/bank/features/accounts/domain"
	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("can not find")
)

type Application struct {
	ledger domain.Ledger
	repo   Repository
}

func NewApplication(repo Repository, ledger domain.Ledger) (*Application, error) {
	return &Application{
		ledger: ledger,
		repo:   repo,
	}, nil
}

// CreateAccount может вернуть ошибки:
//
// Приложение:
//   - [ErrNotFound] — пользователь userId не найден или удалён
func (a *Application) CreateAccount(ctx context.Context, userId uuid.UUID) (uuid.UUID, error) {
	account := domain.NewAccount(userId, 0)
	err := a.repo.SaveAccount(ctx, account)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("saving account: %w", err)
	}

	return account.ID, nil
}

// GetBalance может вернуть ошибки:
//
// Приложение:
//   - [ErrNotFound] — счёт accountId не найден или удалён
//
// Домен:
//   - [domain.ErrNotHolder] — счёт принадлежит не userId
func (a *Application) GetBalance(ctx context.Context, userId uuid.UUID, accountId uuid.UUID) (int64, error) {
	account, err := a.repo.GetById(ctx, accountId)
	if err != nil {
		return 0, fmt.Errorf("requesting account: %w", err)
	}
	if account.HolderID != userId {
		return 0, fmt.Errorf("accessing account: %w", domain.ErrNotHolder)
	}

	return account.Balance(), nil
}

// Transfer может вернуть ошибки:
//
// Приложение:
//   - [ErrNotFound] — счёт sourceId или targetId не найден или удалён
//
// Домен:
//   - [domain.ErrInvalidAmount] — amount <= 0
//   - [domain.ErrTargetIsSelf] — sourceId == targetId
//   - [domain.ErrNotHolder] — счёт sourceId принадлежит не requesterId
//   - [domain.ErrInsufficientFund] — на счёте sourceId меньше amount
func (a *Application) Transfer(ctx context.Context, requesterId uuid.UUID, sourceId uuid.UUID, targetId uuid.UUID, amount int64) (bool, error) {
	// TODO: Возвращает только err
	source, err := a.repo.GetById(ctx, sourceId)
	if err != nil {
		return false, fmt.Errorf("getting source: %w", err)
	}

	target, err := a.repo.GetById(ctx, targetId)
	if err != nil {
		return false, fmt.Errorf("getting target: %w", err)
	}

	_, err = source.Transfer(ctx, a.ledger, requesterId, &target, amount)
	if err != nil {
		return false, fmt.Errorf("transferring: %w", err)
	}

	return true, nil
}

// GetUserAccounts может вернуть ошибки:
//
// Приложение:
//   - [ErrNotFound] — пользователь userId не найден или удалён
func (a *Application) GetUserAccounts(ctx context.Context, userId uuid.UUID) ([]domain.Account, error) {
	if exist, err := a.repo.UserExist(ctx, userId); !exist || err != nil {
		if err != nil {
			return []domain.Account{}, fmt.Errorf("could not find user: %w", err)
		}
		if !exist {
			return []domain.Account{}, fmt.Errorf("%w: %s", ErrNotFound, userId.String())
		}
	}

	return a.repo.GetUserAccounts(ctx, userId)
}

// GetById может вернуть ошибки:
//
// Приложение:
//   - [ErrNotFound] — счёт accId не найден или удалён (из [Repository])
//
// Домен:
//   - [domain.ErrNotHolder] — счёт принадлежит не userId
func (a *Application) GetById(ctx context.Context, userId uuid.UUID, accId uuid.UUID) (domain.Account, error) {
	acc, err := a.repo.GetById(ctx, accId)
	if err != nil {
		return domain.Account{}, fmt.Errorf("can not get account: %w", err)
	}

	if acc.HolderID != userId {
		return domain.Account{}, fmt.Errorf("%w: %s", domain.ErrNotHolder, userId.String())
	}

	return acc, nil
}
