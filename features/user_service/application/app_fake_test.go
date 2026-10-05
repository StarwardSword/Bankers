package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/StarwardSword/bank/features/user_service/application"
	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/google/uuid"
)

var (
	errRepository = errors.New("repository is down")
	errTokens     = errors.New("token service is down")
)

func setupFake(t *testing.T) (context.Context, *application.Application, *fakeRepository, *fakeHasher, *fakeAuthService) {
	t.Helper()

	repo := &fakeRepository{users: map[uuid.UUID]domain.User{}}
	hasher := &fakeHasher{}
	auth := &fakeAuthService{}

	return t.Context(), application.NewApplication(hasher, repo, auth), repo, hasher, auth
}

func createUser(t *testing.T, app *application.Application, name, password string) uuid.UUID {
	t.Helper()

	id, err := app.CreateUser(t.Context(), name, password)
	if err != nil {
		t.Fatalf("creating user %q: %v", name, err)
	}

	return id
}

func createAdmin(t *testing.T, app *application.Application, repo application.Repository, name, password string) uuid.UUID {
	t.Helper()

	id := createUser(t, app, name, password)

	user, err := repo.GetUserById(t.Context(), id)
	if err != nil {
		t.Fatalf("getting user %q: %v", name, err)
	}

	user.Role = domain.RoleAdmin
	if err := repo.UpdateUser(t.Context(), &user); err != nil {
		t.Fatalf("promoting %q to admin: %v", name, err)
	}

	return id
}

func requireLogin(t *testing.T, app *application.Application, name, password string, want error) {
	t.Helper()

	_, err := app.AuthUser(t.Context(), name, password)
	switch {
	case want == nil && err != nil:
		t.Fatalf("AuthUser(%q, %q) err = %v; want nil", name, password, err)
	case want != nil && !errors.Is(err, want):
		t.Fatalf("AuthUser(%q, %q) err = %v; want %v", name, password, err, want)
	}
}

func requireRole(t *testing.T, repo application.Repository, id uuid.UUID, want domain.Role) {
	t.Helper()

	user, err := repo.GetUserById(t.Context(), id)
	if err != nil {
		t.Fatalf("getting user %s: %v", id, err)
	}
	if user.Role != want {
		t.Fatalf("role of %s = %d; want %d", id, user.Role, want)
	}
}

func requireRemoved(t *testing.T, repo application.Repository, id uuid.UUID) {
	t.Helper()

	if _, err := repo.GetUserById(t.Context(), id); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("GetUserById(%s) err = %v; want %v", id, err, application.ErrNotFound)
	}
}

func requireNoTokens(t *testing.T, auth *fakeAuthService) {
	t.Helper()

	if len(auth.requests) != 0 {
		t.Fatalf("token requests = %+v; want none", auth.requests)
	}
}

func TestApplicationCreateUser(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)

	id, err := app.CreateUser(ctx, "alice", "password")
	if err != nil {
		t.Fatalf("could not create user: %v", err)
	}

	stored, ok := repo.users[id]
	if !ok {
		t.Fatalf("user %s was not saved", id)
	}
	if stored.Username != "alice" || stored.Role != domain.RoleUser || stored.Password == "password" {
		t.Fatalf("stored user = %+v; want alice with role %d and a hashed password", stored, domain.RoleUser)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationCreateUserTakenUsername(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	first := createUser(t, app, "alice", "password")

	if _, err := app.CreateUser(ctx, "alice", "another password"); !errors.Is(err, application.ErrAlreadyExists) {
		t.Fatalf("err = %v; want %v", err, application.ErrAlreadyExists)
	}

	if len(repo.users) != 1 {
		t.Fatalf("stored %d user(s); want 1", len(repo.users))
	}
	if _, ok := repo.users[first]; !ok {
		t.Fatalf("first user %s is gone", first)
	}
}

func TestApplicationCreateUserEmptyPassword(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)

	if _, err := app.CreateUser(ctx, "alice", ""); !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInvalidArgument)
	}

	if len(repo.users) != 0 {
		t.Fatalf("stored %d user(s); want none", len(repo.users))
	}
}

func TestApplicationCreateUserRepositoryError(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	repo.err = errRepository

	if _, err := app.CreateUser(ctx, "alice", "password"); !errors.Is(err, errRepository) {
		t.Fatalf("err = %v; want %v", err, errRepository)
	}
}

func TestApplicationAuthUser(t *testing.T) {
	ctx, app, _, _, auth := setupFake(t)
	id := createUser(t, app, "alice", "password")

	token, err := app.AuthUser(ctx, "alice", "password")
	if err != nil {
		t.Fatalf("could not authenticate: %v", err)
	}
	if want := "token for " + id.String(); token != want {
		t.Fatalf("token = %q; want %q", token, want)
	}

	want := []tokenRequest{{ID: id, Role: domain.RoleUser}}
	if !slices.Equal(auth.requests, want) {
		t.Fatalf("token requests = %+v; want %+v", auth.requests, want)
	}
}

