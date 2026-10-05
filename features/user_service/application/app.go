package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/google/uuid"
)

type Application struct {
	hasher domain.PasswordHasher
	repo   Repository
	auth   AuthService
}

func NewApplication(hasher domain.PasswordHasher, repo Repository, auth AuthService) *Application {
	return &Application{
		hasher: hasher,
		repo:   repo,
		auth:   auth,
	}
}

// CreateUser может вернуть ошибки:
//
// Приложение:
//   - [ErrAlreadyExists] — имя name уже занято
//
// Домен:
//   - [domain.ErrInvalidArgument] — name или password пустые
func (app *Application) CreateUser(ctx context.Context, name string, password string) (uuid.UUID, error) {
	role := domain.RoleUser

	u, err := domain.NewUser(app.hasher, name, password, role)
	if err != nil {
		return uuid.Nil, fmt.Errorf("creating user: %w", err)
	}

	err = app.repo.SaveUser(ctx, &u)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("save user: %w", err)
	}

	return u.ID, nil
}

// RemoveUser может вернуть ошибки:
//
// Приложение:
//   - [ErrNotFound] — requesterId или userId не найден или удалён
//   - [ErrPermissionDenied] — requesterId не администратор
func (app *Application) RemoveUser(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID) (bool, error) {
	r, err := app.repo.GetUserById(ctx, requesterId)
	if err != nil {
		return false, fmt.Errorf("get request source: %w", err)
	}

	if r.Role != domain.RoleAdmin {
		return false, ErrPermissionDenied
	}

	u, err := app.repo.GetUserById(ctx, userId)
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}

	err = app.repo.RemoveUser(ctx, &u)
	if err != nil {
		return false, fmt.Errorf("remove user: %w", err)
	}

	return true, nil
}

// ChangeRole может вернуть ошибки:
//
// Приложение:
//   - [ErrNotFound] — requesterId или userId не найден или удалён
//   - [ErrPermissionDenied] — requesterId не администратор
func (app *Application) ChangeRole(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID, role domain.Role) (bool, error) {
	requester, err := app.repo.GetUserById(ctx, requesterId)
	if err != nil {
		return false, fmt.Errorf("unknown request source: %w", err)
	}

	if requester.Role != domain.RoleAdmin {
		return false, ErrPermissionDenied
	}

	u, err := app.repo.GetUserById(ctx, userId)
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}

	u.Role = role

	err = app.repo.UpdateUser(ctx, &u)
	if err != nil {
		return false, fmt.Errorf("saving update: %w", err)
	}

	return true, nil
}

// ChangePassword может вернуть ошибки:
//
// Приложение:
//   - [ErrPermissionDenied] — requesterId != userId
//   - [ErrNotFound] — пользователь userId не найден или удалён
//
// Домен:
//   - [domain.ErrInvalidArgument] — newPassword пустой
//   - [domain.ErrWrongPassword] — oldPassword не совпадает
//   - [domain.ErrHashService] — сбой сравнения хэша
func (app *Application) ChangePassword(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID, oldPassword string, newPassword string) (bool, error) {
	// TODO: Administrator branch
	if requesterId != userId {
		return false, fmt.Errorf("%w", ErrPermissionDenied)
	}

	u, err := app.repo.GetUserById(ctx, userId)
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}

	updated, err := u.SetPassword(app.hasher, oldPassword, newPassword)
	if err != nil {
		return false, fmt.Errorf("setting new password: %w", err)
	}
	if !updated {
		return false, fmt.Errorf("could not update: set failed")
	}

	err = app.repo.UpdateUser(ctx, &u)
	if err != nil {
		return false, fmt.Errorf("saving update: %w", err)
	}

	return true, nil
}

// AuthUser может вернуть ошибки:
//
// Приложение:
//   - [ErrNotFound] — пользователь username не найден или удалён
//   - [ErrWrongPassword] — password не совпадает
//
// Домен:
//   - [domain.ErrHashService] — сбой сравнения хэша
func (app *Application) AuthUser(ctx context.Context, username string, password string) (string, error) {
	u, err := app.repo.GetUserByUsername(ctx, username)
	if errors.Is(err, ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	verified, err := u.Verify(app.hasher, password)
	if err != nil {
		return "", fmt.Errorf("verifying password: %w", err)
	}

	if !verified {
		return "", ErrWrongPassword
	}

	tkn, err := app.auth.MakeToken(u.ID, u.Role)
	if err != nil {
		return "", fmt.Errorf("creating auth token: %w", err)
	}

	return tkn, nil
}
