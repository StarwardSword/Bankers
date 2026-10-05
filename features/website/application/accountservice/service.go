package accountservice

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrTargetNotFound   = errors.New("not found")
	ErrInsufficientFund = errors.New("insufficient fund")
	ErrPermisionDenied  = errors.New("permision denied")
	ErrWrongTarget      = errors.New("wrong target")
	ErrInvalidAmount    = errors.New("amount less or equals to zero")
)

type Service interface {
	CreateAccount(ctx context.Context, userId uuid.UUID) (uuid.UUID, error)
	GetBalance(ctx context.Context, userId uuid.UUID, accountId uuid.UUID) (int64, error)
	Transfer(ctx context.Context, userId uuid.UUID, sourceId uuid.UUID, targetId uuid.UUID, amount int64) (bool, error)
	GetUserAccounts(ctx context.Context, userId uuid.UUID) ([]Account, error)
	GetById(ctx context.Context, userId uuid.UUID, accId uuid.UUID) (Account, error)
}

type Account struct {
	ID       uuid.UUID
	Currency int64
}
