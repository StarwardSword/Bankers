package application

import (
	"context"

	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/google/uuid"
)

type Repository interface {
	SaveUser(ctx context.Context, u *domain.User) error
	RemoveUser(ctx context.Context, u *domain.User) error
	UpdateUser(ctx context.Context, u *domain.User) error
	GetUserById(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetUserByUsername(ctx context.Context, username string) (domain.User, error)
}
