package application

import (
	"github.com/StarwardSword/bank/features/website/application/accountservice"
	"github.com/StarwardSword/bank/features/website/application/authservice"
	"github.com/StarwardSword/bank/features/website/application/userservice"
)

type Application struct {
	Auth    authservice.Service
	Account accountservice.Service
	Users   userservice.Service
}

func NewApplication(auth authservice.Service, account accountservice.Service, users userservice.Service) (*Application, error) {
	return &Application{
		Auth:    auth,
		Account: account,
		Users:   users,
	}, nil
}
