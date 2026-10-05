package application

import (
	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/google/uuid"
)

type AuthService interface {
	MakeToken(id uuid.UUID, role domain.Role) (string, error)
}
