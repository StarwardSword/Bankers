package application

import (
	"context"

	"github.com/StarwardSword/bank/features/accounts/domain"
	"github.com/google/uuid"
)

type Repository interface {
	SaveAccount(ctx context.Context, acc *domain.Account) error
	GetById(ctx context.Context, accId uuid.UUID) (domain.Account, error)
	GetUserAccounts(ctx context.Context, userId uuid.UUID) ([]domain.Account, error)
	UserExist(ctx context.Context, userId uuid.UUID) (bool, error)
}
