package application_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/StarwardSword/bank/features/accounts/application"
	"github.com/StarwardSword/bank/features/accounts/domain"
	"github.com/google/uuid"
)

var (
	errRepository = errors.New("repository is down")
	errLedger     = errors.New("ledger is down")
)

func setupFake(t *testing.T) (context.Context, *application.Application, *fakeRepository, *fakeLedger) {
	t.Helper()

	repo := &fakeRepository{
		users:    map[uuid.UUID]bool{},
		accounts: map[uuid.UUID]uuid.UUID{},
		balances: map[uuid.UUID]int64{},
	}
	ledger := &fakeLedger{repo: repo}

	app, err := application.NewApplication(repo, ledger)
	if err != nil {
		t.Fatalf("creating application: %v", err)
	}

	return t.Context(), app, repo, ledger
}

func requireBalance(t *testing.T, repo *fakeRepository, accountID uuid.UUID, want int64) {
	t.Helper()

	if got := repo.balances[accountID]; got != want {
		t.Fatalf("balance of %s = %d; want %d", accountID, got, want)
	}
}

func requireNoTransactions(t *testing.T, ledger *fakeLedger) {
	t.Helper()

	if len(ledger.transactions) != 0 {
		t.Fatalf("ledger made %d transaction(s); want none", len(ledger.transactions))
	}
}

func TestApplicationCreateAccount(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)
	userID := repo.addUser()

	accountID, err := app.CreateAccount(ctx, userID)
	if err != nil {
		t.Fatalf("could not create account: %v", err)
	}

	if holderID, ok := repo.accounts[accountID]; !ok || holderID != userID {
		t.Fatalf("stored holder = %s (found: %t); want %s", holderID, ok, userID)
	}
	requireBalance(t, repo, accountID, 0)
}

func TestApplicationCreateAccountRepositoryError(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)
	userID := repo.addUser()
	repo.err = errRepository

	if _, err := app.CreateAccount(ctx, userID); !errors.Is(err, errRepository) {
		t.Fatalf("err = %v; want %v", err, errRepository)
	}
}

func TestApplicationGetBalance(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)
	userID := repo.addUser()
	accountID := repo.addAccount(userID, 42)

	balance, err := app.GetBalance(ctx, userID, accountID)
	if err != nil || balance != 42 {
		t.Fatalf("GetBalance = %d, %v; want 42, nil", balance, err)
	}
}

func TestApplicationGetBalanceNotHolder(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)
	accountID := repo.addAccount(repo.addUser(), 42)

	if _, err := app.GetBalance(ctx, repo.addUser(), accountID); !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotHolder)
	}
}

func TestApplicationGetBalanceUnknownAccount(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)

	if _, err := app.GetBalance(ctx, repo.addUser(), uuid.New()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, application.ErrNotFound)
	}
}

func TestApplicationTransfer(t *testing.T) {
	ctx, app, repo, ledger := setupFake(t)
	userID := repo.addUser()
	sourceID := repo.addAccount(userID, 100)
	targetID := repo.addAccount(repo.addUser(), 0)

	ok, err := app.Transfer(ctx, userID, sourceID, targetID, 30)
	if err != nil || !ok {
		t.Fatalf("Transfer = %t, %v; want true, nil", ok, err)
	}

	want := fakeTransaction{SourceID: sourceID, TargetID: targetID, Amount: 30}
	if len(ledger.transactions) != 1 || ledger.transactions[0] != want {
		t.Fatalf("ledger transactions = %+v; want [%+v]", ledger.transactions, want)
	}

	requireBalance(t, repo, sourceID, 70)
	requireBalance(t, repo, targetID, 30)
}

func TestApplicationTransferInsufficientFunds(t *testing.T) {
	ctx, app, repo, ledger := setupFake(t)
	userID := repo.addUser()
	sourceID := repo.addAccount(userID, 29)
	targetID := repo.addAccount(repo.addUser(), 0)

	ok, err := app.Transfer(ctx, userID, sourceID, targetID, 30)
	if ok || !errors.Is(err, domain.ErrInsufficientFund) {
		t.Fatalf("Transfer = %t, %v; want false, %v", ok, err, domain.ErrInsufficientFund)
	}

	requireNoTransactions(t, ledger)
}

func TestApplicationTransferNotHolder(t *testing.T) {
	ctx, app, repo, ledger := setupFake(t)
	sourceID := repo.addAccount(repo.addUser(), 100)
	targetID := repo.addAccount(repo.addUser(), 0)

	ok, err := app.Transfer(ctx, repo.addUser(), sourceID, targetID, 30)
	if ok || !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("Transfer = %t, %v; want false, %v", ok, err, domain.ErrNotHolder)
	}

	requireNoTransactions(t, ledger)
}

func TestApplicationTransferUnknownSource(t *testing.T) {
	ctx, app, repo, ledger := setupFake(t)
	userID := repo.addUser()
	targetID := repo.addAccount(repo.addUser(), 0)

	ok, err := app.Transfer(ctx, userID, uuid.New(), targetID, 30)
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Transfer = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}

	requireNoTransactions(t, ledger)
}

func TestApplicationTransferUnknownTarget(t *testing.T) {
	ctx, app, repo, ledger := setupFake(t)
	userID := repo.addUser()
	sourceID := repo.addAccount(userID, 100)

	ok, err := app.Transfer(ctx, userID, sourceID, uuid.New(), 30)
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Transfer = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}

	requireNoTransactions(t, ledger)
}

