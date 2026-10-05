package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrInsufficientFund = errors.New("not enough currency")
	ErrInvalidAmount    = errors.New("amount must be positive")
	ErrTargetIsNil      = errors.New("target account is nil")
	ErrTargetIsSelf     = errors.New("can not transfer to self")
	ErrNotHolder        = errors.New("account does not belong to the user")
)

type Account struct {
	ID       uuid.UUID
	HolderID uuid.UUID

	balance int64
}

func NewAccount(holderID uuid.UUID, balance int64) *Account {
	res := &Account{
		ID:       uuid.New(),
		HolderID: holderID,
		balance:  balance,
	}

	return res
}

func (a *Account) Balance() int64 {
	return a.balance
}

func (a *Account) Transfer(ctx context.Context, ledger Ledger, initiatorID uuid.UUID, target *Account, amount int64) (uuid.UUID, error) {
	switch {
	case amount <= 0:
		return uuid.Nil, ErrInvalidAmount
	case target == nil:
		return uuid.Nil, ErrTargetIsNil
	case target.ID == a.ID:
		return uuid.Nil, ErrTargetIsSelf
	case initiatorID != a.HolderID:
		return uuid.Nil, fmt.Errorf("%s requested transfer from %s to %s: %w", initiatorID.String(), a.ID.String(), target.ID.String(), ErrNotHolder)
	case a.balance < amount:
		return uuid.Nil, ErrInsufficientFund
	}

	tid, err := ledger.MakeTransaction(ctx, a.ID, target.ID, amount)
	if err != nil {
		return tid, fmt.Errorf("transaction error: %w", err)
	}

	return tid, nil
}
