package auth

import (
	"context"
	"fmt"

	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/google/uuid"
)

type data struct {
	UserId uuid.UUID
	Role   domain.Role
}

func Authenticate(ctx context.Context) (data, error) {
	idval := ctx.Value(UserId)
	id, ok := idval.(uuid.UUID)
	if !ok {
		return data{}, fmt.Errorf("can not find id")
	}

	roleval := ctx.Value(Role)
	role, ok := roleval.(int)
	if !ok {
		return data{}, fmt.Errorf("can not find role")
	}

	return data{
		UserId: id,
		Role:   domain.Role(role),
	}, nil
}
