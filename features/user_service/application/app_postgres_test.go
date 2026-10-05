package application_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StarwardSword/bank/features/user_service/application"
	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/StarwardSword/bank/features/user_service/infrastucture/passwordhasher"
	"github.com/StarwardSword/bank/features/user_service/infrastucture/postgrex"
	"github.com/StarwardSword/bank/pkg/auth"
	"github.com/StarwardSword/bank/pkg/testdb"
	"github.com/google/uuid"
)

type tokenIssuer func(id uuid.UUID, role domain.Role) (string, error)

func (f tokenIssuer) MakeToken(id uuid.UUID, role domain.Role) (string, error) {
	return f(id, role)
}

func setupPostgres(t *testing.T) (context.Context, *application.Application, *postgrex.Repository) {
	t.Helper()

	t.Setenv("JWT_SECRET", "test secret")

	repo := postgrex.NewRepository(t.Context(), testdb.Pool(t))
	app := application.NewApplication(passwordhasher.NewSimpleArgon2Hasher(), repo, tokenIssuer(auth.MakeToken))

	return t.Context(), app, repo
}

func requireToken(t *testing.T, token string, wantID uuid.UUID, wantRole domain.Role) {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: auth.AccessTokenCookie, Value: token})

	ctx, err := auth.PublicAuth(r)
	if err != nil {
		t.Fatalf("token was rejected: %v", err)
	}

	got, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("reading token: %v", err)
	}
	if got.UserId != wantID || got.Role != wantRole {
		t.Fatalf("token is for user %s with role %d; want %s with role %d", got.UserId, got.Role, wantID, wantRole)
	}
}