func TestApplicationTransferLedgerError(t *testing.T) {
	ctx, app, repo, ledger := setupFake(t)
	userID := repo.addUser()
	sourceID := repo.addAccount(userID, 100)
	targetID := repo.addAccount(repo.addUser(), 0)
	ledger.err = errLedger

	ok, err := app.Transfer(ctx, userID, sourceID, targetID, 30)
	if ok || !errors.Is(err, errLedger) {
		t.Fatalf("Transfer = %t, %v; want false, %v", ok, err, errLedger)
	}

	requireBalance(t, repo, sourceID, 100)
}

func TestApplicationGetUserAccounts(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)
	userID := repo.addUser()
	first := repo.addAccount(userID, 0)
	second := repo.addAccount(userID, 50)
	repo.addAccount(repo.addUser(), 7)

	accounts, err := app.GetUserAccounts(ctx, userID)
	if err != nil {
		t.Fatalf("could not get user accounts: %v", err)
	}

	got := map[uuid.UUID]int64{}
	for _, account := range accounts {
		if account.HolderID != userID {
			t.Fatalf("account %s belongs to %s; want %s", account.ID, account.HolderID, userID)
		}
		got[account.ID] = account.Balance()
	}

	want := map[uuid.UUID]int64{first: 0, second: 50}
	if len(accounts) != len(want) || !maps.Equal(got, want) {
		t.Fatalf("accounts = %v; want %v", got, want)
	}
}

func TestApplicationGetUserAccountsUnknownUser(t *testing.T) {
	ctx, app, _, _ := setupFake(t)

	if _, err := app.GetUserAccounts(ctx, uuid.New()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, application.ErrNotFound)
	}
}

func TestApplicationGetUserAccountsRepositoryError(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)
	userID := repo.addUser()
	repo.err = errRepository

	_, err := app.GetUserAccounts(ctx, userID)
	if !errors.Is(err, errRepository) {
		t.Fatalf("err = %v; want %v", err, errRepository)
	}
}

func TestApplicationGetById(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)
	userID := repo.addUser()
	accountID := repo.addAccount(userID, 42)

	account, err := app.GetById(ctx, userID, accountID)
	if err != nil {
		t.Fatalf("could not get account: %v", err)
	}
	if account.ID != accountID || account.HolderID != userID || account.Balance() != 42 {
		t.Fatalf("account = {%s %s %d}; want {%s %s 42}", account.ID, account.HolderID, account.Balance(), accountID, userID)
	}
}

func TestApplicationGetByIdNotHolder(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)
	accountID := repo.addAccount(repo.addUser(), 42)

	if _, err := app.GetById(ctx, repo.addUser(), accountID); !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("err = %v; want %v", err, domain.ErrNotHolder)
	}
}

func TestApplicationGetByIdUnknownAccount(t *testing.T) {
	ctx, app, repo, _ := setupFake(t)

	if _, err := app.GetById(ctx, repo.addUser(), uuid.New()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, application.ErrNotFound)
	}
}

type fakeRepository struct {
	users    map[uuid.UUID]bool
	accounts map[uuid.UUID]uuid.UUID
	balances map[uuid.UUID]int64
	err      error
}

func (r *fakeRepository) addUser() uuid.UUID {
	id := uuid.New()
	r.users[id] = true

	return id
}

func (r *fakeRepository) addAccount(holderID uuid.UUID, balance int64) uuid.UUID {
	id := uuid.New()
	r.accounts[id] = holderID
	r.balances[id] = balance

	return id
}

func (r *fakeRepository) SaveAccount(ctx context.Context, acc *domain.Account) error {
	if r.err != nil {
		return r.err
	}
	if !r.users[acc.HolderID] {
		return fmt.Errorf("holder %s does not exist", acc.HolderID)
	}
	if _, ok := r.accounts[acc.ID]; ok {
		return fmt.Errorf("account %s already exists", acc.ID)
	}

	r.accounts[acc.ID] = acc.HolderID

	return nil
}

func (r *fakeRepository) GetById(ctx context.Context, accID uuid.UUID) (domain.Account, error) {
	if r.err != nil {
		return domain.Account{}, r.err
	}

	holderID, ok := r.accounts[accID]
	if !ok {
		return domain.Account{}, fmt.Errorf("%w: %s", application.ErrNotFound, accID)
	}

	account := domain.NewAccount(holderID, r.balances[accID])
	account.ID = accID

	return *account, nil
}

func (r *fakeRepository) GetUserAccounts(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	if r.err != nil {
		return nil, r.err
	}

	res := []domain.Account{}
	for id, holderID := range r.accounts {
		if holderID != userID {
			continue
		}

		account := domain.NewAccount(holderID, r.balances[id])
		account.ID = id
		res = append(res, *account)
	}

	slices.SortFunc(res, func(a, b domain.Account) int { return bytes.Compare(a.ID[:], b.ID[:]) })

	return res, nil
}

func (r *fakeRepository) UserExist(ctx context.Context, userID uuid.UUID) (bool, error) {
	if r.err != nil {
		return false, r.err
	}

	return r.users[userID], nil
}

type fakeTransaction struct {
	SourceID uuid.UUID
	TargetID uuid.UUID
	Amount   int64
}

type fakeLedger struct {
	repo         *fakeRepository
	transactions []fakeTransaction
	err          error
}

func (l *fakeLedger) MakeTransaction(ctx context.Context, sourceID, targetID uuid.UUID, amount int64) (uuid.UUID, error) {
	if l.err != nil {
		return uuid.Nil, l.err
	}

	l.transactions = append(l.transactions, fakeTransaction{SourceID: sourceID, TargetID: targetID, Amount: amount})
	l.repo.balances[sourceID] -= amount
	l.repo.balances[targetID] += amount

	return uuid.New(), nil
}
