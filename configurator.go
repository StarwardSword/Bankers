package main

import (
	"context"
	"errors"
	"fmt"

	accountApp "github.com/StarwardSword/bank/features/accounts/application"
	accountDomain "github.com/StarwardSword/bank/features/accounts/domain"
	ledgerApp "github.com/StarwardSword/bank/features/ledger/application"
	ledgerDomain "github.com/StarwardSword/bank/features/ledger/domain"
	userApp "github.com/StarwardSword/bank/features/user_service/application"
	userDomain "github.com/StarwardSword/bank/features/user_service/domain"
	webAccService "github.com/StarwardSword/bank/features/website/application/accountservice"
	webAuthService "github.com/StarwardSword/bank/features/website/application/authservice"
	webUserService "github.com/StarwardSword/bank/features/website/application/userservice"
	"github.com/StarwardSword/bank/features/website/infrastucture/localauthservice"
	"github.com/google/uuid"
)

type htmxAccountApp struct {
	accounts *accountApp.Application
}

func (a *htmxAccountApp) CreateAccount(ctx context.Context, userId uuid.UUID) (uuid.UUID, error) {
	return a.accounts.CreateAccount(ctx, userId)
}

func (a *htmxAccountApp) Transfer(ctx context.Context, userId uuid.UUID, sourceId uuid.UUID, targetId uuid.UUID, amount int64) (bool, error) {
	res, err := a.accounts.Transfer(ctx, userId, sourceId, targetId, amount)

	switch {
	case errors.Is(err, accountApp.ErrNotFound):
		return false, fmt.Errorf("%w: %s", webAccService.ErrTargetNotFound, err.Error())
	case errors.Is(err, accountDomain.ErrInvalidAmount):
		return false, fmt.Errorf("%w: %s", webAccService.ErrInvalidAmount, err.Error())
	case errors.Is(err, accountDomain.ErrTargetIsSelf) || errors.Is(err, accountDomain.ErrTargetIsNil):
		return false, fmt.Errorf("%w: %s", webAccService.ErrWrongTarget, err.Error())
	case errors.Is(err, accountDomain.ErrInsufficientFund):
		return false, fmt.Errorf("%w: %s", webAccService.ErrInsufficientFund, err.Error())
	case err != nil:
		return false, fmt.Errorf("transferring: %w", err)
	}

	return res, nil
}

func (a *htmxAccountApp) GetBalance(ctx context.Context, userId uuid.UUID, accountId uuid.UUID) (int64, error) {
	res, err := a.accounts.GetBalance(ctx, userId, accountId)

	switch {
	case errors.Is(err, accountApp.ErrNotFound):
		return 0, fmt.Errorf("%w: %s", webAccService.ErrTargetNotFound, err.Error())
	case errors.Is(err, accountDomain.ErrNotHolder):
		return 0, fmt.Errorf("%w: %s", webAccService.ErrPermisionDenied, err.Error())
	case err != nil:
		return 0, fmt.Errorf("getting balance: %w", err)
	}

	return res, nil
}

func (a htmxAccountApp) GetById(ctx context.Context, userId uuid.UUID, accId uuid.UUID) (webAccService.Account, error) {
	acc, err := a.accounts.GetById(ctx, userId, accId)

	na := webAccService.Account{}
	switch {
	case errors.Is(err, accountApp.ErrNotFound):
		return na, fmt.Errorf("%w: %s", webAccService.ErrTargetNotFound, err.Error())
	case errors.Is(err, accountDomain.ErrNotHolder):
		return na, fmt.Errorf("%w: %s", webAccService.ErrPermisionDenied, err.Error())
	case err != nil:
		return na, fmt.Errorf("getting account: %w", err)
	}

	return webAccService.Account{
		ID:       acc.ID,
		Currency: acc.Balance(),
	}, nil
}

func (a htmxAccountApp) GetUserAccounts(ctx context.Context, userId uuid.UUID) ([]webAccService.Account, error) {
	dat, err := a.accounts.GetUserAccounts(ctx, userId)
	switch {
	case errors.Is(err, accountApp.ErrNotFound):
		return nil, fmt.Errorf("%w: %s", webAccService.ErrTargetNotFound, err.Error())
	case err != nil:
		return nil, fmt.Errorf("getting accounts: %w", err)
	}

	res := make([]webAccService.Account, len(dat))
	for i, v := range dat {
		res[i] = webAccService.Account{
			ID:       v.ID,
			Currency: v.Balance(),
		}
	}

	return res, nil
}

type htmxUserApp struct {
	app *userApp.Application
}

func (a *htmxUserApp) CreateUser(ctx context.Context, name string, password string) (uuid.UUID, error) {
	res, err := a.app.CreateUser(ctx, name, password)
	switch {
	case errors.Is(err, userApp.ErrAlreadyExists):
		return uuid.Nil, fmt.Errorf("%w: %s", webUserService.ErrAlreadyExists, err.Error())
	case errors.Is(err, userDomain.ErrInvalidArgument):
		return uuid.Nil, fmt.Errorf("%w: %s", webUserService.ErrInvalidArgument, err.Error())
	case err != nil:
		return uuid.Nil, fmt.Errorf("creating user: %w", err)
	}

	return res, err
}

func (a *htmxUserApp) RemoveUser(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (a *htmxUserApp) ChangeRole(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID, role userDomain.Role) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (a *htmxUserApp) ChangePassword(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID, oldPassword string, newPassword string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

type htmxAuthApp struct {
	auth *localauthservice.Service
}

func (a *htmxAuthApp) Auth(ctx context.Context, username string, password string) (string, error) {
	res, err := a.auth.Auth(ctx, username, password)
	switch {
	case errors.Is(err, userApp.ErrNotFound):
		return "", fmt.Errorf("%w: %s", webAuthService.ErrNotFound, err.Error())
	case errors.Is(err, userApp.ErrWrongPassword):
		return "", fmt.Errorf("%w: %s", webAuthService.ErrWrongPassword, err.Error())
	case err != nil:
		return "", fmt.Errorf("authenticating: %w", err)
	}

	return res, nil
}

type accountLedgerApp struct {
	app *ledgerApp.Application
}

func (a *accountLedgerApp) MakeTransaction(ctx context.Context, sourceId uuid.UUID, targetId uuid.UUID, amount int64) (transactionId uuid.UUID, err error) {
	res, err := a.app.MakeTransaction(ctx, sourceId, targetId, amount)

	switch {
	case errors.Is(err, ledgerDomain.ErrTargetIsSelf):
		return uuid.Nil, fmt.Errorf("%w: %s", accountDomain.ErrTargetIsSelf, err.Error())
	case errors.Is(err, ledgerDomain.ErrAccountIsNil):
		return uuid.Nil, fmt.Errorf("%w: %s", accountDomain.ErrTargetIsNil, err.Error())
	case errors.Is(err, ledgerDomain.ErrInvalidAmount):
		return uuid.Nil, fmt.Errorf("%w: %s", accountDomain.ErrInvalidAmount, err.Error())
	case errors.Is(err, ledgerDomain.ErrInsufficientFund):
		return uuid.Nil, fmt.Errorf("%w: %s", accountDomain.ErrInsufficientFund, err.Error())
	case errors.Is(err, ledgerDomain.ErrNotFound):
		return uuid.Nil, fmt.Errorf("%w: %s", accountApp.ErrNotFound, err.Error())
	case err != nil:
		return uuid.Nil, fmt.Errorf("making transaction: %w", err)
	}

	return res, nil
}
