package localauthservice

import (
	"context"
	"fmt"

	userapp "github.com/StarwardSword/bank/features/user_service/application"
)

type Service struct {
	userApp *userapp.Application
}

func NewService(app *userapp.Application) (*Service, error) {
	return &Service{
		userApp: app,
	}, nil
}

func (a *Service) Auth(ctx context.Context, username string, password string) (string, error) {
	tkn, err := a.userApp.AuthUser(ctx, username, password)
	if err != nil {
		return "", fmt.Errorf("user service auth: %w", err)
	}

	return tkn, nil
}