func TestApplicationAuthUserExactPassword(t *testing.T) {
	_, app, _, _, _ := setupFake(t)
	createUser(t, app, "alice", " password ")

	requireLogin(t, app, "alice", " password ", nil)
	requireLogin(t, app, "alice", "password", application.ErrWrongPassword)
}

func TestApplicationAuthUserUnknownUser(t *testing.T) {
	ctx, app, _, _, auth := setupFake(t)

	if _, err := app.AuthUser(ctx, "nobody", "password"); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, application.ErrNotFound)
	}

	requireNoTokens(t, auth)
}

func TestApplicationAuthUserWrongPassword(t *testing.T) {
	ctx, app, _, _, auth := setupFake(t)
	createUser(t, app, "alice", "password")

	if _, err := app.AuthUser(ctx, "alice", "wrong password"); !errors.Is(err, application.ErrWrongPassword) {
		t.Fatalf("err = %v; want %v", err, application.ErrWrongPassword)
	}

	requireNoTokens(t, auth)
}

func TestApplicationAuthUserHasherError(t *testing.T) {
	ctx, app, _, hasher, auth := setupFake(t)
	createUser(t, app, "alice", "password")
	hasher.err = domain.ErrHashService

	if _, err := app.AuthUser(ctx, "alice", "password"); !errors.Is(err, domain.ErrHashService) {
		t.Fatalf("err = %v; want %v", err, domain.ErrHashService)
	}

	requireNoTokens(t, auth)
}

func TestApplicationAuthUserTokenError(t *testing.T) {
	ctx, app, _, _, auth := setupFake(t)
	createUser(t, app, "alice", "password")
	auth.err = errTokens

	token, err := app.AuthUser(ctx, "alice", "password")
	if token != "" || !errors.Is(err, errTokens) {
		t.Fatalf("AuthUser = %q, %v; want \"\", %v", token, err, errTokens)
	}
}

func TestApplicationAuthUserRepositoryError(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	createUser(t, app, "alice", "password")
	repo.err = errRepository

	_, err := app.AuthUser(ctx, "alice", "password")
	if !errors.Is(err, errRepository) || errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v; want %v and not %v", err, errRepository, application.ErrNotFound)
	}
}

func TestApplicationChangePassword(t *testing.T) {
	ctx, app, _, _, _ := setupFake(t)
	id := createUser(t, app, "alice", "password")

	ok, err := app.ChangePassword(ctx, id, id, "password", "new password")
	if err != nil || !ok {
		t.Fatalf("ChangePassword = %t, %v; want true, nil", ok, err)
	}

	requireLogin(t, app, "alice", "new password", nil)
	requireLogin(t, app, "alice", "password", application.ErrWrongPassword)
}

