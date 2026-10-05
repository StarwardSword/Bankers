package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/StarwardSword/bank/features/website/adapters/htmx/pkg/renderer"
	"github.com/StarwardSword/bank/features/website/application/accountservice"
	"github.com/a-h/templ"
	"github.com/google/uuid"
)

func handleIndex(userId uuid.UUID) renderer.Handler[accountservice.Service, any] {
	return func(ctx context.Context, service accountservice.Service, q any) (templ.Component, error) {
		accs, err := service.GetUserAccounts(ctx, userId)
		if err != nil {
			return nil, fmt.Errorf("get user accounts: %w", err)
		}

		vm := account{}

		if len(accs) == 0 {
			accId, err := service.CreateAccount(ctx, userId)
			if err != nil {
				return nil, fmt.Errorf("creating account: %w", err)
			}

			vm = account{
				id:       accId,
				currency: 0,
			}
		} else {
			vm = account{
				id:       accs[0].ID,
				currency: accs[0].Currency,
			}
		}

		return index(vm), nil
	}
}

type transferIndexQuery struct {
	AccountId string `form:"uid"`
}

func handleIndexTransfer(userId uuid.UUID) renderer.Handler[accountservice.Service, transferIndexQuery] {
	return func(ctx context.Context, service accountservice.Service, query transferIndexQuery) (templ.Component, error) {
		accountId, err := uuid.Parse(query.AccountId)
		if err != nil {
			return nil, fmt.Errorf("parsing uuid: %w", err)
		}

		// Check if user can use specified account
		_, err = service.GetById(ctx, userId, accountId)
		if err != nil {
			return nil, fmt.Errorf("getting accout: %w", err)
		}

		return transferBlock(accountId), nil
	}
}

type transferCommand struct {
	Source string `form:"source"`
	Target string `form:"target"`
	Amount int64  `form:"amount"`
}

func handleTransfer(userId uuid.UUID) renderer.Handler[accountservice.Service, *transferCommand] {
	return func(ctx context.Context, s accountservice.Service, tc *transferCommand) (templ.Component, error) {
		sourceId, err := uuid.Parse(tc.Source)
		if err != nil {
			return nil, fmt.Errorf("parsing uuid: %w", err)
		}

		sourceAccount, err := s.GetById(ctx, userId, sourceId)
		if err != nil {
			return nil, fmt.Errorf("getting accout: %w", err)
		}

		acc := account{
			id:       sourceId,
			currency: sourceAccount.Currency,
		}

		succ, err := s.Transfer(ctx, userId, sourceId, uuid.MustParse(tc.Target), tc.Amount)
		if errors.Is(err, accountservice.ErrTargetNotFound) {
			return transferResult(acc, false, "Счет получателя не найден"), nil
		}
		if errors.Is(err, accountservice.ErrInsufficientFund) {
			return transferResult(acc, false, "Недостаточно средств для перевода"), nil
		}
		if err != nil {
			return transferResult(acc, false, err.Error()), nil
		}
		if !succ {
			return transferResult(acc, false, "Если вы видите это, то что-то очень сильно сломано"), nil
		}

		balance, err := s.GetBalance(ctx, userId, sourceId)
		if err != nil {
			return nil, fmt.Errorf("request balance: %w", err)
		}

		acc.currency = balance

		return transferResult(acc, true, ""), nil
	}
}
