package userservice

import (
	"context"
	"errors"

	roles "github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/google/uuid"
)

var (
	ErrAlreadyExists    = errors.New("username already exists")
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotFound         = errors.New("not found")
	ErrWrongPassword    = errors.New("wrong password")
)

type Service interface {
	CreateUser(ctx context.Context, name string, password string) (uuid.UUID, error)
	RemoveUser(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID) (bool, error)
	ChangeRole(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID, role roles.Role) (bool, error)
	ChangePassword(ctx context.Context, requesterId uuid.UUID, userId uuid.UUID, oldPassword string, newPassword string) (bool, error)
}
