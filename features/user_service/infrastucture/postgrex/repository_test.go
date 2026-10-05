package postgrex_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/StarwardSword/bank/features/user_service/application"
	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/StarwardSword/bank/features/user_service/infrastucture/postgrex"
	"github.com/StarwardSword/bank/pkg/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setup(t *testing.T) (context.Context, *postgrex.Repository, *pgxpool.Pool) {
	t.Helper()

	pool := testdb.Pool(t)

	return t.Context(), postgrex.NewRepository(t.Context(), pool), pool
}

func newUser(name string) *domain.User {
	return &domain.User{
		ID:       uuid.New(),
		Username: name,
		Password: "hash of " + name,
		Role:     domain.RoleUser,
	}
}

func createUser(t *testing.T, repo *postgrex.Repository, name string) *domain.User {
	t.Helper()

	user := newUser(name)
	if err := repo.SaveUser(t.Context(), user); err != nil {
		t.Fatalf("saving user %q: %v", name, err)
	}

	return user
}

func createAccount(t *testing.T, pool *pgxpool.Pool, holderID uuid.UUID) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := pool.QueryRow(t.Context(), `
		INSERT INTO accounts (holder_id)
		VALUES ($1)
		RETURNING id
	`, holderID).Scan(&id)
	if err != nil {
		t.Fatalf("creating account: %v", err)
	}

	return id
}

func fund(t *testing.T, pool *pgxpool.Pool, accountID uuid.UUID, amount int64) {
	t.Helper()

	tag, err := pool.Exec(t.Context(), `
		INSERT INTO transaction (debit_account_id, credit_account_id, amount)
		SELECT accounts.id, $1, $2
		FROM accounts
		JOIN users ON users.id = accounts.holder_id
		WHERE users.username = 'treasury'
	`, accountID, amount)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("funding %s with %d from the treasury: %v (rows: %d)", accountID, amount, err, tag.RowsAffected())
	}
}

func readStoredUser(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) (username string, removed bool) {
	t.Helper()

	err := pool.QueryRow(t.Context(), `
		SELECT username, removed
		FROM users
		WHERE id = $1
	`, id).Scan(&username, &removed)
	if err != nil {
		t.Fatalf("reading stored user %s: %v", id, err)
	}

	return username, removed
}

func requireUser(t *testing.T, got, want domain.User) {
	t.Helper()

	if got != want {
		t.Fatalf("user = %+v; want %+v", got, want)
	}
}

func requireNotFound(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, application.ErrNotFound)
	}
}

func requireRemovedUser(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()

	username, removed := readStoredUser(t, pool, id)
	if !removed {
		t.Fatalf("user %s is not marked as removed", id)
	}

	suffix, ok := strings.CutPrefix(username, "removed_user_")
	if _, err := uuid.Parse(suffix); !ok || err != nil {
		t.Fatalf("username of removed user = %q; want removed_user_{uuid}", username)
	}

	return username
}

func requireAccountRemoved(t *testing.T, pool *pgxpool.Pool, accountID uuid.UUID, want bool) {
	t.Helper()

	var removed bool
	err := pool.QueryRow(t.Context(), `
		SELECT removed
		FROM accounts
		WHERE id = $1
	`, accountID).Scan(&removed)
	if err != nil {
		t.Fatalf("reading account %s: %v", accountID, err)
	}
	if removed != want {
		t.Fatalf("account %s removed = %t; want %t", accountID, removed, want)
	}
}

func requireBalance(t *testing.T, pool *pgxpool.Pool, accountID uuid.UUID, want int64) {
	t.Helper()

	var got int64
	err := pool.QueryRow(t.Context(), `
		SELECT get_account_balance($1)
	`, accountID).Scan(&got)
	if err != nil || got != want {
		t.Fatalf("balance of %s = %d, %v; want %d, nil", accountID, got, err, want)
	}
}

func TestRepositorySaveUser(t *testing.T) {
	ctx, repo, _ := setup(t)
	user := newUser("alice")

	if err := repo.SaveUser(ctx, user); err != nil {
		t.Fatalf("could not save user: %v", err)
	}

	got, err := repo.GetUserById(ctx, user.ID)
	if err != nil {
		t.Fatalf("could not get saved user: %v", err)
	}

	requireUser(t, got, *user)
}

func TestRepositorySaveUserRoles(t *testing.T) {
	ctx, repo, _ := setup(t)

	for _, role := range []domain.Role{domain.RoleGuest, domain.RoleUser, domain.RoleAdmin} {
		t.Run(fmt.Sprintf("role %d", role), func(t *testing.T) {
			user := newUser(fmt.Sprintf("user with role %d", role))
			user.Role = role

			if err := repo.SaveUser(ctx, user); err != nil {
				t.Fatalf("could not save user: %v", err)
			}

			got, err := repo.GetUserById(ctx, user.ID)
			if err != nil {
				t.Fatalf("could not get saved user: %v", err)
			}

			requireUser(t, got, *user)
		})
	}
}