func TestApplicationPostgresCreateUser(t *testing.T) {
	ctx, app, repo := setupPostgres(t)

	id, err := app.CreateUser(ctx, "alice", "password")
	if err != nil {
		t.Fatalf("could not create user: %v", err)
	}

	stored, err := repo.GetUserById(ctx, id)
	if err != nil {
		t.Fatalf("could not get created user: %v", err)
	}
	if stored.Username != "alice" || stored.Role != domain.RoleUser || stored.Password == "password" {
		t.Fatalf("stored user = %+v; want alice with role %d and a hashed password", stored, domain.RoleUser)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationPostgresCreateUserTakenUsername(t *testing.T) {
	ctx, app, _ := setupPostgres(t)
	createUser(t, app, "alice", "password")

	if _, err := app.CreateUser(ctx, "alice", "another password"); !errors.Is(err, application.ErrAlreadyExists) {
		t.Fatalf("err = %v; want %v", err, application.ErrAlreadyExists)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationPostgresAuthUser(t *testing.T) {
	ctx, app, _ := setupPostgres(t)
	id := createUser(t, app, "alice", "password")

	token, err := app.AuthUser(ctx, "alice", "password")
	if err != nil {
		t.Fatalf("could not authenticate: %v", err)
	}

	requireToken(t, token, id, domain.RoleUser)
}

func TestApplicationPostgresAuthUserWrongPassword(t *testing.T) {
	ctx, app, _ := setupPostgres(t)
	createUser(t, app, "alice", "password")

	token, err := app.AuthUser(ctx, "alice", "wrong password")
	if token != "" || !errors.Is(err, application.ErrWrongPassword) {
		t.Fatalf("AuthUser = %q, %v; want \"\", %v", token, err, application.ErrWrongPassword)
	}
}

func TestApplicationPostgresAuthUserUnknownUser(t *testing.T) {
	ctx, app, _ := setupPostgres(t)

	token, err := app.AuthUser(ctx, "nobody", "password")
	if token != "" || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("AuthUser = %q, %v; want \"\", %v", token, err, application.ErrNotFound)
	}
}

func TestApplicationPostgresAuthUserContextCancelled(t *testing.T) {
	ctx, app, _ := setupPostgres(t)
	createUser(t, app, "alice", "password")

	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	token, err := app.AuthUser(ctx, "alice", "password")
	if token != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("AuthUser = %q, %v; want \"\", %v", token, err, context.Canceled)
	}
}

func TestApplicationPostgresChangePassword(t *testing.T) {
	ctx, app, _ := setupPostgres(t)
	id := createUser(t, app, "alice", "password")

	ok, err := app.ChangePassword(ctx, id, id, "password", "new password")
	if err != nil || !ok {
		t.Fatalf("ChangePassword = %t, %v; want true, nil", ok, err)
	}

	requireLogin(t, app, "alice", "new password", nil)
	requireLogin(t, app, "alice", "password", application.ErrWrongPassword)
}

func TestApplicationPostgresChangePasswordWrongOldPassword(t *testing.T) {
	ctx, app, _ := setupPostgres(t)
	id := createUser(t, app, "alice", "password")

	ok, err := app.ChangePassword(ctx, id, id, "wrong password", "new password")
	if ok || !errors.Is(err, domain.ErrWrongPassword) {
		t.Fatalf("ChangePassword = %t, %v; want false, %v", ok, err, domain.ErrWrongPassword)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationPostgresChangeRole(t *testing.T) {
	ctx, app, repo := setupPostgres(t)
	admin := createAdmin(t, app, repo, "admin", "password")
	alice := createUser(t, app, "alice", "password")

	ok, err := app.ChangeRole(ctx, admin, alice, domain.RoleAdmin)
	if err != nil || !ok {
		t.Fatalf("ChangeRole = %t, %v; want true, nil", ok, err)
	}

	requireRole(t, repo, alice, domain.RoleAdmin)

	token, err := app.AuthUser(ctx, "alice", "password")
	if err != nil {
		t.Fatalf("could not authenticate: %v", err)
	}
	requireToken(t, token, alice, domain.RoleAdmin)
}

func TestApplicationPostgresChangeRoleNotAdmin(t *testing.T) {
	ctx, app, repo := setupPostgres(t)
	alice := createUser(t, app, "alice", "password")

	ok, err := app.ChangeRole(ctx, alice, alice, domain.RoleAdmin)
	if ok || !errors.Is(err, application.ErrPermissionDenied) {
		t.Fatalf("ChangeRole = %t, %v; want false, %v", ok, err, application.ErrPermissionDenied)
	}

	requireRole(t, repo, alice, domain.RoleUser)
}

func TestApplicationPostgresRemoveUser(t *testing.T) {
	ctx, app, repo := setupPostgres(t)
	admin := createAdmin(t, app, repo, "admin", "password")
	alice := createUser(t, app, "alice", "password")

	ok, err := app.RemoveUser(ctx, admin, alice)
	if err != nil || !ok {
		t.Fatalf("RemoveUser = %t, %v; want true, nil", ok, err)
	}

	requireRemoved(t, repo, alice)
	requireLogin(t, app, "alice", "password", application.ErrNotFound)

	createUser(t, app, "alice", "another password")
	requireLogin(t, app, "alice", "another password", nil)
}

func TestApplicationPostgresRemoveUserNotAdmin(t *testing.T) {
	ctx, app, _ := setupPostgres(t)
	alice := createUser(t, app, "alice", "password")
	mallory := createUser(t, app, "mallory", "password")

	ok, err := app.RemoveUser(ctx, mallory, alice)
	if ok || !errors.Is(err, application.ErrPermissionDenied) {
		t.Fatalf("RemoveUser = %t, %v; want false, %v", ok, err, application.ErrPermissionDenied)
	}

	requireLogin(t, app, "alice", "password", nil)
}

func TestApplicationPostgresRemovedAdminCannotAct(t *testing.T) {
	ctx, app, repo := setupPostgres(t)
	root := createAdmin(t, app, repo, "root", "password")
	admin := createAdmin(t, app, repo, "admin", "password")
	alice := createUser(t, app, "alice", "password")

	if _, err := app.RemoveUser(ctx, root, admin); err != nil {
		t.Fatalf("could not remove admin: %v", err)
	}

	ok, err := app.ChangeRole(ctx, admin, alice, domain.RoleAdmin)
	if ok || !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ChangeRole by a removed admin = %t, %v; want false, %v", ok, err, application.ErrNotFound)
	}

	requireRole(t, repo, alice, domain.RoleUser)
}