func TestApplicationChangePasswordWrongOldPassword(t *testing.T) {
	ctx, app, _, _, _ := setupFake(t)
	id := createUser(t, app, "alice", "password")

	ok, err := app.ChangePassword(ctx, id, id, "wrong password", "new password")
	if ok || !errors.Is(err, domain.ErrWrongPassword) {
		t.Fatalf("ChangePassword = %t, %v; want false, %v", ok, err, domain.ErrWrongPassword)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationChangePasswordEmptyNewPassword(t *testing.T) {
	ctx, app, _, _, _ := setupFake(t)
	id := createUser(t, app, "alice", "password")

	ok, err := app.ChangePassword(ctx, id, id, "password", "")
	if ok || !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("ChangePassword = %t, %v; want false, %v", ok, err, domain.ErrInvalidArgument)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationChangePasswordOtherUser(t *testing.T) {
	ctx, app, _, _, _ := setupFake(t)
	alice := createUser(t, app, "alice", "password")
	mallory := createUser(t, app, "mallory", "password")

	ok, err := app.ChangePassword(ctx, mallory, alice, "password", "new password")
	if ok || !errors.Is(err, application.ErrPermissionDenied) {
		t.Fatalf("ChangePassword = %t, %v; want false, %v", ok, err, application.ErrPermissionDenied)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationChangePasswordUnknownUser(t *testing.T) {
	ctx, app, _, _, _ := setupFake(t)
	id := uuid.New()

	ok, err := app.ChangePassword(ctx, id, id, "password", "new password")
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ChangePassword = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}
}

func TestApplicationChangeRole(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	admin := createAdmin(t, app, repo, "admin", "password")
	alice := createUser(t, app, "alice", "password")

	ok, err := app.ChangeRole(ctx, admin, alice, domain.RoleAdmin)
	if err != nil || !ok {
		t.Fatalf("ChangeRole = %t, %v; want true, nil", ok, err)
	}

	requireRole(t, repo, alice, domain.RoleAdmin)
}

func TestApplicationChangeRoleNotAdmin(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	alice := createUser(t, app, "alice", "password")

	ok, err := app.ChangeRole(ctx, alice, alice, domain.RoleAdmin)
	if ok || !errors.Is(err, application.ErrPermissionDenied) {
		t.Fatalf("ChangeRole = %t, %v; want false, %v", ok, err, application.ErrPermissionDenied)
	}

	requireRole(t, repo, alice, domain.RoleUser)
}

func TestApplicationChangeRoleUnknownUser(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	admin := createAdmin(t, app, repo, "admin", "password")

	ok, err := app.ChangeRole(ctx, admin, uuid.New(), domain.RoleAdmin)
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ChangeRole = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}
}

func TestApplicationChangeRoleUnknownRequester(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	alice := createUser(t, app, "alice", "password")

	ok, err := app.ChangeRole(ctx, uuid.New(), alice, domain.RoleAdmin)
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ChangeRole = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}

	requireRole(t, repo, alice, domain.RoleUser)
}

func TestApplicationRemoveUser(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	admin := createAdmin(t, app, repo, "admin", "password")
	alice := createUser(t, app, "alice", "password")

	ok, err := app.RemoveUser(ctx, admin, alice)
	if err != nil || !ok {
		t.Fatalf("RemoveUser = %t, %v; want true, nil", ok, err)
	}

	requireRemoved(t, repo, alice)
	requireLogin(t, app, "alice", "password", application.ErrNotFound)
}

func TestApplicationRemoveUserNotAdmin(t *testing.T) {
	ctx, app, _, _, _ := setupFake(t)
	alice := createUser(t, app, "alice", "password")
	mallory := createUser(t, app, "mallory", "password")

	ok, err := app.RemoveUser(ctx, mallory, alice)
	if ok || !errors.Is(err, application.ErrPermissionDenied) {
		t.Fatalf("RemoveUser = %t, %v; want false, %v", ok, err, application.ErrPermissionDenied)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationRemoveUserUnknownUser(t *testing.T) {
	ctx, app, repo, _, _ := setupFake(t)
	admin := createAdmin(t, app, repo, "admin", "password")

	ok, err := app.RemoveUser(ctx, admin, uuid.New())
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("RemoveUser = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}
}

func TestApplicationRemoveUserUnknownRequester(t *testing.T) {
	ctx, app, _, _, _ := setupFake(t)
	alice := createUser(t, app, "alice", "password")

	ok, err := app.RemoveUser(ctx, uuid.New(), alice)
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("RemoveUser = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}

	requireLogin(t, app, "alice", "password", nil)
}

type fakeRepository struct {
	users map[uuid.UUID]domain.User
	err   error
}

func (r *fakeRepository) SaveUser(ctx context.Context, u *domain.User) error {
	if r.err != nil {
		return r.err
	}
	for _, existing := range r.users {
		if existing.Username == u.Username {
			return fmt.Errorf("%w: %s", application.ErrAlreadyExists, u.Username)
		}
	}
	if _, ok := r.users[u.ID]; ok {
		return fmt.Errorf("user %s already exists", u.ID)
	}

	r.users[u.ID] = *u

	return nil
}

func (r *fakeRepository) RemoveUser(ctx context.Context, u *domain.User) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.users[u.ID]; !ok {
		return fmt.Errorf("%w: %s", application.ErrNotFound, u.ID)
	}

	delete(r.users, u.ID)

	return nil
}

func (r *fakeRepository) UpdateUser(ctx context.Context, u *domain.User) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.users[u.ID]; !ok {
		return fmt.Errorf("%w: %s", application.ErrNotFound, u.ID)
	}

	r.users[u.ID] = *u

	return nil
}

func (r *fakeRepository) GetUserById(ctx context.Context, id uuid.UUID) (domain.User, error) {
	if r.err != nil {
		return domain.User{}, r.err
	}

	user, ok := r.users[id]
	if !ok {
		return domain.User{}, fmt.Errorf("%w: %s", application.ErrNotFound, id)
	}

	return user, nil
}

func (r *fakeRepository) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	if r.err != nil {
		return domain.User{}, r.err
	}

	for _, user := range r.users {
		if user.Username == username {
			return user, nil
		}
	}

	return domain.User{}, fmt.Errorf("%w: %s", application.ErrNotFound, username)
}

type fakeHasher struct {
	err error
}

func (h *fakeHasher) Hash(password string) string {
	sum := sha256.Sum256([]byte(password))

	return hex.EncodeToString(sum[:])
}

func (h *fakeHasher) Compare(raw, hashed string) (bool, error) {
	if h.err != nil {
		return false, h.err
	}

	return h.Hash(raw) == hashed, nil
}

type tokenRequest struct {
	ID   uuid.UUID
	Role domain.Role
}

type fakeAuthService struct {
	requests []tokenRequest
	err      error
}

func (a *fakeAuthService) MakeToken(id uuid.UUID, role domain.Role) (string, error) {
	if a.err != nil {
		return "", a.err
	}

	a.requests = append(a.requests, tokenRequest{ID: id, Role: role})

	return "token for " + id.String(), nil
}