func TestRepositorySaveUserTakenUsername(t *testing.T) {
	ctx, repo, _ := setup(t)
	first := createUser(t, repo, "alice")

	err := repo.SaveUser(ctx, newUser("alice"))
	if !errors.Is(err, application.ErrAlreadyExists) {
		t.Fatalf("err = %v; want %v", err, application.ErrAlreadyExists)
	}

	got, err := repo.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("could not get user: %v", err)
	}

	requireUser(t, got, *first)
}

func TestRepositorySaveUserDoesNotOverwrite(t *testing.T) {
	ctx, repo, _ := setup(t)
	first := createUser(t, repo, "alice")

	stolen := newUser("mallory")
	stolen.ID = first.ID

	if err := repo.SaveUser(ctx, stolen); err == nil {
		t.Fatalf("saved a user over an existing one")
	}

	got, err := repo.GetUserById(ctx, first.ID)
	if err != nil {
		t.Fatalf("could not get user: %v", err)
	}
	requireUser(t, got, *first)

	_, err = repo.GetUserByUsername(ctx, "mallory")
	requireNotFound(t, err)
}

func TestRepositoryGetUserByIdNotFound(t *testing.T) {
	ctx, repo, _ := setup(t)

	_, err := repo.GetUserById(ctx, uuid.New())
	requireNotFound(t, err)
}

func TestRepositoryGetUserByUsername(t *testing.T) {
	ctx, repo, _ := setup(t)
	alice := createUser(t, repo, "alice")
	createUser(t, repo, "bob")

	got, err := repo.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("could not get user: %v", err)
	}

	requireUser(t, got, *alice)
}

func TestRepositoryGetUserByUsernameNotFound(t *testing.T) {
	ctx, repo, _ := setup(t)

	_, err := repo.GetUserByUsername(ctx, "nobody")
	requireNotFound(t, err)
}

func TestRepositoryUpdateUser(t *testing.T) {
	ctx, repo, _ := setup(t)
	user := createUser(t, repo, "alice")

	user.Username = "alice smith"
	user.Password = "new hash"
	user.Role = domain.RoleAdmin

	if err := repo.UpdateUser(ctx, user); err != nil {
		t.Fatalf("could not update user: %v", err)
	}

	got, err := repo.GetUserById(ctx, user.ID)
	if err != nil {
		t.Fatalf("could not get updated user: %v", err)
	}

	requireUser(t, got, *user)
}

func TestRepositoryUpdateUserNotFound(t *testing.T) {
	ctx, repo, _ := setup(t)

	requireNotFound(t, repo.UpdateUser(ctx, newUser("ghost")))
}

func TestRepositoryUpdateUserTakenUsername(t *testing.T) {
	ctx, repo, _ := setup(t)
	alice := createUser(t, repo, "alice")
	bob := createUser(t, repo, "bob")

	renamed := *bob
	renamed.Username = "alice"

	if err := repo.UpdateUser(ctx, &renamed); err == nil {
		t.Fatalf("renamed a user to a taken username")
	}

	for _, want := range []*domain.User{alice, bob} {
		got, err := repo.GetUserById(ctx, want.ID)
		if err != nil {
			t.Fatalf("could not get user: %v", err)
		}

		requireUser(t, got, *want)
	}
}

func TestRepositoryUpdateUserWaitsForRemoval(t *testing.T) {
	ctx, repo, pool := setup(t)
	alice := createUser(t, repo, "alice")

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("beginning removal: %v", err)
	}
	defer tx.Rollback(context.Background())

	_, err = tx.Exec(ctx, `
		UPDATE users SET
			removed = true,
			username = 'removed_user_' || gen_random_uuid()
		WHERE id = $1
	`, alice.ID)
	if err != nil {
		t.Fatalf("removing user: %v", err)
	}

	updated := *alice
	updated.Password = "new hash"
	done := make(chan error, 1)
	go func() { done <- repo.UpdateUser(ctx, &updated) }()

	select {
	case err := <-done:
		t.Fatalf("updated a user that is being removed (err: %v)", err)
	case <-time.After(200 * time.Millisecond):
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("committing removal: %v", err)
	}

	requireNotFound(t, <-done)
	requireRemovedUser(t, pool, alice.ID)
}

func TestRepositoryRemoveUser(t *testing.T) {
	ctx, repo, pool := setup(t)
	alice := createUser(t, repo, "alice")
	bob := createUser(t, repo, "bob")
	aliceAccounts := []uuid.UUID{createAccount(t, pool, alice.ID), createAccount(t, pool, alice.ID)}
	bobAccount := createAccount(t, pool, bob.ID)

	if err := repo.RemoveUser(ctx, alice); err != nil {
		t.Fatalf("could not remove user: %v", err)
	}

	requireRemovedUser(t, pool, alice.ID)
	for _, id := range aliceAccounts {
		requireAccountRemoved(t, pool, id, true)
	}
	requireAccountRemoved(t, pool, bobAccount, false)

	got, err := repo.GetUserById(ctx, bob.ID)
	if err != nil {
		t.Fatalf("could not get another user: %v", err)
	}
	requireUser(t, got, *bob)
}

func TestRepositoryRemoveUserKeepsHistory(t *testing.T) {
	ctx, repo, pool := setup(t)
	alice := createUser(t, repo, "alice")
	account := createAccount(t, pool, alice.ID)
	fund(t, pool, account, 100)

	if err := repo.RemoveUser(ctx, alice); err != nil {
		t.Fatalf("could not remove user with transaction history: %v", err)
	}

	requireRemovedUser(t, pool, alice.ID)
	requireAccountRemoved(t, pool, account, true)
	requireBalance(t, pool, account, 100)
}

func TestRepositoryRemoveUserNotFound(t *testing.T) {
	ctx, repo, _ := setup(t)

	requireNotFound(t, repo.RemoveUser(ctx, newUser("ghost")))
}

func TestRepositoryRemoveUserFreesUsername(t *testing.T) {
	ctx, repo, _ := setup(t)
	first := createUser(t, repo, "alice")
	if err := repo.RemoveUser(ctx, first); err != nil {
		t.Fatalf("could not remove user: %v", err)
	}

	second := createUser(t, repo, "alice")

	got, err := repo.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("could not get user: %v", err)
	}
	requireUser(t, got, *second)

	if err := repo.RemoveUser(ctx, second); err != nil {
		t.Fatalf("could not remove the second user with the same name: %v", err)
	}
}

func TestRepositoryRemoveUserConcurrently(t *testing.T) {
	const attempts = 10

	ctx, repo, pool := setup(t)
	alice := createUser(t, repo, "alice")

	start := make(chan struct{})
	errs := make(chan error, attempts)
	for range attempts {
		go func() {
			<-start
			errs <- repo.RemoveUser(ctx, alice)
		}()
	}
	close(start)

	removed := 0
	for range attempts {
		err := <-errs
		if err == nil {
			removed++
			continue
		}

		requireNotFound(t, err)
	}

	if removed != 1 {
		t.Fatalf("%d removal(s) succeeded; want 1", removed)
	}
	requireRemovedUser(t, pool, alice.ID)
}

func TestRepositoryRemovedUserIsNotFound(t *testing.T) {
	ctx, repo, pool := setup(t)
	alice := createUser(t, repo, "alice")
	if err := repo.RemoveUser(ctx, alice); err != nil {
		t.Fatalf("could not remove user: %v", err)
	}
	removedName := requireRemovedUser(t, pool, alice.ID)

	t.Run("GetUserById", func(t *testing.T) {
		_, err := repo.GetUserById(ctx, alice.ID)
		requireNotFound(t, err)
	})

	t.Run("GetUserByUsername", func(t *testing.T) {
		_, err := repo.GetUserByUsername(ctx, "alice")
		requireNotFound(t, err)
	})

	t.Run("GetUserByRemovedUsername", func(t *testing.T) {
		_, err := repo.GetUserByUsername(ctx, removedName)
		requireNotFound(t, err)
	})

	t.Run("UpdateUser", func(t *testing.T) {
		requireNotFound(t, repo.UpdateUser(ctx, alice))

		if username, _ := readStoredUser(t, pool, alice.ID); username != removedName {
			t.Fatalf("username = %q after update; want %q", username, removedName)
		}
	})

	t.Run("RemoveUser", func(t *testing.T) {
		requireNotFound(t, repo.RemoveUser(ctx, alice))
	})
}

func TestRepositoryContextCancelled(t *testing.T) {
	ctx, repo, _ := setup(t)
	user := createUser(t, repo, "alice")

	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	t.Run("SaveUser", func(t *testing.T) {
		err := repo.SaveUser(ctx, newUser("bob"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})

	t.Run("RemoveUser", func(t *testing.T) {
		err := repo.RemoveUser(ctx, user)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})

	t.Run("UpdateUser", func(t *testing.T) {
		err := repo.UpdateUser(ctx, user)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})

	t.Run("GetUserById", func(t *testing.T) {
		_, err := repo.GetUserById(ctx, user.ID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})

	t.Run("GetUserByUsername", func(t *testing.T) {
		_, err := repo.GetUserByUsername(ctx, user.Username)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want %v", err, context.Canceled)
		}
	})
}
